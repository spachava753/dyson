package shutil

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ModuleName is the Starlark stdlib module name for Dyson's shutil compatibility module.
const ModuleName = "shutil"

// Module is the Starlark module namespace exposed by load("shutil.star", "shutil").
var Module = &starlarkstruct.Module{
	Name: ModuleName,
	Members: starlark.StringDict{
		// Python docs: https://docs.python.org/3/library/shutil.html#shutil.which
		"which": nil,
		// Python docs: https://docs.python.org/3/library/shutil.html#shutil.copyfile
		"copyfile": nil,
		// Python docs: https://docs.python.org/3/library/shutil.html#shutil.copy
		"copy": nil,
		// Python docs: https://docs.python.org/3/library/shutil.html#shutil.copy2
		"copy2": nil,
		// Python docs: https://docs.python.org/3/library/shutil.html#shutil.copytree
		"copytree": nil,
		// Python docs: https://docs.python.org/3/library/shutil.html#shutil.rmtree
		"rmtree": nil,
		// Python docs: https://docs.python.org/3/library/shutil.html#shutil.move
		"move": nil,
		// Python docs: https://docs.python.org/3/library/shutil.html#shutil.disk_usage
		"disk_usage": nil,
		// Python docs: https://docs.python.org/3/library/shutil.html#shutil.chown
		"chown": nil,
		// Python docs: https://docs.python.org/3/library/shutil.html#shutil.get_terminal_size
		"get_terminal_size": nil,
	},
}
