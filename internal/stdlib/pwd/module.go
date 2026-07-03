package pwd

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ModuleName is the Starlark stdlib module name for Dyson's pwd compatibility module.
const ModuleName = "pwd"

// Module is the Starlark module namespace exposed by load("pwd.star", "pwd").
var Module = &starlarkstruct.Module{
	Name: ModuleName,
	Members: starlark.StringDict{
		// Python docs: https://docs.python.org/3/library/pwd.html#pwd.getpwnam
		"getpwnam": nil,
		// Python docs: https://docs.python.org/3/library/pwd.html#pwd.getpwuid
		"getpwuid": nil,
		// Python docs: https://docs.python.org/3/library/pwd.html#pwd.getpwall
		"getpwall": nil,
		// Python docs: https://docs.python.org/3/library/pwd.html#pwd.struct_passwd
		"struct_passwd": nil,
	},
}
