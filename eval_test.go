package dyson

import (
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson/internal/chunkedfile"
	"go.starlark.net/lib/math"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarktest"
)

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
			mods := maps.Clone(StdlibModules)
			mods[math.Module.Name+".star"] = starlark.StringDict{
				"math": math.Module,
			}
			s := NewSphere(func(thread *starlark.Thread, msg string) {
				fmt.Fprintln(&sb, msg)
			}, mods, registry)
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
			}, mods, registry)
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
