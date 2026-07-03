package grp

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ModuleName is the Starlark stdlib module name for Dyson's grp compatibility module.
const ModuleName = "grp"

// Module is the Starlark module namespace exposed by load("grp.star", "grp").
var Module = &starlarkstruct.Module{
	Name: ModuleName,
	Members: starlark.StringDict{
		// Python docs: https://docs.python.org/3/library/grp.html#grp.getgrnam
		"getgrnam": nil,
		// Python docs: https://docs.python.org/3/library/grp.html#grp.getgrgid
		"getgrgid": nil,
		// Python docs: https://docs.python.org/3/library/grp.html#grp.getgrall
		"getgrall": nil,
		// Python docs: https://docs.python.org/3/library/grp.html#grp.struct_group
		"struct_group": nil,
	},
}
