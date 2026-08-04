package os

import (
	"errors"
	"fmt"
	hostos "os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/spachava753/dyson/internal/chunkedfile"
	"github.com/spachava753/dyson/internal/stdlibfs"
	"github.com/spf13/afero"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarktest"
	"go.starlark.net/syntax"
)

func TestOSTestdata(t *testing.T) {
	fsys := stdlibfs.NewIOFS(fstest.MapFS{
		"dir/nested.txt":   {Data: []byte("nested"), Mode: 0o644},
		"file.txt":         {Data: []byte("content"), Mode: 0o644},
		"sibling/file.txt": {Data: []byte("content"), Mode: 0o644},
	})
	module := MakeModule(ModuleConfig{FS: fsys})
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

func TestSymlinkInspectionRejectsAferoStatFallback(t *testing.T) {
	fsys := afero.NewMemMapFs()
	if err := afero.WriteFile(fsys, "file.txt", []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	module := MakeModule(ModuleConfig{FS: fsys})
	pathModule := module.Members["path"].(starlark.HasAttrs)
	lexists, err := pathModule.Attr("lexists")
	if err != nil {
		t.Fatal(err)
	}
	islink, err := pathModule.Attr("islink")
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name     string
		function starlark.Value
	}{
		{name: "lstat", function: module.Members["lstat"]},
		{name: "lexists", function: lexists},
		{name: "islink", function: islink},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := starlark.Call(
				&starlark.Thread{Name: "test"},
				test.function,
				starlark.Tuple{starlark.String("file.txt")},
				nil,
			)
			if !errors.Is(err, errors.ErrUnsupported) {
				t.Fatalf("%s error = %v, want errors.ErrUnsupported", test.name, err)
			}
		})
	}
}

type recordingPathTruncateFS struct {
	afero.Fs
	path      string
	size      int64
	openCalls int
}

func (f *recordingPathTruncateFS) Truncate(path string, size int64) error {
	f.path = path
	f.size = size
	return nil
}

func (f *recordingPathTruncateFS) OpenFile(name string, flag int, perm hostos.FileMode) (afero.File, error) {
	f.openCalls++
	return f.Fs.OpenFile(name, flag, perm)
}

func TestTruncatePrefersPathOperation(t *testing.T) {
	fsys := &recordingPathTruncateFS{Fs: afero.NewMemMapFs()}
	module := MakeModule(ModuleConfig{FS: fsys})

	_, err := starlark.Call(
		&starlark.Thread{Name: "test"},
		module.Members["truncate"],
		starlark.Tuple{starlark.String("file.txt"), starlark.MakeInt(2)},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if fsys.path != "file.txt" || fsys.size != 2 || fsys.openCalls != 0 {
		t.Fatalf("path truncate = (%q, %d), open calls = %d", fsys.path, fsys.size, fsys.openCalls)
	}
}

func TestTruncateFallsBackForGenericAferoFileSystem(t *testing.T) {
	backing := afero.NewMemMapFs()
	fsys := stdlibfs.NewHost(backing, "root")
	if err := fsys.Mkdir(".", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(fsys, "file.txt", []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	module := MakeModule(ModuleConfig{FS: fsys})

	_, err := starlark.Call(
		&starlark.Thread{Name: "test"},
		module.Members["truncate"],
		starlark.Tuple{starlark.String("file.txt"), starlark.MakeInt(2)},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	info, err := fsys.Stat("file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 2 {
		t.Fatalf("truncated size = %d, want 2", info.Size())
	}
}

func TestOSHostTestdata(t *testing.T) {
	t.Setenv("DYSON_OS_TEST", "before")
	module := MakeModule(HostConfig(t.TempDir()))
	runOSTestdata(t, module, filepath.Join("testdata", "host.star"), nil)
}

func TestWalkClassifiesAndFollowsDirectorySymlinks(t *testing.T) {
	root := t.TempDir()
	if err := hostos.Mkdir(filepath.Join(root, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := hostos.WriteFile(filepath.Join(root, "real", "file.txt"), []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := hostos.Symlink("real", filepath.Join(root, "alias")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	module := MakeModule(HostConfig(root))
	_, err := starlark.ExecFile(newTestThread(t, module), "walk.star", `
load("assert.star", "assert")
load("os.star", "os")

without_links = [
    (".", ["alias", "real"], []),
    ("real", [], ["file.txt"]),
]
assert.eq(os.walk("."), without_links)

with_links = [
    (".", ["alias", "real"], []),
    ("alias", [], ["file.txt"]),
    ("real", [], ["file.txt"]),
]
assert.eq(os.walk(".", followlinks=True), with_links)
assert.true(os.path.realpath("alias/missing.txt").endswith("/real/missing.txt"))
os.symlink("missing-target", "dangling")
assert.true(os.path.realpath("dangling/child.txt").endswith("/missing-target/child.txt"))
`, nil)
	if err != nil {
		t.Fatal(err)
	}
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
