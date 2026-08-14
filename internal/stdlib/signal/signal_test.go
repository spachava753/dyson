package signal

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/spachava753/dyson/internal/chunkedfile"
	"github.com/spachava753/starlarkx/starlark"
	"github.com/spachava753/starlarkx/starlarktest"
	"github.com/spachava753/starlarkx/syntax"
)

func TestSignalTestdata(t *testing.T) {
	filename := filepath.Join("testdata", "signal.star")
	for i, chunk := range chunkedfile.Read(filename, t) {
		t.Run(fmt.Sprintf("chunk_%02d", i+1), func(t *testing.T) {
			thread := newTestThread(t)
			_, err := starlark.ExecFileOptions(&syntax.FileOptions{}, thread, filename, chunk.Source, nil)
			if err != nil {
				chunk.GotErrorAnyLine(err.Error())
			}
			chunk.Done()
		})
	}
}

func newTestThread(t *testing.T) *starlark.Thread {
	thread := &starlark.Thread{
		Name: "test",
		Load: func(thread *starlark.Thread, name string) (starlark.StringDict, error) {
			switch name {
			case ModuleName + ".star":
				return starlark.StringDict{ModuleName: Module}, nil
			case "assert.star":
				return starlarktest.LoadAssertModule()
			default:
				return nil, fmt.Errorf("unknown module %q", name)
			}
		},
	}
	starlarktest.SetReporter(thread, t)
	return thread
}
