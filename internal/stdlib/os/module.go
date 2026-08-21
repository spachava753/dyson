package os

import (
	"github.com/spachava753/dyson/internal/stdlibfs"
	"github.com/spachava753/dyson/internal/xos"
	"github.com/spachava753/starlarkx/starlark"
	"github.com/spachava753/starlarkx/starlarkstruct"
	"github.com/spf13/afero"
)

// ModuleName is the Starlark stdlib module name for Dyson's os compatibility module.
const ModuleName = "os"

// Module is the default Starlark module namespace exposed by load("os.star", "os").
var Module = MakeModule(HostConfig("."))

// ModuleConfig groups the host domains used by Dyson's os compatibility module.
type ModuleConfig struct {
	FS              afero.Fs
	Env             xos.Env
	Process         xos.Process
	WorkingDir      xos.WorkingDir
	Platform        xos.Platform
	CommandRunner   xos.CommandRunner
	Clock           xos.Clock
	FileDescriptors *stdlibfs.FileDescriptors
}

// HostConfig returns a module configuration backed by the host filesystem and OS.
func HostConfig(root string) ModuleConfig {
	host := xos.Host{}
	return ModuleConfig{
		FS:            stdlibfs.NewHost(afero.NewOsFs(), root),
		Env:           host,
		Process:       host,
		WorkingDir:    host,
		Platform:      host.Platform(),
		CommandRunner: host,
		Clock:         host,
	}
}

// MakeModule returns a Starlark os module backed by config. File, environment,
// process, working-directory, and platform behavior are controlled by separate
// domain interfaces; missing domains fail at call time.
func MakeModule(config ModuleConfig) *starlarkstruct.Module {
	if config.FS == nil {
		config.FS = stdlibfs.Unavailable{}
	}
	if config.Platform.OSName == "" {
		config.Platform = xos.Host{}.Platform()
	}
	if config.FileDescriptors == nil {
		config.FileDescriptors = stdlibfs.NewFileDescriptors()
	}
	primitives := makePrimitiveModule(config)
	path := makePathModule(primitives)
	flags := config.Platform.OpenFlags
	members := make(starlark.StringDict, len(primitives.Members)+28)
	for name, value := range primitives.Members {
		if len(name) >= 5 && name[:5] == "path_" {
			continue
		}
		members[name] = value
	}
	members["path"] = path
	members["name"] = starlark.String(config.Platform.OSName)
	members["curdir"] = starlark.String(".")
	members["pardir"] = starlark.String("..")
	members["sep"] = starlark.String("/")
	members["altsep"] = starlark.None
	members["extsep"] = starlark.String(".")
	members["pathsep"] = starlark.String(config.Platform.PathListSeparator)
	members["linesep"] = starlark.String("\n")
	members["defpath"] = starlark.String("/bin:/usr/bin")
	members["devnull"] = starlark.String(config.Platform.DevNull)
	members["F_OK"] = starlark.MakeInt(0)
	members["R_OK"] = starlark.MakeInt(4)
	members["W_OK"] = starlark.MakeInt(2)
	members["X_OK"] = starlark.MakeInt(1)
	members["O_RDONLY"] = starlark.MakeInt(flags.ReadOnly)
	members["O_WRONLY"] = starlark.MakeInt(flags.WriteOnly)
	members["O_RDWR"] = starlark.MakeInt(flags.ReadWrite)
	members["O_APPEND"] = starlark.MakeInt(flags.Append)
	members["O_CREAT"] = starlark.MakeInt(flags.Create)
	members["O_EXCL"] = starlark.MakeInt(flags.Exclusive)
	members["O_SYNC"] = starlark.MakeInt(flags.Sync)
	members["O_TRUNC"] = starlark.MakeInt(flags.Truncate)
	members["SEEK_SET"] = starlark.MakeInt(0)
	members["SEEK_CUR"] = starlark.MakeInt(1)
	members["SEEK_END"] = starlark.MakeInt(2)
	module := &starlarkstruct.Module{Name: ModuleName, Members: members}
	module.Freeze()
	return module
}

