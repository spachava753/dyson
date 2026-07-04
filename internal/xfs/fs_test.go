package xfs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/nalgeon/be"
)

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

func TestIOFSRejectsParentTraversal(t *testing.T) {
	fsys := IOFS{FS: fstest.MapFS{
		"file.txt": {Data: []byte("content")},
	}}

	_, err := fsys.ReadDir("..")
	be.Equal(t, errors.Is(err, fs.ErrInvalid), true)

	_, err = fsys.Stat("../file.txt")
	be.Equal(t, errors.Is(err, fs.ErrInvalid), true)
}
