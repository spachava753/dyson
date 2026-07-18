package dyson

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson/internal/chunkedfile"
	"go.starlark.net/lib/math"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarktest"
)

func TestEvalStopsWhenContextEnds(t *testing.T) {
	s := NewSphere(nil, nil, nil, DefaultCodecRegistry(), false)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := s.Eval(ctx, `while True: pass`)
	if err == nil || !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("Eval() error = %v, want cancellation error", err)
	}

	be.Err(t, s.Eval(t.Context(), `completed = True`), nil)
}

func TestNewSphereAcceptsInitialGlobals(t *testing.T) {
	initialGlobals := starlark.StringDict{
		"answer": starlark.MakeInt(21),
		"double": starlark.NewBuiltin("double", func(
			thread *starlark.Thread,
			fn *starlark.Builtin,
			args starlark.Tuple,
			kwargs []starlark.Tuple,
		) (starlark.Value, error) {
			var value int
			if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "value", &value); err != nil {
				return nil, err
			}
			return starlark.MakeInt(value * 2), nil
		}),
	}
	s := NewSphere(nil, nil, initialGlobals, DefaultCodecRegistry(), false)

	be.Err(t, s.Eval(t.Context(), `
if double(answer) != 42:
    fail("initial globals unavailable")
`), nil)
}

func TestEvalTestdata(t *testing.T) {
	err := filepath.WalkDir("testdata", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".star") {
			return nil
		}
		t.Run(d.Name(), func(t *testing.T) {
			chunks := chunkedfile.Read(path, t)
			var sb strings.Builder
			m, err := starlarktest.LoadAssertModule()
			be.Err(t, err, nil)
			registry := DefaultCodecRegistry()
			mods := StdlibModules()
			mods[math.Module.Name+".star"] = starlark.StringDict{
				"math": math.Module,
			}
			s := NewSphere(func(thread *starlark.Thread, msg string) {
				fmt.Fprintln(&sb, msg)
			}, mods, nil, registry, true)
			lastIdx := len(chunks) - 1

			// run the simulated repl chunks
			for i := range lastIdx {
				err = s.Eval(t.Context(), chunks[i].Source)
				if err != nil {
					chunks[i].GotErrorAnyLine(err.Error())
				}
				chunks[i].Done()
			}

			// get the log
			log := s.Log()

			mods["assert.star"] = m

			// run the last chunk after replaying, which will be assertions
			s = NewSphere(func(thread *starlark.Thread, msg string) {
				fmt.Fprintln(&sb, msg)
			}, mods, nil, registry, true)
			starlarktest.SetReporter(s.t, t)
			// Replay may encounter the same intentional Starlark failures as setup chunks.
			_ = s.Replay(t.Context(), log)

			err = s.Eval(t.Context(), chunks[lastIdx].Source)
			if err != nil {
				chunks[lastIdx].GotErrorAnyLine(err.Error())
			}
			chunks[lastIdx].Done()
		})

		return nil
	})
	be.Err(t, err, nil)
}
