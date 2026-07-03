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
	"go.starlark.net/starlark"
)

// StdlibModules returns Dyson's loadable standard-library compatibility
// modules, keyed by the Starlark load path. Each call returns a fresh map and a
// fresh time module so session-local monotonic clocks do not share an origin.
func StdlibModules() map[string]starlark.StringDict {
	return map[string]starlark.StringDict{
		stdlibglob.ModuleName + ".star": {
			stdlibglob.ModuleName: stdlibglob.Module,
		},
		stdlibgrp.ModuleName + ".star": {
			stdlibgrp.ModuleName: stdlibgrp.Module,
		},
		stdlibos.ModuleName + ".star": {
			stdlibos.ModuleName: stdlibos.Module,
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
			stdlibtempfile.ModuleName: stdlibtempfile.Module,
		},
		stdlibtime.ModuleName + ".star": {
			stdlibtime.ModuleName: stdlibtime.MakeModule(gotime.Now()),
		},
	}
}
