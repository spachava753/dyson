package xfs

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
)

type descriptorTestFile struct {
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

func TestHostFSSupportsParentTraversal(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "base")
	sibling := filepath.Join(root, "sibling")
	be.Err(t, os.Mkdir(base, 0o755), nil)
	be.Err(t, os.Mkdir(sibling, 0o755), nil)
	be.Err(t, os.WriteFile(filepath.Join(sibling, "file.txt"), []byte("content"), 0o644), nil)

	entries, err := HostFS{Root: base}.ReadDir("../sibling")
	be.Err(t, err, nil)
	be.Equal(t, len(entries), 1)
	be.Equal(t, entries[0].Name(), "file.txt")

	_, err = HostFS{Root: base}.Lstat("../sibling/file.txt")
	be.Err(t, err, nil)
}

func TestHostFSOpenReadRejectsDirectory(t *testing.T) {
	root := t.TempDir()

	file, err := (HostFS{Root: root}).OpenRead(".")
	be.Equal(t, file, ReadFile(nil))
	be.Equal(t, errors.Is(err, syscall.EISDIR), true)
	pathErr, ok := errors.AsType[*os.PathError](err)
	be.Equal(t, ok, true)
	be.Equal(t, pathErr.Op, "open")
	be.Equal(t, pathErr.Path, root)
}

func TestIOFSOpenReadRejectsDirectory(t *testing.T) {
	fsys := IOFS{FS: fstest.MapFS{
		"file.txt": {Data: []byte("content")},
	}}

	file, err := fsys.OpenRead(".")
	be.Equal(t, file, ReadFile(nil))
	be.Equal(t, errors.Is(err, syscall.EISDIR), true)
	pathErr, ok := errors.AsType[*fs.PathError](err)
	be.Equal(t, ok, true)
	be.Equal(t, pathErr.Op, "open")
	be.Equal(t, pathErr.Path, ".")
}

func TestIOFSRejectsParentTraversal(t *testing.T) {
	fsys := IOFS{FS: fstest.MapFS{
		"file.txt": {Data: []byte("content")},
	}}

	_, err := fsys.ReadDir("..")
	be.Equal(t, errors.Is(err, fs.ErrInvalid), true)

	_, err = fsys.Stat("../file.txt")
	be.Equal(t, errors.Is(err, fs.ErrInvalid), true)
}
