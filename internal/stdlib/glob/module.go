package glob

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ModuleName is the Starlark stdlib module name for Dyson's glob compatibility module.
const ModuleName = "glob"

// Module is the Starlark module namespace exposed by load("glob.star", "glob").
var Module = &starlarkstruct.Module{
	Name: ModuleName,
	Members: starlark.StringDict{
		// Python docs: https://docs.python.org/3/library/glob.html#glob.glob
		"glob": nil,
		// Python docs: https://docs.python.org/3/library/glob.html#glob.iglob
		"iglob": nil,
		// Python docs: https://docs.python.org/3/library/glob.html#glob.escape
		"escape": nil,
	},
}
