package dyson

import (
	stdlibre "github.com/spachava753/dyson/internal/stdlib/re"
	stdlibtime "github.com/spachava753/dyson/internal/stdlib/time"
	"go.starlark.net/starlark"
)

// StdlibModules contains Dyson's loadable standard-library compatibility
// modules, keyed by the Starlark load path.
var StdlibModules = map[string]starlark.StringDict{
	stdlibre.ModuleName + ".star": starlark.StringDict{
		stdlibre.ModuleName: stdlibre.Module,
	},
	stdlibtime.ModuleName + ".star": starlark.StringDict{
		stdlibtime.ModuleName: stdlibtime.Module,
	},
}
