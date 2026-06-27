package builtins

import "go.starlark.net/starlark"

// builtinsFunc contains Dyson's Python-like builtin functions that should be
// injected into a Starlark globals namespace for generated programs.
var builtinsFunc = starlark.StringDict{
	"abs":       starlark.NewBuiltin("abs", absBuiltin),
	"range":     starlark.NewBuiltin("range", rangeBuiltin),
	"reversed":  starlark.NewBuiltin("reversed", reversedBuiltin),
	"round":     starlark.NewBuiltin("round", roundBuiltin),
	"sorted":    starlark.NewBuiltin("sorted", sortedBuiltin),
	"sum":       starlark.NewBuiltin("sum", sumBuiltin),
	"min":       starlark.NewBuiltin("min", minBuiltin),
	"max":       starlark.NewBuiltin("max", maxBuiltin),
	"oct":       starlark.NewBuiltin("oct", octBuiltin),
	"bin":       starlark.NewBuiltin("bin", binBuiltin),
	"ord":       starlark.NewBuiltin("ord", ordBuiltin),
	"pow":       starlark.NewBuiltin("pow", powBuiltin),
	"enumerate": starlark.NewBuiltin("enumerate", enumerateBuiltin),
	"open":      starlark.NewBuiltin("open", openBuiltin),
	"chr":       starlark.NewBuiltin("chr", chrBuiltin),
}
