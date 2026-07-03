package os

import (
	"fmt"

	"github.com/spachava753/dyson/internal/xfs"
	"go.starlark.net/starlark"
)

type filesystem struct {
	fsys xfs.FS
}

// listdir implements os.listdir for string paths.
//
// It mirrors the supported subset of Python's os.listdir:
// https://docs.python.org/3/library/os.html#os.listdir
func (f filesystem) listdir(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	path := "."
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path?", &path); err != nil {
		return nil, err
	}

	entries, err := f.fsys.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn.Name(), err)
	}
	items := make([]starlark.Value, len(entries))
	for i, entry := range entries {
		items[i] = starlark.String(entry.Name())
	}
	return starlark.NewList(items), nil
}

// pathLexists implements os.path.lexists for string paths.
//
// It mirrors the supported subset of Python's os.path.lexists. Broken-symlink
// behavior depends on whether the backing FS has a real Lstat implementation:
// https://docs.python.org/3/library/os.path.html#os.path.lexists
func (f filesystem) pathLexists(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	_, err := f.fsys.Lstat(path)
	return starlark.Bool(err == nil), nil
}

// pathIsDir implements os.path.isdir for string paths.
//
// It mirrors the supported subset of Python's os.path.isdir:
// https://docs.python.org/3/library/os.path.html#os.path.isdir
func (f filesystem) pathIsDir(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}

	info, err := f.fsys.Stat(path)
	return starlark.Bool(err == nil && info.IsDir()), nil
}
