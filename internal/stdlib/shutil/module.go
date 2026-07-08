package shutil

import (
	_ "embed"
	"fmt"

	stdlibos "github.com/spachava753/dyson/internal/stdlib/os"
	"github.com/spachava753/dyson/internal/xfs"
	"github.com/spachava753/dyson/internal/xos"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"
)

// ModuleName is the Starlark stdlib module name for Dyson's shutil compatibility module.
const ModuleName = "shutil"

//go:embed shutil.star
var source string

// Module is the default Starlark module namespace exposed by load("shutil.star", "shutil").
var Module = MakeModule(ModuleConfig{OS: stdlibos.Module})

// ModuleConfig groups the host domains used by Dyson's shutil module.
type ModuleConfig struct {
	OS       *starlarkstruct.Module
	FS       xfs.FS
	Env      xos.Env
	Terminal xos.Terminal
	Platform xos.Platform
}

// HostConfig returns a shutil configuration backed by the host filesystem and OS.
func HostConfig(root string, osModule *starlarkstruct.Module) ModuleConfig {
	host := xos.Host{}
	return ModuleConfig{OS: osModule, FS: xfs.HostFS{Root: root}, Env: host, Terminal: host, Platform: host.Platform()}
}

// MakeModule returns a Starlark shutil module backed by config.
func MakeModule(config ModuleConfig) *starlarkstruct.Module {
	if config.OS == nil {
		config.OS = stdlibos.Module
	}
	if config.Platform.OSName == "" {
		config.Platform = xos.PortablePlatform
	}
	return loadModule(config.OS, makePrimitiveModule(config), config.Platform)
}

func loadModule(osModule, primitives *starlarkstruct.Module, platform xos.Platform) *starlarkstruct.Module {
	globals, err := starlark.ExecFileOptions(
		&syntax.FileOptions{While: true, Recursion: true},
		&starlark.Thread{Name: ModuleName + ".star"},
		ModuleName+".star",
		source,
		starlark.StringDict{
			"module":       starlark.NewBuiltin("module", starlarkstruct.MakeModule),
			"os":           osModule,
			"_shutil":      primitives,
			"pathsep":      starlark.String(platform.PathListSeparator),
			"default_path": starlark.String("/bin:/usr/bin"),
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
