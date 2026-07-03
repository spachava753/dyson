package os

import (
	goos "os"
	"runtime"

	"github.com/spachava753/dyson/internal/xfs"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ModuleName is the Starlark stdlib module name for Dyson's os compatibility module.
const ModuleName = "os"

// Module is the default Starlark module namespace exposed by load("os.star", "os").
var Module = MakeModule(xfs.HostFS{Root: "."})

// MakeModule returns a Starlark os module backed by fsys. Path behavior is
// defined by fsys, so callers can choose host-style parent traversal or
// contained io/fs semantics.
func MakeModule(fsys xfs.FS) *starlarkstruct.Module {
	filesystem := filesystem{fsys: fsys}
	pathModule := &starlarkstruct.Module{
		Name: ModuleName + ".path",
		Members: starlark.StringDict{
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.abspath
			"abspath": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.basename
			"basename": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.dirname
			"dirname": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.exists
			"exists": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.lexists
			"lexists": starlark.NewBuiltin(ModuleName+".path.lexists", filesystem.pathLexists),
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.expanduser
			"expanduser": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.expandvars
			"expandvars": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.getatime
			"getatime": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.getmtime
			"getmtime": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.getctime
			"getctime": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.getsize
			"getsize": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.isabs
			"isabs": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.isdir
			"isdir": starlark.NewBuiltin(ModuleName+".path.isdir", filesystem.pathIsDir),
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.isfile
			"isfile": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.islink
			"islink": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.ismount
			"ismount": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.join
			"join": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.normpath
			"normpath": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.realpath
			"realpath": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.relpath
			"relpath": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.samefile
			"samefile": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.split
			"split": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.splitdrive
			"splitdrive": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.splitext
			"splitext": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.commonpath
			"commonpath": nil,
			// Python docs: https://docs.python.org/3/library/os.path.html#os.path.supports_unicode_filenames
			"supports_unicode_filenames": starlark.True,
		},
	}

	return &starlarkstruct.Module{
		Name: ModuleName,
		Members: starlark.StringDict{
			// Python docs: https://docs.python.org/3/library/os.html#os.getcwd
			"getcwd": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.chdir
			"chdir": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.get_exec_path
			"get_exec_path": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.getpid
			"getpid": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.getppid
			"getppid": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.kill
			"kill": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.system
			"system": nil,

			// Python docs: https://docs.python.org/3/library/os.html#os.environ
			"environ": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.getenv
			"getenv": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.putenv
			"putenv": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.unsetenv
			"unsetenv": nil,

			// Python docs: https://docs.python.org/3/library/os.html#os.listdir
			"listdir": starlark.NewBuiltin(ModuleName+".listdir", filesystem.listdir),
			// Python docs: https://docs.python.org/3/library/os.html#os.scandir
			"scandir": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.walk
			"walk": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.stat
			"stat": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.lstat
			"lstat": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.access
			"access": nil,

			// Python docs: https://docs.python.org/3/library/os.html#os.mkdir
			"mkdir": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.makedirs
			"makedirs": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.rmdir
			"rmdir": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.removedirs
			"removedirs": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.remove
			"remove": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.unlink
			"unlink": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.rename
			"rename": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.replace
			"replace": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.renames
			"renames": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.chmod
			"chmod": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.chown
			"chown": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.utime
			"utime": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.truncate
			"truncate": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.link
			"link": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.symlink
			"symlink": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.readlink
			"readlink": nil,

			// Python docs: https://docs.python.org/3/library/os.html#os.open
			"open": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.close
			"close": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.read
			"read": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.write
			"write": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.fsync
			"fsync": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.ftruncate
			"ftruncate": nil,

			// Python docs: https://docs.python.org/3/library/os.html#os.getuid
			"getuid": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.geteuid
			"geteuid": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.getgid
			"getgid": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.getegid
			"getegid": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.getgroups
			"getgroups": nil,
			// Python docs: https://docs.python.org/3/library/os.html#os.umask
			"umask": nil,

			"path": pathModule,

			// Python docs: https://docs.python.org/3/library/os.html#os.name
			"name": starlark.String(pythonOSName()),
			// Python docs: https://docs.python.org/3/library/os.html#os.curdir
			"curdir": starlark.String("."),
			// Python docs: https://docs.python.org/3/library/os.html#os.pardir
			"pardir": starlark.String(".."),
			// Python docs: https://docs.python.org/3/library/os.html#os.sep
			"sep": starlark.String("/"),
			// Python docs: https://docs.python.org/3/library/os.html#os.altsep
			"altsep": starlark.None,
			// Python docs: https://docs.python.org/3/library/os.html#os.extsep
			"extsep": starlark.String("."),
			// Python docs: https://docs.python.org/3/library/os.html#os.pathsep
			"pathsep": starlark.String(string(goos.PathListSeparator)),
			// Python docs: https://docs.python.org/3/library/os.html#os.linesep
			"linesep": starlark.String("\n"),
			// Python docs: https://docs.python.org/3/library/os.html#os.defpath
			"defpath": starlark.String("/bin:/usr/bin"),
			// Python docs: https://docs.python.org/3/library/os.html#os.devnull
			"devnull": starlark.String(goos.DevNull),

			// Python docs: https://docs.python.org/3/library/os.html#os.F_OK
			"F_OK": starlark.MakeInt(0),
			// Python docs: https://docs.python.org/3/library/os.html#os.R_OK
			"R_OK": starlark.MakeInt(4),
			// Python docs: https://docs.python.org/3/library/os.html#os.W_OK
			"W_OK": starlark.MakeInt(2),
			// Python docs: https://docs.python.org/3/library/os.html#os.X_OK
			"X_OK": starlark.MakeInt(1),

			// Python docs: https://docs.python.org/3/library/os.html#open-constants
			"O_RDONLY": starlark.MakeInt(goos.O_RDONLY),
			"O_WRONLY": starlark.MakeInt(goos.O_WRONLY),
			"O_RDWR":   starlark.MakeInt(goos.O_RDWR),
			"O_APPEND": starlark.MakeInt(goos.O_APPEND),
			"O_CREAT":  starlark.MakeInt(goos.O_CREATE),
			"O_EXCL":   starlark.MakeInt(goos.O_EXCL),
			"O_SYNC":   starlark.MakeInt(goos.O_SYNC),
			"O_TRUNC":  starlark.MakeInt(goos.O_TRUNC),

			// Python docs: https://docs.python.org/3/library/os.html#os.SEEK_SET
			"SEEK_SET": starlark.MakeInt(0),
			// Python docs: https://docs.python.org/3/library/os.html#os.SEEK_CUR
			"SEEK_CUR": starlark.MakeInt(1),
			// Python docs: https://docs.python.org/3/library/os.html#os.SEEK_END
			"SEEK_END": starlark.MakeInt(2),
		},
	}
}

func pythonOSName() string {
	if runtime.GOOS == "windows" {
		return "nt"
	}
	return "posix"
}
