package dyson

import (
	gotime "time"

	stdlibre "github.com/spachava753/dyson/internal/stdlib/re"
	stdlibtime "github.com/spachava753/dyson/internal/stdlib/time"
	"go.starlark.net/starlark"
)

// StdlibModules returns Dyson's loadable standard-library compatibility
// modules, keyed by the Starlark load path. Each call returns a fresh map and a
// fresh time module so session-local monotonic clocks do not share an origin.
func StdlibModules() map[string]starlark.StringDict {
	return map[string]starlark.StringDict{
		stdlibre.ModuleName + ".star": {
			stdlibre.ModuleName: stdlibre.Module,
		},
		stdlibtime.ModuleName + ".star": {
			stdlibtime.ModuleName: stdlibtime.MakeModule(gotime.Now()),
		},
	}
}
