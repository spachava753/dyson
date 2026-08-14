package re

import (
	"github.com/spachava753/starlarkx/starlark"
	"github.com/spachava753/starlarkx/starlarkstruct"
)

// ModuleName is the Python-compatible name of the regular-expression module.
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
	patternMethods = map[string]*starlark.Builtin{
		"search":    starlark.NewBuiltin("search", patternSearch),
		"match":     starlark.NewBuiltin("match", patternMatch),
		"fullmatch": starlark.NewBuiltin("fullmatch", patternFullmatch),
		"split":     starlark.NewBuiltin("split", patternSplit),
		"findall":   starlark.NewBuiltin("findall", patternFindall),
		"finditer":  starlark.NewBuiltin("finditer", patternFinditer),
		"sub":       starlark.NewBuiltin("sub", patternSub),
		"subn":      starlark.NewBuiltin("subn", patternSubn),
	}
	matchMethods = map[string]*starlark.Builtin{
		"expand":    starlark.NewBuiltin("expand", matchExpand),
		"group":     starlark.NewBuiltin("group", matchGroup),
		"groups":    starlark.NewBuiltin("groups", matchGroups),
		"groupdict": starlark.NewBuiltin("groupdict", matchGroupdict),
		"start":     starlark.NewBuiltin("start", matchStart),
		"end":       starlark.NewBuiltin("end", matchEnd),
		"span":      starlark.NewBuiltin("span", matchSpanMethod),
	}
)

// Module is the Starlark module namespace exposed by load("re.star", "re").
var Module = &starlarkstruct.Module{
	Name: ModuleName,
	Members: starlark.StringDict{
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
	},
}

func init() {
	Module.Freeze()
}

func mustSet(dict *starlark.Dict, key string, value starlark.Value) {
	if err := dict.SetKey(starlark.String(key), value); err != nil {
		panic(err)
	}
}
