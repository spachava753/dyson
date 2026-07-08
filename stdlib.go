package dyson

import (
	gotime "time"

	stdlibglob "github.com/spachava753/dyson/internal/stdlib/glob"
	stdlibgrp "github.com/spachava753/dyson/internal/stdlib/grp"
	stdlibos "github.com/spachava753/dyson/internal/stdlib/os"
	stdlibpwd "github.com/spachava753/dyson/internal/stdlib/pwd"
	stdlibre "github.com/spachava753/dyson/internal/stdlib/re"
	stdlibshutil "github.com/spachava753/dyson/internal/stdlib/shutil"
	stdlibsignal "github.com/spachava753/dyson/internal/stdlib/signal"
	stdlibsubprocess "github.com/spachava753/dyson/internal/stdlib/subprocess"
	stdlibtempfile "github.com/spachava753/dyson/internal/stdlib/tempfile"
	stdlibtime "github.com/spachava753/dyson/internal/stdlib/time"
	"github.com/spachava753/dyson/internal/xfs"
	"github.com/spachava753/dyson/internal/xos"
	"go.starlark.net/starlark"
)

// StdlibModules returns Dyson's loadable standard-library compatibility
// modules, keyed by the Starlark load path. Each call returns fresh filesystem
// backed os/glob modules and a fresh time module so per-session policy and
// monotonic clocks do not share state.
func StdlibModules() map[string]starlark.StringDict {
	fileDescriptors := xfs.NewFileDescriptors()
	osConfig := stdlibos.HostConfig(".")
	osConfig.FileDescriptors = fileDescriptors
	osModule := stdlibos.MakeModule(osConfig)
	globModule := stdlibglob.MakeModule(osModule)

	return map[string]starlark.StringDict{
		stdlibglob.ModuleName + ".star": {
			stdlibglob.ModuleName: globModule,
		},
		stdlibgrp.ModuleName + ".star": {
			stdlibgrp.ModuleName: stdlibgrp.Module,
		},
		stdlibos.ModuleName + ".star": {
			stdlibos.ModuleName: osModule,
		},
		stdlibpwd.ModuleName + ".star": {
			stdlibpwd.ModuleName: stdlibpwd.Module,
		},
		stdlibre.ModuleName + ".star": {
			stdlibre.ModuleName: stdlibre.Module,
		},
		stdlibshutil.ModuleName + ".star": {
			stdlibshutil.ModuleName: stdlibshutil.Module,
		},
		stdlibsignal.ModuleName + ".star": {
			stdlibsignal.ModuleName: stdlibsignal.Module,
		},
		stdlibsubprocess.ModuleName + ".star": {
			stdlibsubprocess.ModuleName: stdlibsubprocess.Module,
		},
		stdlibtempfile.ModuleName + ".star": {
			stdlibtempfile.ModuleName: stdlibtempfile.MakeModule(stdlibtempfile.ModuleConfig{FS: xfs.HostFS{Root: "."}, Env: xos.Host{}, FileDescriptors: fileDescriptors}),
		},
		stdlibtime.ModuleName + ".star": {
			stdlibtime.ModuleName: stdlibtime.MakeModule(gotime.Now()),
		},
	}
}
