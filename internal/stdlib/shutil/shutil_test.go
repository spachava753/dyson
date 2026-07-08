package shutil

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson/internal/chunkedfile"
	stdlibos "github.com/spachava753/dyson/internal/stdlib/os"
	"github.com/spachava753/dyson/internal/xfs"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarktest"
	"go.starlark.net/syntax"
)

func TestShutilTestdata(t *testing.T) {
	t.Setenv("COLUMNS", "120")
	t.Setenv("LINES", "40")
	root := t.TempDir()
	writeFile(t, root, "src.txt", "hello")
	writeFile(t, root, "move-src.txt", "move")
	writeFile(t, root, "tree/a.txt", "a")
	writeFile(t, root, "tree/skip.tmp", "skip")
	writeFile(t, root, "tree/sub/b.txt", "b")
	writeFile(t, root, "bin/tool", "#!/bin/sh\n")
	be.Err(t, os.Chmod(filepath.Join(root, "bin", "tool"), 0o755), nil)

	fileDescriptors := xfs.NewFileDescriptors()
	osConfig := stdlibos.HostConfig(root)
	osConfig.FileDescriptors = fileDescriptors
	osModule := stdlibos.MakeModule(osConfig)
	module := MakeModule(HostConfig(root, osModule))

	filename, err := filepath.Abs(filepath.Join("testdata", "shutil.star"))
	be.Err(t, err, nil)
	for i, chunk := range chunkedfile.Read(filename, t) {
		t.Run(fmt.Sprintf("chunk_%02d", i+1), func(t *testing.T) {
			thread := newTestThread(t, osModule, module)
			_, err := starlark.ExecFileOptions(&syntax.FileOptions{}, thread, filename, chunk.Source, nil)
			if err != nil {
				chunk.GotErrorAnyLine(err.Error())
			}
			chunk.Done()
		})
	}
}

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	be.Err(t, os.MkdirAll(filepath.Dir(path), 0o777), nil)
	be.Err(t, os.WriteFile(path, []byte(content), 0o666), nil)
}

func newTestThread(t *testing.T, osModule, shutilModule starlark.Value) *starlark.Thread {
	thread := &starlark.Thread{
		Name: "shutil_test",
		Load: func(thread *starlark.Thread, name string) (starlark.StringDict, error) {
			switch name {
			case "assert.star":
				return starlarktest.LoadAssertModule()
			case stdlibos.ModuleName + ".star":
				return starlark.StringDict{stdlibos.ModuleName: osModule}, nil
			case ModuleName + ".star":
				return starlark.StringDict{ModuleName: shutilModule}, nil
			default:
				return nil, fmt.Errorf("unknown module %q", name)
			}
		},
	}
	starlarktest.SetReporter(thread, t)
	return thread
}
