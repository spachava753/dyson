package os

import (
	"fmt"

	"github.com/spachava753/dyson/internal/xfs"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

type filesystem struct {
	fsys xfs.FS
}

func makePrimitiveModule(fsys xfs.FS) *starlarkstruct.Module {
	filesystem := filesystem{fsys: fsys}
	builtin := func(member, name string) *starlark.Builtin {
		return starlark.NewBuiltin(ModuleName+name, filesystem.notImplemented(member))
	}
	return &starlarkstruct.Module{
		Name: ModuleName + "._primitive",
		Members: starlark.StringDict{
			"getcwd":        builtin("getcwd", ".getcwd"),
			"chdir":         builtin("chdir", ".chdir"),
			"get_exec_path": builtin("get_exec_path", ".get_exec_path"),
			"getpid":        builtin("getpid", ".getpid"),
			"getppid":       builtin("getppid", ".getppid"),
			"kill":          builtin("kill", ".kill"),
			"system":        builtin("system", ".system"),

			"environ":  builtin("environ", ".environ"),
			"getenv":   builtin("getenv", ".getenv"),
			"putenv":   builtin("putenv", ".putenv"),
			"unsetenv": builtin("unsetenv", ".unsetenv"),

			"listdir": starlark.NewBuiltin(ModuleName+".listdir", filesystem.listdir),
			"scandir": builtin("scandir", ".scandir"),
			"walk":    builtin("walk", ".walk"),
			"stat":    builtin("stat", ".stat"),
			"lstat":   builtin("lstat", ".lstat"),
			"access":  builtin("access", ".access"),

			"mkdir":      builtin("mkdir", ".mkdir"),
			"makedirs":   builtin("makedirs", ".makedirs"),
			"rmdir":      builtin("rmdir", ".rmdir"),
			"removedirs": builtin("removedirs", ".removedirs"),
			"remove":     builtin("remove", ".remove"),
			"unlink":     builtin("unlink", ".unlink"),
			"rename":     builtin("rename", ".rename"),
			"replace":    builtin("replace", ".replace"),
			"renames":    builtin("renames", ".renames"),
			"chmod":      builtin("chmod", ".chmod"),
			"chown":      builtin("chown", ".chown"),
			"utime":      builtin("utime", ".utime"),
			"truncate":   builtin("truncate", ".truncate"),
			"link":       builtin("link", ".link"),
			"symlink":    builtin("symlink", ".symlink"),
			"readlink":   builtin("readlink", ".readlink"),

			"open":      builtin("open", ".open"),
			"close":     builtin("close", ".close"),
			"read":      builtin("read", ".read"),
			"write":     builtin("write", ".write"),
			"fsync":     builtin("fsync", ".fsync"),
			"ftruncate": builtin("ftruncate", ".ftruncate"),

			"getuid":    builtin("getuid", ".getuid"),
			"geteuid":   builtin("geteuid", ".geteuid"),
			"getgid":    builtin("getgid", ".getgid"),
			"getegid":   builtin("getegid", ".getegid"),
			"getgroups": builtin("getgroups", ".getgroups"),
			"umask":     builtin("umask", ".umask"),

			"path_abspath":    builtin("path_abspath", ".path.abspath"),
			"path_exists":     builtin("path_exists", ".path.exists"),
			"path_lexists":    starlark.NewBuiltin(ModuleName+".path.lexists", filesystem.pathLexists),
			"path_expanduser": builtin("path_expanduser", ".path.expanduser"),
			"path_expandvars": builtin("path_expandvars", ".path.expandvars"),
			"path_getatime":   builtin("path_getatime", ".path.getatime"),
			"path_getmtime":   builtin("path_getmtime", ".path.getmtime"),
			"path_getctime":   builtin("path_getctime", ".path.getctime"),
			"path_getsize":    builtin("path_getsize", ".path.getsize"),
			"path_isdir":      starlark.NewBuiltin(ModuleName+".path.isdir", filesystem.pathIsDir),
			"path_isfile":     builtin("path_isfile", ".path.isfile"),
			"path_islink":     builtin("path_islink", ".path.islink"),
			"path_ismount":    builtin("path_ismount", ".path.ismount"),
			"path_realpath":   builtin("path_realpath", ".path.realpath"),
			"path_samefile":   builtin("path_samefile", ".path.samefile"),
		},
	}
}

func (f filesystem) notImplemented(member string) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		panic(fmt.Sprintf("%s: primitive %s is not implemented", fn.Name(), member))
	}
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
