package glob

import (
	"fmt"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson/internal/chunkedfile"
	stdlibos "github.com/spachava753/dyson/internal/stdlib/os"
	"github.com/spachava753/dyson/internal/stdlibfs"
	"github.com/spachava753/starlarkx/starlark"
	"github.com/spachava753/starlarkx/starlarkstruct"
	"github.com/spachava753/starlarkx/starlarktest"
	"github.com/spachava753/starlarkx/syntax"
)

func TestGlobTestdata(t *testing.T) {
	filename, err := filepath.Abs(filepath.Join("testdata", "glob.star"))
	be.Err(t, err, nil)

	fsys := stdlibfs.NewIOFS(fstest.MapFS{
		"a.txt":             {Data: []byte("a")},
		"b.py":              {Data: []byte("b")},
		".hidden":           {Data: []byte("hidden")},
		"subdir/nested.txt": {Data: []byte("nested")},
	})
	osModule := stdlibos.MakeModule(stdlibos.ModuleConfig{FS: fsys})
	module := MakeModule(osModule)

	predeclared := starlark.StringDict{
		"root": starlark.String("."),
		"sep":  starlark.String("/"),
	}

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

func TestGlobRepeatedSeparatorAfterCurrentDirectoryStaysRelative(t *testing.T) {
	fsys := stdlibfs.NewIOFS(fstest.MapFS{
		"file.txt": {Data: []byte("content")},
	})
	osModule := stdlibos.MakeModule(stdlibos.ModuleConfig{FS: fsys})
	module := MakeModule(osModule)

	value, err := starlark.Call(
		newTestThread(t, module),
		module.Members["glob"],
		starlark.Tuple{starlark.String(".//file.txt")},
		nil,
	)
	be.Err(t, err, nil)
	list, ok := value.(*starlark.List)
	be.Equal(t, ok, true)
	be.Equal(t, list.Len(), 1)
	be.Equal(t, list.Index(0), starlark.Value(starlark.String(".//file.txt")))
}

func newTestThread(t *testing.T, module *starlarkstruct.Module) *starlark.Thread {
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
