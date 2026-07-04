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
	module := MakeModule(xfs.IOFS{FS: fstest.MapFS{
		"dir/nested.txt": {Data: []byte("nested")},
		"file.txt":       {Data: []byte("content")},
		"sibling/file.txt": {
			Data: []byte("content"),
		},
	}})
	predeclared := starlark.StringDict{
		"root_entries": starlark.NewList([]starlark.Value{
			starlark.String("dir"),
			starlark.String("file.txt"),
			starlark.String("sibling"),
		}),
		"sibling_dir": starlark.String("sibling"),
	}
	filename := filepath.Join("testdata", "os.star")
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