func makePrimitiveModule(config ModuleConfig) *starlarkstruct.Module {
	filesystem := FileSystem{fsys: config.FS, platform: config.Platform, fds: config.FileDescriptors, clock: config.Clock}
	environment := Environment{env: config.Env, platform: config.Platform}
	process := Process{process: config.Process, commandRunner: config.CommandRunner}
	workingDir := WorkingDirectory{dir: config.WorkingDir}
	return &starlarkstruct.Module{
		Name: ModuleName + "._primitive",
		Members: starlark.StringDict{
			"getcwd":        starlark.NewBuiltin(ModuleName+".getcwd", workingDir.getcwd),
			"chdir":         starlark.NewBuiltin(ModuleName+".chdir", workingDir.chdir),
			"get_exec_path": starlark.NewBuiltin(ModuleName+".get_exec_path", environment.getExecPath),
			"getpid":        starlark.NewBuiltin(ModuleName+".getpid", process.getpid),
			"getppid":       starlark.NewBuiltin(ModuleName+".getppid", process.getppid),
			"kill":          starlark.NewBuiltin(ModuleName+".kill", process.kill),
			"system":        starlark.NewBuiltin(ModuleName+".system", process.system),

			"environ":  newEnvironValue(environment),
			"getenv":   starlark.NewBuiltin(ModuleName+".getenv", environment.getenv),
			"putenv":   starlark.NewBuiltin(ModuleName+".putenv", environment.putenv),
			"unsetenv": starlark.NewBuiltin(ModuleName+".unsetenv", environment.unsetenv),

			"listdir": starlark.NewBuiltin(ModuleName+".listdir", filesystem.listdir),
			"scandir": starlark.NewBuiltin(ModuleName+".scandir", filesystem.scandir),
			"walk":    starlark.NewBuiltin(ModuleName+".walk", filesystem.walk),
			"stat":    starlark.NewBuiltin(ModuleName+".stat", filesystem.stat),
			"lstat":   starlark.NewBuiltin(ModuleName+".lstat", filesystem.lstat),
			"access":  starlark.NewBuiltin(ModuleName+".access", filesystem.access),

			"mkdir":      starlark.NewBuiltin(ModuleName+".mkdir", filesystem.mkdir),
			"makedirs":   starlark.NewBuiltin(ModuleName+".makedirs", filesystem.makedirs),
			"rmdir":      starlark.NewBuiltin(ModuleName+".rmdir", filesystem.rmdir),
			"removedirs": starlark.NewBuiltin(ModuleName+".removedirs", filesystem.removedirs),
			"remove":     starlark.NewBuiltin(ModuleName+".remove", filesystem.remove),
			"unlink":     starlark.NewBuiltin(ModuleName+".unlink", filesystem.remove),
			"rename":     starlark.NewBuiltin(ModuleName+".rename", filesystem.rename),
			"replace":    starlark.NewBuiltin(ModuleName+".replace", filesystem.rename),
			"renames":    starlark.NewBuiltin(ModuleName+".renames", filesystem.renames),
			"chmod":      starlark.NewBuiltin(ModuleName+".chmod", filesystem.chmod),
			"chown":      starlark.NewBuiltin(ModuleName+".chown", filesystem.chown),
			"utime":      starlark.NewBuiltin(ModuleName+".utime", filesystem.utime),
			"truncate":   starlark.NewBuiltin(ModuleName+".truncate", filesystem.truncate),
			"link":       starlark.NewBuiltin(ModuleName+".link", filesystem.link),
			"symlink":    starlark.NewBuiltin(ModuleName+".symlink", filesystem.symlink),
			"readlink":   starlark.NewBuiltin(ModuleName+".readlink", filesystem.readlink),

			"open":      starlark.NewBuiltin(ModuleName+".open", filesystem.open),
			"close":     starlark.NewBuiltin(ModuleName+".close", filesystem.close),
			"read":      starlark.NewBuiltin(ModuleName+".read", filesystem.read),
			"write":     starlark.NewBuiltin(ModuleName+".write", filesystem.write),
			"fsync":     starlark.NewBuiltin(ModuleName+".fsync", filesystem.fsync),
			"ftruncate": starlark.NewBuiltin(ModuleName+".ftruncate", filesystem.ftruncate),

			"getuid":    starlark.NewBuiltin(ModuleName+".getuid", process.getuid),
			"geteuid":   starlark.NewBuiltin(ModuleName+".geteuid", process.geteuid),
			"getgid":    starlark.NewBuiltin(ModuleName+".getgid", process.getgid),
			"getegid":   starlark.NewBuiltin(ModuleName+".getegid", process.getegid),
			"getgroups": starlark.NewBuiltin(ModuleName+".getgroups", process.getgroups),
			"umask":     starlark.NewBuiltin(ModuleName+".umask", process.umask),

			"path_abspath":    starlark.NewBuiltin(ModuleName+".path.abspath", filesystem.pathAbspath),
			"path_exists":     starlark.NewBuiltin(ModuleName+".path.exists", filesystem.pathExists),
			"path_lexists":    starlark.NewBuiltin(ModuleName+".path.lexists", filesystem.pathLexists),
			"path_expanduser": starlark.NewBuiltin(ModuleName+".path.expanduser", environment.pathExpanduser),
			"path_expandvars": starlark.NewBuiltin(ModuleName+".path.expandvars", environment.pathExpandvars),
			"path_getatime":   starlark.NewBuiltin(ModuleName+".path.getatime", filesystem.pathGetatime),
			"path_getmtime":   starlark.NewBuiltin(ModuleName+".path.getmtime", filesystem.pathGetmtime),
			"path_getctime":   starlark.NewBuiltin(ModuleName+".path.getctime", filesystem.pathGetctime),
			"path_getsize":    starlark.NewBuiltin(ModuleName+".path.getsize", filesystem.pathGetsize),
			"path_isdir":      starlark.NewBuiltin(ModuleName+".path.isdir", filesystem.pathIsDir),
			"path_isfile":     starlark.NewBuiltin(ModuleName+".path.isfile", filesystem.pathIsFile),
			"path_islink":     starlark.NewBuiltin(ModuleName+".path.islink", filesystem.pathIslink),
			"path_ismount":    starlark.NewBuiltin(ModuleName+".path.ismount", filesystem.pathIsmount),
			"path_realpath":   starlark.NewBuiltin(ModuleName+".path.realpath", filesystem.pathRealpath),
			"path_samefile":   starlark.NewBuiltin(ModuleName+".path.samefile", filesystem.pathSamefile),
		},
	}
}
