package subprocess

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ModuleName is the Starlark stdlib module name for Dyson's subprocess compatibility module.
const ModuleName = "subprocess"

// Module is the Starlark module namespace exposed by load("subprocess.star", "subprocess").
var Module = &starlarkstruct.Module{
	Name: ModuleName,
	Members: starlark.StringDict{
		// Python docs: https://docs.python.org/3/library/subprocess.html#subprocess.run
		"run": nil,
		// Python docs: https://docs.python.org/3/library/subprocess.html#subprocess.getoutput
		"getoutput": nil,
		// Python docs: https://docs.python.org/3/library/subprocess.html#subprocess.getstatusoutput
		"getstatusoutput": nil,
		// Python docs: https://docs.python.org/3/library/subprocess.html#subprocess.CompletedProcess
		"CompletedProcess": nil,
		// Python docs: https://docs.python.org/3/library/subprocess.html#subprocess.PIPE
		"PIPE": starlark.MakeInt(-1),
		// Python docs: https://docs.python.org/3/library/subprocess.html#subprocess.STDOUT
		"STDOUT": starlark.MakeInt(-2),
		// Python docs: https://docs.python.org/3/library/subprocess.html#subprocess.DEVNULL
		"DEVNULL": starlark.MakeInt(-3),
	},
}
