package shutil

import (
	stdlibos "github.com/spachava753/dyson/internal/stdlib/os"
	"github.com/spachava753/dyson/internal/stdlibfs"
	"github.com/spachava753/dyson/internal/xos"
	"github.com/spf13/afero"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ModuleName is the Starlark stdlib module name for Dyson's shutil compatibility module.
const ModuleName = "shutil"

// Module is the default Starlark module namespace exposed by load("shutil.star", "shutil").
var Module = MakeModule(ModuleConfig{OS: stdlibos.Module})

// ModuleConfig groups the host domains used by Dyson's shutil module.
type ModuleConfig struct {
	OS       *starlarkstruct.Module
	FS       afero.Fs
	Env      xos.Env
	Terminal xos.Terminal
	Platform xos.Platform
}

// HostConfig returns a shutil configuration backed by the host filesystem and OS.
func HostConfig(root string, osModule *starlarkstruct.Module) ModuleConfig {
	host := xos.Host{}
	return ModuleConfig{OS: osModule, FS: stdlibfs.NewHost(afero.NewOsFs(), root), Env: host, Terminal: host, Platform: host.Platform()}
}

// MakeModule returns a Starlark shutil module backed by config.
func MakeModule(config ModuleConfig) *starlarkstruct.Module {
	if config.OS == nil {
		config.OS = stdlibos.Module
	}
	if config.Platform.OSName == "" {
		config.Platform = xos.Host{}.Platform()
	}
	implementation := moduleFunctions{
		os: config.OS,
		primitives: primitives{
			fsys:     config.FS,
			env:      config.Env,
			terminal: config.Terminal,
			platform: config.Platform,
		},
	}
	module := &starlarkstruct.Module{
		Name: ModuleName,
		Members: starlark.StringDict{
			"copyfile":          starlark.NewBuiltin(ModuleName+".copyfile", implementation.copyfile),
			"copymode":          starlark.NewBuiltin(ModuleName+".copymode", implementation.copymode),
			"copystat":          starlark.NewBuiltin(ModuleName+".copystat", implementation.copystat),
			"copy":              starlark.NewBuiltin(ModuleName+".copy", implementation.copy),
			"copy2":             starlark.NewBuiltin(ModuleName+".copy2", implementation.copy2),
			"copytree":          starlark.NewBuiltin(ModuleName+".copytree", implementation.copytree),
			"rmtree":            starlark.NewBuiltin(ModuleName+".rmtree", implementation.rmtree),
			"move":              starlark.NewBuiltin(ModuleName+".move", implementation.move),
			"disk_usage":        starlark.NewBuiltin(ModuleName+".disk_usage", implementation.diskUsage),
			"chown":             starlark.NewBuiltin(ModuleName+".chown", implementation.chown),
			"get_terminal_size": starlark.NewBuiltin(ModuleName+".get_terminal_size", implementation.getTerminalSize),
			"which":             starlark.NewBuiltin(ModuleName+".which", implementation.which),
			"ignore_patterns":   starlark.NewBuiltin(ModuleName+".ignore_patterns", implementation.ignorePatterns),
		},
	}
	module.Freeze()
	return module
}
