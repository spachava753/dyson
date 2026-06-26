package os

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

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

	path, err := module.Attr("path")
	be.Err(t, err, nil)
	be.True(t, path != nil)
}

func TestOSTestdata(t *testing.T) {
	filename := filepath.Join("testdata", "os.star")
	for i, chunk := range chunkedfile.Read(filename, t) {
		t.Run(fmt.Sprintf("chunk_%02d", i+1), func(t *testing.T) {
			thread := newTestThread(t)
			_, err := starlark.ExecFileOptions(&syntax.FileOptions{}, thread, filename, chunk.Source, testGlobals(t))
			if err != nil {
				chunk.GotErrorAnyLine(err.Error())
			}
			chunk.Done()
		})
	}
}

func testGlobals(t *testing.T) starlark.StringDict {
	t.Helper()

	t.Setenv("DYSON_TEST_ENV", "initial")
	dir := t.TempDir()
	file := filepath.Join(dir, "sample.txt")
	subdir := filepath.Join(dir, "subdir")
	be.Err(t, os.WriteFile(file, []byte("sample"), 0o644), nil)
	be.Err(t, os.Mkdir(subdir, 0o755), nil)

	return starlark.StringDict{
		"TEST_TMPDIR":  starlark.String(dir),
		"TEST_FILE":    starlark.String(file),
		"TEST_DIR":     starlark.String(subdir),
		"TEST_MISSING": starlark.String(filepath.Join(dir, "missing.txt")),
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
