package tempfile

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ModuleName is the Starlark stdlib module name for Dyson's tempfile compatibility module.
const ModuleName = "tempfile"

// Module is the Starlark module namespace exposed by load("tempfile.star", "tempfile").
var Module = &starlarkstruct.Module{
	Name: ModuleName,
	Members: starlark.StringDict{
		// Python docs: https://docs.python.org/3/library/tempfile.html#tempfile.mkdtemp
		"mkdtemp": nil,
		// Python docs: https://docs.python.org/3/library/tempfile.html#tempfile.NamedTemporaryFile
		"NamedTemporaryFile": nil,
		// Python docs: https://docs.python.org/3/library/tempfile.html#tempfile.TemporaryDirectory
		"TemporaryDirectory": nil,
	},
}
