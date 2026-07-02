package dyson

import (
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

func TestEvalTestdata(t *testing.T) {
	err := filepath.WalkDir("testdata", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), "_test.star") {
			return nil
		}
		t.Run(d.Name(), func(t *testing.T) {
			chunks := chunkedfile.Read(path, t)
			var sb strings.Builder
			m, err := starlarktest.LoadAssertModule()
			be.Err(t, err, nil)
			registry := DefaultCodecRegistry()
			s := NewSphere(func(thread *starlark.Thread, msg string) {
				fmt.Fprintln(&sb, msg)
			}, map[string]starlark.StringDict{
				math.Module.Name: math.Module.Members,
			}, registry)
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

			// run the last chunk after replaying, which will be assertions
			s = NewSphere(func(thread *starlark.Thread, msg string) {
				fmt.Fprintln(&sb, msg)
			}, map[string]starlark.StringDict{
				"assert.star":    m,
				math.Module.Name: math.Module.Members,
			}, registry)
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
