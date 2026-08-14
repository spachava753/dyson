package glob

import (
	stdlibos "github.com/spachava753/dyson/internal/stdlib/os"
	"github.com/spachava753/starlarkx/starlark"
	"github.com/spachava753/starlarkx/starlarkstruct"
)

// ModuleName is the Starlark stdlib module name for Dyson's glob compatibility module.
const ModuleName = "glob"

// Module is the default Starlark module namespace exposed by load("glob.star", "glob").
var Module = MakeModule(stdlibos.Module)

// MakeModule returns a Starlark glob module using osModule for filesystem access.
func MakeModule(osModule *starlarkstruct.Module) *starlarkstruct.Module {
	implementation := moduleImplementation{os: osModule}
	module := &starlarkstruct.Module{
		Name: ModuleName,
		Members: starlark.StringDict{
			"glob":   starlark.NewBuiltin(ModuleName+".glob", implementation.glob),
			"iglob":  starlark.NewBuiltin(ModuleName+".iglob", implementation.glob),
			"escape": starlark.NewBuiltin(ModuleName+".escape", escape),
		},
	}
	module.Freeze()
	return module
}
