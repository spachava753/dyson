package stdlibfs

import (
	"errors"
	"io/fs"
	"os"
	"testing"
	"testing/fstest"

	"github.com/nalgeon/be"
	"github.com/spf13/afero"
)

type readDirFSWithPlainOpen struct {
	source fstest.MapFS
}

func (f readDirFSWithPlainOpen) Open(name string) (fs.File, error) {
	file, err := f.source.Open(name)
	if err != nil {
		return nil, err
	}
	return struct{ fs.File }{File: file}, nil
}

func (f readDirFSWithPlainOpen) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(f.source, name)
}

func TestIOFSPreservesSourceReadDirFS(t *testing.T) {
	fsys := NewIOFS(readDirFSWithPlainOpen{source: fstest.MapFS{
		"a.txt": {Data: []byte("a")},
		"b.txt": {Data: []byte("b")},
	}})

	entries, err := ReadDir(fsys, "./")
	be.Err(t, err, nil)
	be.Equal(t, len(entries), 2)
	be.Equal(t, entries[0].Name(), "a.txt")
	be.Equal(t, entries[1].Name(), "b.txt")
}

func TestIOFSNormalizesLeadingCurrentDirectoryComponents(t *testing.T) {
	fsys := NewIOFS(fstest.MapFS{
		"file.txt": {Data: []byte("content")},
	})

	content, err := afero.ReadFile(fsys, ".//./file.txt")
	be.Err(t, err, nil)
	be.Equal(t, string(content), "content")

	_, err = fsys.Open("file.txt/")
	be.Equal(t, errors.Is(err, fs.ErrInvalid), true)
}

func TestIOFSOpenFileRejectsNonReadOnlyFlags(t *testing.T) {
	fsys := NewIOFS(fstest.MapFS{
		"file.txt": {Data: []byte("content")},
	})

	file, err := fsys.OpenFile("file.txt", os.O_WRONLY|os.O_TRUNC, 0)
	be.Equal(t, file, afero.File(nil))
	be.Equal(t, errors.Is(err, fs.ErrPermission), true)

	file, err = fsys.OpenFile("./file.txt", os.O_RDONLY, 0)
	be.Err(t, err, nil)
	be.Err(t, file.Close(), nil)
}

func TestIOFSLstatUsesReadLinkFS(t *testing.T) {
	fsys := NewIOFS(fstest.MapFS{
		"target.txt": {Data: []byte("content")},
		"link.txt":   {Data: []byte("target.txt"), Mode: fs.ModeSymlink},
	})

	info, err := Lstat(fsys, "./link.txt")
	be.Err(t, err, nil)
	be.Equal(t, info.Mode()&fs.ModeSymlink != 0, true)
	target, err := fsys.ReadlinkIfPossible(".//link.txt")
	be.Err(t, err, nil)
	be.Equal(t, target, "target.txt")
}

type fsWithoutReadLink struct {
	fs.FS
}

func TestIOFSLstatFailsWithoutReadLinkFS(t *testing.T) {
	fsys := NewIOFS(fsWithoutReadLink{FS: fstest.MapFS{
		"file.txt": {Data: []byte("content")},
	}})

	_, err := Lstat(fsys, "file.txt")
	be.Equal(t, errors.Is(err, errors.ErrUnsupported), true)
}
