package os

import (
	"fmt"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/spachava753/dyson/internal/chunkedfile"
	"github.com/spachava753/dyson/internal/xfs"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarktest"
	"go.starlark.net/syntax"
)

func TestOSTestdata(t *testing.T) {
	module := MakeModule(ModuleConfig{FS: xfs.IOFS{FS: fstest.MapFS{
		"dir/nested.txt": {Data: []byte("nested"), Mode: 0o644},
		"file.txt":       {Data: []byte("content"), Mode: 0o644},
		"sibling/file.txt": {
			Data: []byte("content"), Mode: 0o644,
		},
	}}})
	predeclared := starlark.StringDict{
		"root_entries": starlark.NewList([]starlark.Value{
			starlark.String("dir"),
			starlark.String("file.txt"),
			starlark.String("sibling"),
		}),
		"sibling_dir": starlark.String("sibling"),
	}
	runOSTestdata(t, module, filepath.Join("testdata", "os.star"), predeclared)
}

func TestOSHostTestdata(t *testing.T) {
	t.Setenv("DYSON_OS_TEST", "before")
	module := MakeModule(HostConfig(t.TempDir()))
	runOSTestdata(t, module, filepath.Join("testdata", "host.star"), nil)
}

func runOSTestdata(t *testing.T, module starlark.Value, filename string, predeclared starlark.StringDict) {
	t.Helper()
	for i, chunk := range chunkedfile.Read(filename, t) {
		t.Run(fmt.Sprintf("chunk_%02d", i+1), func(t *testing.T) {
			thread := newTestThread(t, module)
			_, err := starlark.ExecFileOptions(&syntax.FileOptions{}, thread, filename, chunk.Source, predeclared)
			if err != nil {
				chunk.GotErrorAnyLine(err.Error())
			}
			chunk.Done()
		})
	}
}

func newTestThread(t *testing.T, module starlark.Value) *starlark.Thread {
	thread := &starlark.Thread{
		Name: "test",
		Load: func(thread *starlark.Thread, name string) (starlark.StringDict, error) {
			switch name {
			case ModuleName + ".star":
				return starlark.StringDict{
					ModuleName: module,
				}, nil
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
