package xfs

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// FS is the minimum filesystem surface used by Starlark stdlib modules.
// Implementations own their path and containment policy.
type FS interface {
	ReadDir(name string) ([]fs.DirEntry, error)
	Stat(name string) (fs.FileInfo, error)
	Lstat(name string) (fs.FileInfo, error)
}

// MutFS optionally adds filesystem mutation operations.
type MutFS interface {
	Mkdir(name string, perm fs.FileMode) error
	Remove(name string) error
	Rename(oldname, newname string) error
	Chmod(name string, mode fs.FileMode) error
	Chown(name string, uid, gid int) error
	Chtimes(name string, atime, mtime time.Time) error
	Truncate(name string, size int64) error
	Link(oldname, newname string) error
	Symlink(oldname, newname string) error
	Readlink(name string) (string, error)
}

// File is a file handle used by descriptor-style Starlark operations.
type File interface {
	io.Reader
	io.Writer
	io.Closer
	Sync() error
	Truncate(size int64) error
}

// OpenFS optionally adds descriptor-style file I/O.
type OpenFS interface {
	OpenFile(name string, flag int, perm fs.FileMode) (File, error)
}

// FileDescriptors stores process-local file descriptors shared by Starlark stdlib modules.
type FileDescriptors struct {
	mu    sync.Mutex
	next  int
	files map[int]File
}

// NewFileDescriptors returns an empty descriptor table whose first allocated descriptor is 3.
func NewFileDescriptors() *FileDescriptors {
	return &FileDescriptors{next: 3, files: map[int]File{}}
}

// Store records file and returns its descriptor number.
func (f *FileDescriptors) Store(file File) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	fd := f.next
	f.next++
	f.files[fd] = file
	return fd
}

// Lookup returns the file for fd.
func (f *FileDescriptors) Lookup(fd int) (File, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	file, ok := f.files[fd]
	return file, ok
}

// Delete removes fd from the table.
func (f *FileDescriptors) Delete(fd int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.files, fd)
}

// PathFS optionally adds absolute and canonical path resolution.
type PathFS interface {
	Abs(name string) (string, error)
	Realpath(name string) (string, error)
}

// SameFileFS optionally adds path identity checks.
type SameFileFS interface {
	SameFile(a, b string) (bool, error)
}

// Usage describes filesystem capacity around a path.
type Usage struct {
	Total uint64
	Used  uint64
	Free  uint64
}

// UsageFS optionally adds filesystem capacity reporting.
type UsageFS interface {
	DiskUsage(name string) (Usage, error)
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

// Mkdir creates a host directory.
func (f HostFS) Mkdir(name string, perm fs.FileMode) error {
	return os.Mkdir(f.resolve(name), perm)
}

// Remove removes a host path.
func (f HostFS) Remove(name string) error {
	return os.Remove(f.resolve(name))
}

// Rename renames a host path.
func (f HostFS) Rename(oldname, newname string) error {
	return os.Rename(f.resolve(oldname), f.resolve(newname))
}

// Chmod changes host path permissions.
func (f HostFS) Chmod(name string, mode fs.FileMode) error {
	return os.Chmod(f.resolve(name), mode)
}

// Chown changes host path ownership.
func (f HostFS) Chown(name string, uid, gid int) error {
	return os.Chown(f.resolve(name), uid, gid)
}

// Chtimes changes host path access and modification times.
func (f HostFS) Chtimes(name string, atime, mtime time.Time) error {
	return os.Chtimes(f.resolve(name), atime, mtime)
}

// Truncate changes host file size.
func (f HostFS) Truncate(name string, size int64) error {
	return os.Truncate(f.resolve(name), size)
}

// Link creates a hard link.
func (f HostFS) Link(oldname, newname string) error {
	return os.Link(f.resolve(oldname), f.resolve(newname))
}

// Symlink creates a symbolic link.
func (f HostFS) Symlink(oldname, newname string) error {
	return os.Symlink(oldname, f.resolve(newname))
}

// Readlink reads a symbolic link target.
func (f HostFS) Readlink(name string) (string, error) {
	return os.Readlink(f.resolve(name))
}

// OpenFile opens a host file.
func (f HostFS) OpenFile(name string, flag int, perm fs.FileMode) (File, error) {
	return os.OpenFile(f.resolve(name), flag, perm)
}

// Abs resolves name to an absolute host path.
func (f HostFS) Abs(name string) (string, error) {
	return filepath.Abs(f.resolve(name))
}

// Realpath resolves symbolic links in name.
func (f HostFS) Realpath(name string) (string, error) {
	return filepath.EvalSymlinks(f.resolve(name))
}

// SameFile reports whether two paths identify the same host file.
func (f HostFS) SameFile(a, b string) (bool, error) {
	ia, err := os.Stat(f.resolve(a))
	if err != nil {
		return false, err
	}
	ib, err := os.Stat(f.resolve(b))
	if err != nil {
		return false, err
	}
	return os.SameFile(ia, ib), nil
}

// DiskUsage returns host filesystem capacity around name.
func (f HostFS) DiskUsage(name string) (Usage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(f.resolve(name), &stat); err != nil {
		return Usage{}, err
	}
	blockSize := uint64(stat.Bsize)
	total := stat.Blocks * blockSize
	free := stat.Bavail * blockSize
	return Usage{Total: total, Used: total - free, Free: free}, nil
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
