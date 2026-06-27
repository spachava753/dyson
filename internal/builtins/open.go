package builtins

import (
	"fmt"
	"io"
	"os"
	"strings"

	"go.starlark.net/starlark"
)

// openBuiltin implements the Starlark open builtin for read-only text and
// binary files, returning a small file object with read, close, and closed.
//
// It mirrors the supported subset of Python's open:
// https://docs.python.org/3/library/functions.html#open
func openBuiltin(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path, mode string
	mode = "r"
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "file", &path, "mode?", &mode); err != nil {
		return nil, err
	}
	if path == "" || strings.ContainsRune(path, 0) {
		return nil, fmt.Errorf("%s: invalid path", fn.Name())
	}
	binary := false
	switch mode {
	case "r", "rt":
	case "rb":
		binary = true
	default:
		return nil, fmt.Errorf("%s: unsupported mode %q", fn.Name(), mode)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn.Name(), err)
	}
	return &fileValue{path: path, file: file, binary: binary}, nil
}

// fileValue is the Starlark-visible file object returned by openBuiltin.
type fileValue struct {
	path   string
	file   *os.File
	binary bool
	closed bool
}

// String returns a Python-like representation of the open file object.
func (f *fileValue) String() string { return fmt.Sprintf("<open file %q>", f.path) }

// Type reports the Starlark-visible type name for open file objects.
func (f *fileValue) Type() string { return "file" }

// Freeze marks the file value immutable for Starlark's shared-value semantics;
// it does not close or otherwise mutate the underlying file handle.
func (f *fileValue) Freeze() {}

// Truth reports that open file objects are truthy until closed.
func (f *fileValue) Truth() starlark.Bool {
	return starlark.Bool(!f.closed)
}

// Hash rejects hashing because file objects are mutable handle wrappers.
func (f *fileValue) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: file") }

// Attr exposes Python-like file attributes and bound methods.
func (f *fileValue) Attr(name string) (starlark.Value, error) {
	switch name {
	case "read":
		return starlark.NewBuiltin("file.read", f.read), nil
	case "close":
		return starlark.NewBuiltin("file.close", f.close), nil
	case "closed":
		return starlark.Bool(f.closed), nil
	}
	return nil, nil
}

// AttrNames returns the names discoverable on open file values.
func (f *fileValue) AttrNames() []string { return []string{"close", "closed", "read"} }

// read implements file.read, returning str for text mode and bytes for binary
// mode from the file's current offset through EOF.
func (f *fileValue) read(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	if f.closed {
		return nil, fmt.Errorf("%s: I/O operation on closed file", fn.Name())
	}
	data, err := io.ReadAll(f.file)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn.Name(), err)
	}
	if f.binary {
		return starlark.Bytes(string(data)), nil
	}
	return starlark.String(string(data)), nil
}

// close implements file.close, closing the underlying file handle and returning
// None. Repeated close calls are accepted, matching Python's idempotent close.
func (f *fileValue) close(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	if f.closed {
		return starlark.None, nil
	}
	f.closed = true
	if err := f.file.Close(); err != nil {
		return nil, fmt.Errorf("%s: %w", fn.Name(), err)
	}
	return starlark.None, nil
}
