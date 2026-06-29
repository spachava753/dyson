package dyson

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson/internal/chunkedfile"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarktest"
)

func TestEvalTestdata(t *testing.T) {
	filepath.WalkDir("testdata", func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), "_test.star") {
			return nil
		}
		chunks := chunkedfile.Read(path, t)
		for i, chunk := range chunks {
			var sb strings.Builder
			t.Run(fmt.Sprintf("chunk_%02d", i+1), func(t *testing.T) {
				m, err := starlarktest.LoadAssertModule()
				be.Err(t, err, nil)
				s := NewSphere(func(thread *starlark.Thread, msg string) {
					fmt.Fprintln(&sb, msg)
				}, map[string]starlark.StringDict{
					"assert.star": m,
				})
				err = s.Eval(t.Context(), chunk.Source)
				if err != nil {
					chunk.GotErrorAnyLine(err.Error())
				}
				chunk.Done()
			})
		}
		return nil
	})
}

func TestRecordTestdata(t *testing.T) {
	newTestSphere := func(t *testing.T, custom starlark.StringDict) *Sphere {
		t.Helper()
		m, err := starlarktest.LoadAssertModule()
		be.Err(t, err, nil)
		return NewSphere(func(thread *starlark.Thread, msg string) {
			t.Log(msg)
		}, map[string]starlark.StringDict{
			"assert.star": m,
			"custom.star": custom,
		})
	}
	tupleVal := func(vals ...starlark.Value) SerializedVal {
		return serialize(starlark.Tuple(vals))
	}
	intVal := func(i int) SerializedVal {
		return serialize(starlark.MakeInt(i))
	}

	t.Run("basic", func(t *testing.T) {
		s := newTestSphere(t, starlark.StringDict{
			"add": starlark.NewBuiltin("add", func(
				thread *starlark.Thread,
				fn *starlark.Builtin,
				args starlark.Tuple,
				kwargs []starlark.Tuple,
			) (starlark.Value, error) {
				var x, y int
				if err := starlark.UnpackPositionalArgs(fn.Name(), args, kwargs, 2, &x, &y); err != nil {
					return nil, err
				}
				return starlark.MakeInt(x + y), nil
			}),
		})
		be.Err(t, s.Eval(t.Context(), `
load("custom.star", "add")
result = add(1, 2)
`), nil)
		be.Equal(t, len(s.Log), 1)
		be.Equal(t, len(s.Log[0].Calls), 1)
		be.Equal(t, s.Log[0].Calls[0], HostCall{
			FnName: "add",
			Args: SerializedVal{
				Type: "tuple",
				List: []SerializedVal{
					{Type: "int", Hash: 0x3000014},
					{Type: "int", Hash: 0x3c00019},
				},
			},
			Kwargs:   []SerializedVal{},
			Response: SerializedVal{Type: "int", Hash: 0x480001e},
		})
	})

	t.Run("records repeated calls as ordered events", func(t *testing.T) {
		count := 0
		s := newTestSphere(t, starlark.StringDict{
			"tick": starlark.NewBuiltin("tick", func(
				thread *starlark.Thread,
				fn *starlark.Builtin,
				args starlark.Tuple,
				kwargs []starlark.Tuple,
			) (starlark.Value, error) {
				count++
				return starlark.MakeInt(count), nil
			}),
		})
		be.Err(t, s.Eval(t.Context(), `
load("custom.star", "tick")
first = tick("same")
second = tick("same")
`), nil)
		be.Equal(t, len(s.Log), 1)
		be.Equal(t, len(s.Log[0].Calls), 2)
		be.Equal(t, s.Log[0].Calls[0], HostCall{
			FnName:   "tick",
			Args:     tupleVal(starlark.String("same")),
			Kwargs:   []SerializedVal{},
			Response: intVal(1),
		})
		be.Equal(t, s.Log[0].Calls[1], HostCall{
			FnName:   "tick",
			Args:     tupleVal(starlark.String("same")),
			Kwargs:   []SerializedVal{},
			Response: intVal(2),
		})
	})

	t.Run("keeps host calls with their chunk", func(t *testing.T) {
		s := newTestSphere(t, starlark.StringDict{
			"echo": starlark.NewBuiltin("echo", func(
				thread *starlark.Thread,
				fn *starlark.Builtin,
				args starlark.Tuple,
				kwargs []starlark.Tuple,
			) (starlark.Value, error) {
				return args[0], nil
			}),
		})
		first := `load("custom.star", "echo")
one = echo("first")
`
		second := `load("custom.star", "echo")
two = echo("second")
`
		be.Err(t, s.Eval(t.Context(), first), nil)
		be.Err(t, s.Eval(t.Context(), second), nil)
		be.Equal(t, len(s.Log), 2)
		be.Equal(t, s.Log[0].Code, first)
		be.Equal(t, s.Log[1].Code, second)
		be.Equal(t, s.Log[0].Calls, []HostCall{{
			FnName:   "echo",
			Args:     tupleVal(starlark.String("first")),
			Kwargs:   []SerializedVal{},
			Response: serialize(starlark.String("first")),
		}})
		be.Equal(t, s.Log[1].Calls, []HostCall{{
			FnName:   "echo",
			Args:     tupleVal(starlark.String("second")),
			Kwargs:   []SerializedVal{},
			Response: serialize(starlark.String("second")),
		}})
	})

	t.Run("records keyword arguments", func(t *testing.T) {
		s := newTestSphere(t, starlark.StringDict{
			"kwcount": starlark.NewBuiltin("kwcount", func(
				thread *starlark.Thread,
				fn *starlark.Builtin,
				args starlark.Tuple,
				kwargs []starlark.Tuple,
			) (starlark.Value, error) {
				return starlark.MakeInt(len(kwargs)), nil
			}),
		})
		be.Err(t, s.Eval(t.Context(), `
load("custom.star", "kwcount")
seen = kwcount(1, mode="fast")
`), nil)
		be.Equal(t, len(s.Log), 1)
		be.Equal(t, len(s.Log[0].Calls), 1)
		be.Equal(t, s.Log[0].Calls[0], HostCall{
			FnName:   "kwcount",
			Args:     tupleVal(starlark.MakeInt(1)),
			Kwargs:   []SerializedVal{tupleVal(starlark.String("mode"), starlark.String("fast"))},
			Response: intVal(1),
		})
	})

	t.Run("records nested serialized values", func(t *testing.T) {
		s := newTestSphere(t, starlark.StringDict{
			"mirror": starlark.NewBuiltin("mirror", func(
				thread *starlark.Thread,
				fn *starlark.Builtin,
				args starlark.Tuple,
				kwargs []starlark.Tuple,
			) (starlark.Value, error) {
				return args[0], nil
			}),
		})
		be.Err(t, s.Eval(t.Context(), `
load("custom.star", "mirror")
value = mirror([1, "two", True])
`), nil)
		list := starlark.NewList([]starlark.Value{starlark.MakeInt(1), starlark.String("two"), starlark.Bool(true)})
		be.Equal(t, len(s.Log), 1)
		be.Equal(t, len(s.Log[0].Calls), 1)
		be.Equal(t, s.Log[0].Calls[0], HostCall{
			FnName:   "mirror",
			Args:     tupleVal(list),
			Kwargs:   []SerializedVal{},
			Response: serialize(list),
		})
	})

	t.Run("records builtin errors", func(t *testing.T) {
		s := newTestSphere(t, starlark.StringDict{
			"explode": starlark.NewBuiltin("explode", func(
				thread *starlark.Thread,
				fn *starlark.Builtin,
				args starlark.Tuple,
				kwargs []starlark.Tuple,
			) (starlark.Value, error) {
				return nil, fmt.Errorf("boom")
			}),
		})
		err := s.Eval(t.Context(), `
load("custom.star", "explode")
explode(1)
`)
		if err == nil {
			t.Fatal("expected eval to fail")
		}
		be.Equal(t, len(s.Log), 1)
		be.Equal(t, len(s.Log[0].Calls), 1)
		call := s.Log[0].Calls[0]
		be.Equal(t, call.FnName, "explode")
		be.Equal(t, call.Args, tupleVal(starlark.MakeInt(1)))
		be.Equal(t, call.Kwargs, []SerializedVal{})
		be.Equal(t, call.Response, SerializedVal{})
		callErr, ok := call.Err.(error)
		if !ok {
			t.Fatalf("recorded error has type %T, want error", call.Err)
		}
		if !strings.Contains(callErr.Error(), "boom") {
			t.Fatalf("recorded error %q does not contain boom", callErr.Error())
		}
	})
}
