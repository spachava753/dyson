package stdlibfs

import (
	"errors"
	"fmt"
	"sync"

	"github.com/spf13/afero"
)

var errFileDescriptorsClosed = errors.New("file descriptor table is closed")

// FileDescriptors stores process-local Afero files shared by Starlark modules.
type FileDescriptors struct {
	mu     sync.Mutex
	next   int
	files  map[int]afero.File
	closed bool
}

// NewFileDescriptors returns an empty descriptor table whose first descriptor is 3.
func NewFileDescriptors() *FileDescriptors {
	return &FileDescriptors{next: 3, files: map[int]afero.File{}}
}

// Store records file and returns its descriptor number. A closed table leaves file open.
func (f *FileDescriptors) Store(file afero.File) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return 0, errFileDescriptorsClosed
	}
	fd := f.next
	f.next++
	f.files[fd] = file
	return fd, nil
}

// Lookup returns the file for fd.
func (f *FileDescriptors) Lookup(fd int) (afero.File, bool) {
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

// Close closes every stored file and prevents future stores. It is idempotent.
func (f *FileDescriptors) Close() error {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return nil
	}
	f.closed = true
	files := f.files
	f.files = nil
	f.mu.Unlock()

	var closeErrors []error
	for fd, file := range files {
		if err := file.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close file descriptor %d: %w", fd, err))
		}
	}
	return errors.Join(closeErrors...)
}
