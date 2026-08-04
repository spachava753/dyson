package stdlibfs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/afero"
)

// ErrNotConfigured reports an attempted operation without filesystem access.
var ErrNotConfigured = errors.New("filesystem operations are not configured")

// Unavailable is an afero filesystem whose operations fail closed.
type Unavailable struct{}

var _ afero.Fs = Unavailable{}

func (Unavailable) Create(string) (afero.File, error)  { return nil, ErrNotConfigured }
func (Unavailable) Mkdir(string, os.FileMode) error    { return ErrNotConfigured }
func (Unavailable) MkdirAll(string, os.FileMode) error { return ErrNotConfigured }
func (Unavailable) Open(string) (afero.File, error)    { return nil, ErrNotConfigured }
func (Unavailable) OpenFile(string, int, os.FileMode) (afero.File, error) {
	return nil, ErrNotConfigured
}
func (Unavailable) Remove(string) error                        { return ErrNotConfigured }
func (Unavailable) RemoveAll(string) error                     { return ErrNotConfigured }
func (Unavailable) Rename(string, string) error                { return ErrNotConfigured }
func (Unavailable) Stat(string) (os.FileInfo, error)           { return nil, ErrNotConfigured }
func (Unavailable) Name() string                               { return "unavailable" }
func (Unavailable) Chmod(string, os.FileMode) error            { return ErrNotConfigured }
func (Unavailable) Chown(string, int, int) error               { return ErrNotConfigured }
func (Unavailable) Chtimes(string, time.Time, time.Time) error { return ErrNotConfigured }

// Host resolves relative paths from Root while retaining normal host semantics
// for absolute paths and parent traversal.
type Host struct {
	FS   afero.Fs
	Root string
}

// NewHost returns a rooted view over fsys using Dyson's host path policy.
// Host-only extensions resolve paths from Root; Truncate uses an OS path only
// for Afero's OsFs so other injected backends can retain generic behavior.
func NewHost(fsys afero.Fs, root string) Host {
	return Host{FS: fsys, Root: root}
}

var (
	_ afero.Fs          = Host{}
	_ afero.Lstater     = Host{}
	_ afero.Linker      = Host{}
	_ afero.LinkReader  = Host{}
	_ readDirFileSystem = Host{}
)

func (h Host) Create(name string) (afero.File, error) {
	return h.FS.Create(h.resolve(name))
}

func (h Host) Mkdir(name string, perm os.FileMode) error {
	return h.FS.Mkdir(h.resolve(name), perm)
}

func (h Host) MkdirAll(path string, perm os.FileMode) error {
	return h.FS.MkdirAll(h.resolve(path), perm)
}

func (h Host) Open(name string) (afero.File, error) {
	return h.FS.Open(h.resolve(name))
}

// ReadDir preserves host directory entries without forcing per-child FileInfo.
func (h Host) ReadDir(name string) ([]fs.DirEntry, error) {
	name = h.resolve(name)
	switch h.FS.(type) {
	case afero.OsFs, *afero.OsFs:
		return os.ReadDir(name)
	default:
		return ReadDir(h.FS, name)
	}
}

func (h Host) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	return h.FS.OpenFile(h.resolve(name), flag, perm)
}

func (h Host) Remove(name string) error {
	return h.FS.Remove(h.resolve(name))
}

func (h Host) RemoveAll(path string) error {
	return h.FS.RemoveAll(h.resolve(path))
}

func (h Host) Rename(oldname, newname string) error {
	return h.FS.Rename(h.resolve(oldname), h.resolve(newname))
}

func (h Host) Stat(name string) (os.FileInfo, error) {
	return h.FS.Stat(h.resolve(name))
}

func (Host) Name() string { return "dyson-host" }

func (h Host) Chmod(name string, mode os.FileMode) error {
	return h.FS.Chmod(h.resolve(name), mode)
}

func (h Host) Chown(name string, uid, gid int) error {
	return h.FS.Chown(h.resolve(name), uid, gid)
}

func (h Host) Chtimes(name string, atime, mtime time.Time) error {
	return h.FS.Chtimes(h.resolve(name), atime, mtime)
}

type pathTruncater interface {
	Truncate(name string, size int64) error
}

// Truncate changes a path's size without opening it when the backend supports
// path truncation. Afero's OsFs is handled explicitly because it omits that method.
func (h Host) Truncate(name string, size int64) error {
	name = h.resolve(name)
	if truncater, ok := h.FS.(pathTruncater); ok {
		return truncater.Truncate(name, size)
	}
	switch h.FS.(type) {
	case afero.OsFs, *afero.OsFs:
		return os.Truncate(name, size)
	default:
		return &os.PathError{Op: "truncate", Path: name, Err: errors.ErrUnsupported}
	}
}

// LstatIfPossible implements afero.Lstater through the configured backend.
func (h Host) LstatIfPossible(name string) (os.FileInfo, bool, error) {
	name = h.resolve(name)
	if lstater, ok := h.FS.(afero.Lstater); ok {
		return lstater.LstatIfPossible(name)
	}
	return nil, false, &os.PathError{Op: "lstat", Path: name, Err: errors.ErrUnsupported}
}

// SymlinkIfPossible implements afero.Linker through the configured backend.
func (h Host) SymlinkIfPossible(oldname, newname string) error {
	linker, ok := h.FS.(afero.Linker)
	if !ok {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: afero.ErrNoSymlink}
	}
	return linker.SymlinkIfPossible(oldname, h.resolve(newname))
}

