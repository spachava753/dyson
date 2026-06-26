package time

import (
	"fmt"
	"path/filepath"
	"testing"
	"testing/synctest"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson/internal/chunkedfile"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/starlarktest"
	"go.starlark.net/syntax"
)

func TestLoadModuleShape(t *testing.T) {
	globals, err := LoadModule()
	be.Err(t, err, nil)
	be.Equal(t, len(globals), 1)

	module, ok := globals[ModuleName].(*starlarkstruct.Module)
	be.True(t, ok)
	be.True(t, module != nil)

	timeFn, err := module.Attr("time")
	be.Err(t, err, nil)
	be.True(t, timeFn != nil)
}

func TestTimeTestdata(t *testing.T) {
	filename := filepath.Join("testdata", "time.star")
	for i, chunk := range chunkedfile.Read(filename, t) {
		t.Run(fmt.Sprintf("chunk_%02d", i+1), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				thread := newTestThread(t)
				_, err := starlark.ExecFileOptions(&syntax.FileOptions{}, thread, filename, chunk.Source, nil)
				if err != nil {
					chunk.GotErrorAnyLine(err.Error())
				}
				chunk.Done()
			})
		})
	}
}

func newTestThread(t *testing.T) *starlark.Thread {
	thread := &starlark.Thread{
		Name: "test",
		Load: func(thread *starlark.Thread, name string) (starlark.StringDict, error) {
			switch name {
			case ModuleName + ".star":
				return LoadModule()
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
