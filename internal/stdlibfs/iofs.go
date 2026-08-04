package stdlibfs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

// IOFS adapts a read-only io/fs.FS to Afero while preserving Dyson's path,
// descriptor-open, and no-follow requirements.
type IOFS struct {
	afero.FromIOFS
}

var (
	_ afero.Fs         = IOFS{}
	_ afero.Lstater    = IOFS{}
	_ afero.LinkReader = IOFS{}
)

// NewIOFS returns a read-only Afero filesystem backed by fsys.
func NewIOFS(fsys fs.FS) IOFS {
	return IOFS{FromIOFS: afero.FromIOFS{FS: fsys}}
}

// Open accepts Python-style leading current-directory components.
func (f IOFS) Open(name string) (afero.File, error) {
	normalized, err := ioFSPath("open", name)
	if err != nil {
		return nil, err
	}
	return f.FromIOFS.Open(normalized)
}

// ReadDir preserves a source filesystem's optimized or non-file-based
// directory traversal while applying Dyson's path normalization.
func (f IOFS) ReadDir(name string) ([]fs.DirEntry, error) {
	normalized, err := ioFSPath("readdir", name)
	if err != nil {
		return nil, err
	}
	return fs.ReadDir(f.FromIOFS.FS, normalized)
}

// OpenFile allows only a genuine read-only descriptor open. Afero's bare
// FromIOFS ignores flags, which can otherwise make requested writes appear to succeed.
func (f IOFS) OpenFile(name string, flag int, _ os.FileMode) (afero.File, error) {
	normalized, err := ioFSPath("open", name)
	if err != nil {
		return nil, err
	}
	if flag != os.O_RDONLY {
		return nil, &os.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return f.FromIOFS.Open(normalized)
}

// Stat accepts Python-style leading current-directory components.
func (f IOFS) Stat(name string) (os.FileInfo, error) {
	normalized, err := ioFSPath("stat", name)
	if err != nil {
		return nil, err
	}
	return fs.Stat(f.FromIOFS.FS, normalized)
}

// LstatIfPossible uses io/fs.ReadLinkFS rather than falling back to Stat.
func (f IOFS) LstatIfPossible(name string) (os.FileInfo, bool, error) {
	normalized, err := ioFSPath("lstat", name)
	if err != nil {
		return nil, true, err
	}
	readLinkFS, ok := f.FromIOFS.FS.(fs.ReadLinkFS)
	if !ok {
		return nil, false, &os.PathError{Op: "lstat", Path: name, Err: errors.ErrUnsupported}
	}
	info, err := readLinkFS.Lstat(normalized)
	return info, true, err
}

// ReadlinkIfPossible delegates to io/fs.ReadLinkFS when available.
func (f IOFS) ReadlinkIfPossible(name string) (string, error) {
	normalized, err := ioFSPath("readlink", name)
	if err != nil {
		return "", err
	}
	readLinkFS, ok := f.FromIOFS.FS.(fs.ReadLinkFS)
	if !ok {
		return "", &os.PathError{Op: "readlink", Path: name, Err: afero.ErrNoReadlink}
	}
	return readLinkFS.ReadLink(normalized)
}

func ioFSPath(op, name string) (string, error) {
	normalized := filepath.ToSlash(name)
	for strings.HasPrefix(normalized, "./") {
		normalized = strings.TrimLeft(normalized[2:], "/")
	}
	if normalized == "" {
		normalized = "."
	}
	if !fs.ValidPath(normalized) {
		return "", &os.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	return normalized, nil
}