// ReadlinkIfPossible implements afero.LinkReader through the configured backend.
func (h Host) ReadlinkIfPossible(name string) (string, error) {
	reader, ok := h.FS.(afero.LinkReader)
	if !ok {
		return "", &os.PathError{Op: "readlink", Path: name, Err: afero.ErrNoReadlink}
	}
	return reader.ReadlinkIfPossible(h.resolve(name))
}

// Link creates a hard link between host paths.
func (h Host) Link(oldname, newname string) error {
	return os.Link(h.resolve(oldname), h.resolve(newname))
}

// Abs resolves name to an absolute host path.
func (h Host) Abs(name string) (string, error) {
	return filepath.Abs(h.resolve(name))
}

// Realpath resolves symbolic links while tolerating missing components, matching
// Python's default strict=False behavior.
func (h Host) Realpath(name string) (string, error) {
	path, err := h.Abs(name)
	if err != nil {
		return "", err
	}
	resolved, pending := splitAbsolutePath(path)
	followedLinks := 0
	for len(pending) > 0 {
		part := pending[0]
		pending = pending[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			resolved = filepath.Dir(resolved)
			continue
		}

		candidate := filepath.Join(resolved, part)
		info, err := Lstat(h.FS, candidate)
		if errors.Is(err, errors.ErrUnsupported) {
			return "", err
		}
		if err != nil {
			return joinPathComponents(candidate, pending), nil
		}
		if info.Mode()&fs.ModeSymlink == 0 {
			resolved = candidate
			continue
		}
		followedLinks++
		if followedLinks > 40 {
			return joinPathComponents(candidate, pending), nil
		}
		reader, ok := h.FS.(afero.LinkReader)
		if !ok {
			return joinPathComponents(candidate, pending), nil
		}
		target, err := reader.ReadlinkIfPossible(candidate)
		if err != nil {
			return joinPathComponents(candidate, pending), nil
		}
		target = filepath.FromSlash(target)
		if filepath.IsAbs(target) {
			var targetParts []string
			resolved, targetParts = splitAbsolutePath(target)
			pending = append(targetParts, pending...)
		} else {
			pending = append(splitPathComponents(target), pending...)
		}
	}
	return filepath.Clean(resolved), nil
}

func splitAbsolutePath(path string) (string, []string) {
	path = filepath.Clean(path)
	volume := filepath.VolumeName(path)
	root := volume + string(filepath.Separator)
	return root, splitPathComponents(strings.TrimPrefix(path, root))
}

func splitPathComponents(path string) []string {
	var components []string
	for component := range strings.SplitSeq(path, string(filepath.Separator)) {
		if component != "" {
			components = append(components, component)
		}
	}
	return components
}

func joinPathComponents(path string, components []string) string {
	for _, component := range components {
		path = filepath.Join(path, component)
	}
	return filepath.Clean(path)
}

// SameFile reports whether two paths identify the same host file.
func (h Host) SameFile(a, b string) (bool, error) {
	first, err := h.FS.Stat(h.resolve(a))
	if err != nil {
		return false, err
	}
	second, err := h.FS.Stat(h.resolve(b))
	if err != nil {
		return false, err
	}
	return os.SameFile(first, second), nil
}

// Usage describes filesystem capacity around a path.
type Usage struct {
	Total uint64
	Used  uint64
	Free  uint64
}

// DiskUsage returns host filesystem capacity around name.
func (h Host) DiskUsage(name string) (Usage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(h.resolve(name), &stat); err != nil {
		return Usage{}, err
	}
	blockSize := uint64(stat.Bsize)
	total := stat.Blocks * blockSize
	free := stat.Bavail * blockSize
	return Usage{Total: total, Used: total - free, Free: free}, nil
}

func (h Host) resolve(name string) string {
	if name == "" {
		name = "."
	}
	name = filepath.FromSlash(name)
	if filepath.IsAbs(name) {
		return name
	}
	root := h.Root
	if root == "" {
		root = "."
	}
	return filepath.Join(root, name)
}

type readDirFileSystem interface {
	ReadDir(name string) ([]fs.DirEntry, error)
}

// ReadDir returns directory entries, preferring a backend's direct operation
// before adapting Afero's os.FileInfo results.
func ReadDir(fsys afero.Fs, name string) ([]fs.DirEntry, error) {
	if reader, ok := fsys.(readDirFileSystem); ok {
		return reader.ReadDir(name)
	}
	infos, err := afero.ReadDir(fsys, name)
	if err != nil {
		return nil, err
	}
	entries := make([]fs.DirEntry, len(infos))
	for i, info := range infos {
		entries[i] = fs.FileInfoToDirEntry(info)
	}
	return entries, nil
}

// Lstat uses Afero's optional no-follow operation and fails when unavailable
// or when the backend reports that it fell back to Stat.
func Lstat(fsys afero.Fs, name string) (os.FileInfo, error) {
	lstater, ok := fsys.(afero.Lstater)
	if !ok {
		return nil, &os.PathError{Op: "lstat", Path: name, Err: errors.ErrUnsupported}
	}
	info, usedLstat, err := lstater.LstatIfPossible(name)
	if !usedLstat {
		return nil, &os.PathError{Op: "lstat", Path: name, Err: errors.ErrUnsupported}
	}
	return info, err
}

// OpenRead opens a regular file for reading and rejects directories eagerly.
func OpenRead(fsys afero.Fs, name string) (afero.File, error) {
	file, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if info.IsDir() {
		_ = file.Close()
		return nil, &os.PathError{Op: "open", Path: name, Err: syscall.EISDIR}
	}
	return file, nil
}
