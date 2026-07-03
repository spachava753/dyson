package dyson

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson/internal/codec"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarktest"
)

func mustSerialize(t *testing.T, registry codec.Registry, val starlark.Value) codec.SerializedVal {
	t.Helper()
	sv, err := registry.Serialize(val)
	be.Err(t, err, nil)
	return sv
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
		}, DefaultCodecRegistry())
	}
	defaultRegistry := DefaultCodecRegistry()
	tupleVal := func(vals ...starlark.Value) codec.SerializedVal {
		return mustSerialize(t, defaultRegistry, starlark.Tuple(vals))
	}
	intVal := func(i int) codec.SerializedVal {
		return mustSerialize(t, defaultRegistry, starlark.MakeInt(i))
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
		be.Equal(t, len(s.log), 1)
		be.Equal(t, len(s.log[0].Calls), 1)
		be.Equal(t, s.log[0].Calls[0], HostCall{
			FnName: "add",
			Args: codec.SerializedVal{
				Type:    "tuple",
				Version: 1,
				List: []codec.SerializedVal{
					{Type: "int", Version: 1, Hash: 0x3000014, Data: []byte("1")},
					{Type: "int", Version: 1, Hash: 0x3c00019, Data: []byte("2")},
				},
			},
			Kwargs:   []codec.SerializedVal{},
			Response: codec.SerializedVal{Type: "int", Version: 1, Hash: 0x480001e, Data: []byte("3")},
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
		be.Equal(t, len(s.log), 1)
		be.Equal(t, len(s.log[0].Calls), 2)
		be.Equal(t, s.log[0].Calls[0], HostCall{
			FnName:   "tick",
			Args:     tupleVal(starlark.String("same")),
			Kwargs:   []codec.SerializedVal{},
			Response: intVal(1),
		})
		be.Equal(t, s.log[0].Calls[1], HostCall{
			FnName:   "tick",
			Args:     tupleVal(starlark.String("same")),
			Kwargs:   []codec.SerializedVal{},
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
		be.Equal(t, len(s.log), 2)
		be.Equal(t, s.log[0].Code, first)
		be.Equal(t, s.log[1].Code, second)
		be.Equal(t, s.log[0].Calls, []HostCall{{
			FnName:   "echo",
			Args:     tupleVal(starlark.String("first")),
			Kwargs:   []codec.SerializedVal{},
			Response: mustSerialize(t, defaultRegistry, starlark.String("first")),
		}})
		be.Equal(t, s.log[1].Calls, []HostCall{{
			FnName:   "echo",
			Args:     tupleVal(starlark.String("second")),
			Kwargs:   []codec.SerializedVal{},
			Response: mustSerialize(t, defaultRegistry, starlark.String("second")),
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
		be.Equal(t, len(s.log), 1)
		be.Equal(t, len(s.log[0].Calls), 1)
		be.Equal(t, s.log[0].Calls[0], HostCall{
			FnName:   "kwcount",
			Args:     tupleVal(starlark.MakeInt(1)),
			Kwargs:   []codec.SerializedVal{tupleVal(starlark.String("mode"), starlark.String("fast"))},
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
		be.Equal(t, len(s.log), 1)
		be.Equal(t, len(s.log[0].Calls), 1)
		be.Equal(t, s.log[0].Calls[0], HostCall{
			FnName:   "mirror",
			Args:     tupleVal(list),
			Kwargs:   []codec.SerializedVal{},
			Response: mustSerialize(t, defaultRegistry, list),
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
		be.Equal(t, len(s.log), 1)
		be.Equal(t, len(s.log[0].Calls), 1)
		call := s.log[0].Calls[0]
		be.Equal(t, call.FnName, "explode")
		be.Equal(t, call.Args, tupleVal(starlark.MakeInt(1)))
		be.Equal(t, call.Kwargs, []codec.SerializedVal{})
		be.Equal(t, call.Response, codec.SerializedVal{})
		callErr, ok := call.Err.(error)
		if !ok {
			t.Fatalf("recorded error has type %T, want error", call.Err)
		}
		if !strings.Contains(callErr.Error(), "boom") {
			t.Fatalf("recorded error %q does not contain boom", callErr.Error())
		}
	})

	t.Run("fails before host effect when input codec is missing", func(t *testing.T) {
		called := false
		registry := DefaultCodecRegistry()
		delete(registry, starlark.String("").Type())
		m, err := starlarktest.LoadAssertModule()
		be.Err(t, err, nil)
		s := NewSphere(func(thread *starlark.Thread, msg string) {
			t.Log(msg)
		}, map[string]starlark.StringDict{
			"assert.star": m,
			"custom.star": starlark.StringDict{
				"effect": starlark.NewBuiltin("effect", func(
					thread *starlark.Thread,
					fn *starlark.Builtin,
					args starlark.Tuple,
					kwargs []starlark.Tuple,
				) (starlark.Value, error) {
					called = true
					return starlark.None, nil
				}),
			},
		}, registry)
		err = s.Eval(t.Context(), `
load("custom.star", "effect")
effect("unsupported")
`)
		if err == nil {
			t.Fatal("expected eval to fail")
		}
		if !strings.Contains(err.Error(), "string") {
			t.Fatalf("serialization error %q does not mention string", err.Error())
		}
		be.Equal(t, called, false)
		be.Equal(t, len(s.log), 1)
		be.Equal(t, len(s.log[0].Calls), 0)
	})
}
