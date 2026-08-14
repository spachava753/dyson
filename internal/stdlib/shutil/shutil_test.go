package shutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson/internal/chunkedfile"
	stdlibos "github.com/spachava753/dyson/internal/stdlib/os"
	"github.com/spachava753/dyson/internal/stdlibfs"
	"github.com/spachava753/starlarkx/starlark"
	"github.com/spachava753/starlarkx/starlarktest"
	"github.com/spachava753/starlarkx/syntax"
	"github.com/spf13/afero"
)

type openFSWithoutIdentity struct {
	afero.Fs
}

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

	fileDescriptors := stdlibfs.NewFileDescriptors()
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

func TestRmtreeDoesNotFollowDirectorySymlinks(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeFile(t, outside, "keep.txt", "keep")
	be.Err(t, os.Mkdir(filepath.Join(root, "tree"), 0o755), nil)
	if err := os.Symlink(outside, filepath.Join(root, "tree", "outside")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	osModule := stdlibos.MakeModule(stdlibos.HostConfig(root))
	module := MakeModule(HostConfig(root, osModule))
	_, err := starlark.Call(newTestThread(t, osModule, module), module.Members["rmtree"], starlark.Tuple{starlark.String("tree")}, nil)
	be.Err(t, err, nil)
	_, err = os.Stat(filepath.Join(outside, "keep.txt"))
	be.Err(t, err, nil)

	be.Err(t, os.Symlink(outside, filepath.Join(root, "tree-link")), nil)
	_, err = starlark.Call(newTestThread(t, osModule, module), module.Members["rmtree"], starlark.Tuple{starlark.String("tree-link")}, nil)
	if err == nil || !strings.Contains(err.Error(), "cannot call rmtree on a symbolic link") {
		t.Fatalf("rmtree symlink error = %v", err)
	}
	_, err = os.Lstat(filepath.Join(root, "tree-link"))
	be.Err(t, err, nil)
}

func TestCopyfileFailsClosedWithoutSameFileCheck(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "source.txt", "keep")
	host := stdlibfs.NewHost(afero.NewOsFs(), root)
	filesystem := openFSWithoutIdentity{Fs: host}
	osConfig := stdlibos.HostConfig(root)
	osConfig.FS = filesystem
	osModule := stdlibos.MakeModule(osConfig)
	config := HostConfig(root, osModule)
	config.FS = filesystem
	module := MakeModule(config)
	_, err := starlark.Call(newTestThread(t, osModule, module), module.Members["copyfile"], starlark.Tuple{
		starlark.String("source.txt"), starlark.String("source.txt"),
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "cannot check") {
		t.Fatalf("copyfile error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(root, "source.txt"))
	be.Err(t, err, nil)
	be.Equal(t, string(content), "keep")
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
