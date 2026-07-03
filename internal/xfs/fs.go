package xfs

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FS is the narrow filesystem surface exposed to Starlark stdlib modules.
// Implementations define their own path policy, including whether parent
// traversal such as ../name is allowed.
type FS interface {
	ReadDir(name string) ([]fs.DirEntry, error)
	Stat(name string) (fs.FileInfo, error)
	Lstat(name string) (fs.FileInfo, error)
}

// HostFS exposes the host filesystem below Root using normal OS path semantics.
// Relative paths may use .. and can escape Root; use IOFS for contained io/fs
// semantics instead.
type HostFS struct {
	Root string
}

// ReadDir reads a host directory.
func (f HostFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(f.resolve(name))
}

// Stat returns host file info, following symlinks.
func (f HostFS) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(f.resolve(name))
}

// Lstat returns host file info without following symlinks.
func (f HostFS) Lstat(name string) (fs.FileInfo, error) {
	return os.Lstat(f.resolve(name))
}

func (f HostFS) resolve(name string) string {
	if name == "" {
		name = "."
	}
	name = filepath.FromSlash(name)
	if filepath.IsAbs(name) {
		return name
	}
	root := f.Root
	if root == "" {
		root = "."
	}
	return filepath.Join(root, name)
}

// IOFS adapts an io/fs filesystem to FS. It preserves io/fs containment rules:
// paths must be slash-separated, relative, valid fs paths, so ../ traversal is
// rejected by this adapter.
type IOFS struct {
	FS fs.FS
}

// ReadDir reads a contained io/fs directory.
func (f IOFS) ReadDir(name string) ([]fs.DirEntry, error) {
	name, ok := ioPath(name)
	if !ok {
		return nil, fs.ErrInvalid
	}
	return fs.ReadDir(f.FS, name)
}

// Stat returns contained io/fs file info. Plain io/fs follows the backing
// filesystem's stat semantics.
func (f IOFS) Stat(name string) (fs.FileInfo, error) {
	name, ok := ioPath(name)
	if !ok {
		return nil, fs.ErrInvalid
	}
	return fs.Stat(f.FS, name)
}

// Lstat falls back to Stat because plain io/fs has no lstat operation.
func (f IOFS) Lstat(name string) (fs.FileInfo, error) {
	return f.Stat(name)
}

func ioPath(name string) (string, bool) {
	name = filepath.ToSlash(name)
	for strings.HasPrefix(name, "./") {
		name = strings.TrimPrefix(name, "./")
	}
	if name == "" {
		name = "."
	}
	return name, fs.ValidPath(name)
}
