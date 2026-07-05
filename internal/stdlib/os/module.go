package os

import (
	_ "embed"
	"fmt"

	"github.com/spachava753/dyson/internal/xfs"
	"github.com/spachava753/dyson/internal/xos"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"
)

// ModuleName is the Starlark stdlib module name for Dyson's os compatibility module.
const ModuleName = "os"

//go:embed os.star
var source string

// Module is the default Starlark module namespace exposed by load("os.star", "os").
var Module = MakeModule(HostConfig("."))

// ModuleConfig groups the host domains used by Dyson's os compatibility module.
type ModuleConfig struct {
	FS         xfs.FS
	Env        xos.Env
	Process    xos.Process
	WorkingDir xos.WorkingDir
	Platform   xos.Platform
}

// HostConfig returns a module configuration backed by the host filesystem and OS.
func HostConfig(root string) ModuleConfig {
	host := xos.Host{}
	return ModuleConfig{
		FS:         xfs.HostFS{Root: root},
		Env:        host,
		Process:    host,
		WorkingDir: host,
		Platform:   host.Platform(),
	}
}

// MakeModule returns a Starlark os module backed by config. File, environment,
// process, working-directory, and platform behavior are controlled by separate
// domain interfaces; missing domains fail at call time.
func MakeModule(config ModuleConfig) *starlarkstruct.Module {
	if config.Platform.OSName == "" {
		config.Platform = xos.PortablePlatform
	}
	return loadModule(makePrimitiveModule(config), config.Platform)
}

func loadModule(primitives *starlarkstruct.Module, platform xos.Platform) *starlarkstruct.Module {
	flags := platform.OpenFlags
	globals, err := starlark.ExecFileOptions(
		&syntax.FileOptions{While: true, Recursion: true},
		&starlark.Thread{Name: ModuleName + ".star"},
		ModuleName+".star",
		source,
		starlark.StringDict{
			"module": starlark.NewBuiltin("module", starlarkstruct.MakeModule),
			"_os":    primitives,

			"os_name": starlark.String(platform.OSName),
			"pathsep": starlark.String(platform.PathListSeparator),
			"devnull": starlark.String(platform.DevNull),

			"o_rdonly": starlark.MakeInt(flags.ReadOnly),
			"o_wronly": starlark.MakeInt(flags.WriteOnly),
			"o_rdwr":   starlark.MakeInt(flags.ReadWrite),
			"o_append": starlark.MakeInt(flags.Append),
			"o_creat":  starlark.MakeInt(flags.Create),
			"o_excl":   starlark.MakeInt(flags.Exclusive),
			"o_sync":   starlark.MakeInt(flags.Sync),
			"o_trunc":  starlark.MakeInt(flags.Truncate),
		},
	)
	if err != nil {
		panic(fmt.Sprintf("load %s.star: %v", ModuleName, err))
	}

	module, ok := globals[ModuleName].(*starlarkstruct.Module)
	if !ok {
		panic(fmt.Sprintf("load %s.star: global %q is %T", ModuleName, ModuleName, globals[ModuleName]))
	}
	return module
}

func makePrimitiveModule(config ModuleConfig) *starlarkstruct.Module {
	filesystem := FileSystem{fsys: config.FS, platform: config.Platform}
	environment := Environment{env: config.Env, platform: config.Platform}
	process := Process{process: config.Process}
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

			"environ":  starlark.NewBuiltin(ModuleName+".environ", environment.environ),
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
