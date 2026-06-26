package re

import (
	"sync"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

const ModuleName = "re"

const (
	flagNoFlag = 0
)

const (
	_ = 1 << iota // Python reserves value 1 for TEMPLATE/T.
	flagIgnoreCase
	flagLocale
	flagMultiline
	flagDotAll
	flagUnicode
	flagVerbose
	flagDebug
	flagASCII
)

var (
	patternAttrNames = []string{
		"findall",
		"finditer",
		"flags",
		"fullmatch",
		"groupindex",
		"groups",
		"match",
		"pattern",
		"search",
		"split",
		"sub",
		"subn",
	}
	matchAttrNames = []string{
		"end",
		"endpos",
		"expand",
		"group",
		"groupdict",
		"groups",
		"lastgroup",
		"lastindex",
		"pos",
		"re",
		"span",
		"start",
		"string",
	}
)

var module = sync.OnceValue(func() starlark.StringDict {
	members := starlark.StringDict{
		"compile":   starlark.NewBuiltin(ModuleName+".compile", compile),
		"search":    starlark.NewBuiltin(ModuleName+".search", search),
		"match":     starlark.NewBuiltin(ModuleName+".match", match),
		"fullmatch": starlark.NewBuiltin(ModuleName+".fullmatch", fullMatch),
		"split":     starlark.NewBuiltin(ModuleName+".split", split),
		"findall":   starlark.NewBuiltin(ModuleName+".findall", findAll),
		"finditer":  starlark.NewBuiltin(ModuleName+".finditer", findIter),
		"sub":       starlark.NewBuiltin(ModuleName+".sub", sub),
		"subn":      starlark.NewBuiltin(ModuleName+".subn", subn),
		"escape":    starlark.NewBuiltin(ModuleName+".escape", escape),
		"purge":     starlark.NewBuiltin(ModuleName+".purge", purge),

		"NOFLAG":     starlark.MakeInt(flagNoFlag),
		"ASCII":      starlark.MakeInt(flagASCII),
		"A":          starlark.MakeInt(flagASCII),
		"IGNORECASE": starlark.MakeInt(flagIgnoreCase),
		"I":          starlark.MakeInt(flagIgnoreCase),
		"LOCALE":     starlark.MakeInt(flagLocale),
		"L":          starlark.MakeInt(flagLocale),
		"UNICODE":    starlark.MakeInt(flagUnicode),
		"U":          starlark.MakeInt(flagUnicode),
		"MULTILINE":  starlark.MakeInt(flagMultiline),
		"M":          starlark.MakeInt(flagMultiline),
		"DOTALL":     starlark.MakeInt(flagDotAll),
		"S":          starlark.MakeInt(flagDotAll),
		"VERBOSE":    starlark.MakeInt(flagVerbose),
		"X":          starlark.MakeInt(flagVerbose),
		"DEBUG":      starlark.MakeInt(flagDebug),
	}

	module := &starlarkstruct.Module{Name: ModuleName, Members: members}
	module.Freeze()
	return starlark.StringDict{ModuleName: module}
})

// LoadModule returns Dyson's Python-compatible re module.
func LoadModule() (starlark.StringDict, error) {
	return module(), nil
}

func mustSet(dict *starlark.Dict, key string, value starlark.Value) {
	if err := dict.SetKey(starlark.String(key), value); err != nil {
		panic(err)
	}
}
