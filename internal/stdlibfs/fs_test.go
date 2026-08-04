package stdlibfs

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"testing/fstest"

	"github.com/nalgeon/be"
	"github.com/spf13/afero"
)

type descriptorTestFile struct {
	afero.File
	closeCalls int
	closeErr   error
}

func (*descriptorTestFile) Read([]byte) (int, error)       { return 0, io.EOF }
func (*descriptorTestFile) Write(data []byte) (int, error) { return len(data), nil }
func (f *descriptorTestFile) Close() error                 { f.closeCalls++; return f.closeErr }
func (*descriptorTestFile) Sync() error                    { return nil }
func (*descriptorTestFile) Truncate(int64) error           { return nil }

func TestFileDescriptorsClose(t *testing.T) {
	closeErr := errors.New("close failed")
	first := &descriptorTestFile{closeErr: closeErr}
	second := &descriptorTestFile{}
	files := NewFileDescriptors()

	firstFD, err := files.Store(first)
	be.Err(t, err, nil)
	be.Equal(t, firstFD, 3)
	secondFD, err := files.Store(second)
	be.Err(t, err, nil)
	be.Equal(t, secondFD, 4)

	err = files.Close()
	be.Equal(t, errors.Is(err, closeErr), true)
	be.Equal(t, first.closeCalls, 1)
	be.Equal(t, second.closeCalls, 1)
	_, found := files.Lookup(firstFD)
	be.Equal(t, found, false)

	be.Err(t, files.Close(), nil)
	be.Equal(t, first.closeCalls, 1)
	be.Equal(t, second.closeCalls, 1)

	late := &descriptorTestFile{}
	_, err = files.Store(late)
	be.Equal(t, errors.Is(err, errFileDescriptorsClosed), true)
	be.Equal(t, late.closeCalls, 0)
	be.Err(t, late.Close(), nil)
}

type aferoFSOnly struct {
	afero.Fs
}

func TestLstatFailsWhenUnsupported(t *testing.T) {
	backing := afero.NewMemMapFs()
	be.Err(t, afero.WriteFile(backing, "file.txt", []byte("content"), 0o600), nil)

	_, err := Lstat(aferoFSOnly{Fs: backing}, "file.txt")
	be.Equal(t, errors.Is(err, errors.ErrUnsupported), true)
	pathErr, ok := errors.AsType[*fs.PathError](err)
	be.Equal(t, ok, true)
	be.Equal(t, pathErr.Op, "lstat")
	be.Equal(t, pathErr.Path, "file.txt")
}

func TestLstatRejectsBackendStatFallback(t *testing.T) {
	backing := afero.NewMemMapFs()
	be.Err(t, afero.WriteFile(backing, "file.txt", []byte("content"), 0o600), nil)

	_, err := Lstat(backing, "file.txt")
	be.Equal(t, errors.Is(err, errors.ErrUnsupported), true)
}

func TestHostRealpathRejectsBackendStatFallback(t *testing.T) {
	host := NewHost(afero.NewMemMapFs(), ".")

	_, err := host.Realpath("file.txt")
	be.Equal(t, errors.Is(err, errors.ErrUnsupported), true)
}

func TestHostUsesConfiguredFileSystem(t *testing.T) {
	backing := afero.NewMemMapFs()
	host := NewHost(backing, "base")

	be.Err(t, host.MkdirAll(".", 0o755), nil)
	be.Err(t, afero.WriteFile(host, "file.txt", []byte("content"), 0o600), nil)
	content, err := afero.ReadFile(backing, "base/file.txt")
	be.Err(t, err, nil)
	be.Equal(t, string(content), "content")
}

type recordingReadDirFS struct {
	afero.Fs
	name string
}

func (f *recordingReadDirFS) ReadDir(name string) ([]fs.DirEntry, error) {
	f.name = name
	return nil, nil
}

func TestHostReadDirDelegatesResolvedPath(t *testing.T) {
	backing := &recordingReadDirFS{Fs: afero.NewMemMapFs()}
	host := NewHost(backing, "root")

	_, err := host.ReadDir("dir")
	be.Err(t, err, nil)
	be.Equal(t, backing.name, filepath.Join("root", "dir"))
}

func TestHostReadDirDoesNotRequireChildStat(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "read-only")
	be.Err(t, os.Mkdir(dir, 0o700), nil)
	be.Err(t, os.WriteFile(filepath.Join(dir, "child.txt"), []byte("content"), 0o600), nil)
	be.Err(t, os.Chmod(dir, 0o400), nil)
	defer os.Chmod(dir, 0o700)
	if _, err := os.Stat(filepath.Join(dir, "child.txt")); err == nil {
		t.Skip("platform permits child lookup without directory search permission")
	}

	host := NewHost(afero.NewOsFs(), root)
	entries, err := host.ReadDir("read-only")
	be.Err(t, err, nil)
	be.Equal(t, len(entries), 1)
	be.Equal(t, entries[0].Name(), "child.txt")
}

func TestHostTruncateUsesRootedPath(t *testing.T) {
	root := t.TempDir()
	be.Err(t, os.WriteFile(filepath.Join(root, "file.txt"), []byte("content"), 0o600), nil)
	host := NewHost(afero.NewOsFs(), root)

	be.Err(t, host.Truncate("file.txt", 2), nil)
	content, err := os.ReadFile(filepath.Join(root, "file.txt"))
	be.Err(t, err, nil)
	be.Equal(t, string(content), "co")
}

func TestHostSupportsParentTraversal(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "base")
	sibling := filepath.Join(root, "sibling")
	be.Err(t, os.Mkdir(base, 0o755), nil)
	be.Err(t, os.Mkdir(sibling, 0o755), nil)
	be.Err(t, os.WriteFile(filepath.Join(sibling, "file.txt"), []byte("content"), 0o644), nil)

	host := NewHost(afero.NewOsFs(), base)
	entries, err := ReadDir(host, "../sibling")
	be.Err(t, err, nil)
	be.Equal(t, len(entries), 1)
	be.Equal(t, entries[0].Name(), "file.txt")

	_, err = Lstat(host, "../sibling/file.txt")
	be.Err(t, err, nil)
}

func TestOpenReadRejectsDirectory(t *testing.T) {
	filesystems := []afero.Fs{
		NewHost(afero.NewOsFs(), t.TempDir()),
		NewIOFS(fstest.MapFS{"file.txt": {Data: []byte("content")}}),
	}
	for _, fsys := range filesystems {
		file, err := OpenRead(fsys, ".")
		be.Equal(t, file, afero.File(nil))
		be.Equal(t, errors.Is(err, syscall.EISDIR), true)
		pathErr, ok := errors.AsType[*fs.PathError](err)
		be.Equal(t, ok, true)
		be.Equal(t, pathErr.Op, "open")
	}
}
