package re

import (
	"fmt"

	"go.starlark.net/starlark"
)

func compile(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var expr starlark.Value
	flags := starlark.MakeInt(0)
	if err := starlark.UnpackArgs("re.compile", args, kwargs, "pattern", &expr, "flags?", &flags); err != nil {
		return nil, err
	}

	if compiled, ok := asPattern(expr); ok {
		if flags.Sign() != 0 {
			return nil, fmt.Errorf("re.compile: cannot process flags argument with a compiled pattern")
		}
		return compiled, nil
	}
	if err := requireRegexText("re.compile", "pattern", expr); err != nil {
		return nil, err
	}
	return newPattern(expr, flags), nil
}

func search(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := unpackPatternStringFlags("re.search", args, kwargs); err != nil {
		return nil, err
	}
	return nil, notImplemented("re.search")
}

func match(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := unpackPatternStringFlags("re.match", args, kwargs); err != nil {
		return nil, err
	}
	return nil, notImplemented("re.match")
}

func fullMatch(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := unpackPatternStringFlags("re.fullmatch", args, kwargs); err != nil {
		return nil, err
	}
	return nil, notImplemented("re.fullmatch")
}

func split(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var expr, text starlark.Value
	maxsplit := starlark.MakeInt(0)
	flags := starlark.MakeInt(0)
	if err := starlark.UnpackArgs("re.split", args, kwargs, "pattern", &expr, "string", &text, "maxsplit?", &maxsplit, "flags?", &flags); err != nil {
		return nil, err
	}
	if err := requirePattern("re.split", expr); err != nil {
		return nil, err
	}
	if err := requireRegexText("re.split", "string", text); err != nil {
		return nil, err
	}
	return nil, notImplemented("re.split")
}

func findAll(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := unpackPatternStringFlags("re.findall", args, kwargs); err != nil {
		return nil, err
	}
	return nil, notImplemented("re.findall")
}

func findIter(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := unpackPatternStringFlags("re.finditer", args, kwargs); err != nil {
		return nil, err
	}
	return nil, notImplemented("re.finditer")
}

func sub(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := unpackSubArgs("re.sub", args, kwargs); err != nil {
		return nil, err
	}
	return nil, notImplemented("re.sub")
}

func subn(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := unpackSubArgs("re.subn", args, kwargs); err != nil {
		return nil, err
	}
	return nil, notImplemented("re.subn")
}

func escape(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var expr starlark.Value
	if err := starlark.UnpackArgs("re.escape", args, kwargs, "pattern", &expr); err != nil {
		return nil, err
	}
	if err := requireRegexText("re.escape", "pattern", expr); err != nil {
		return nil, err
	}
	return nil, notImplemented("re.escape")
}

func purge(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("re.purge", args, kwargs); err != nil {
		return nil, err
	}
	return starlark.None, nil
}

func unpackPatternStringFlags(fn string, args starlark.Tuple, kwargs []starlark.Tuple) error {
	var expr, text starlark.Value
	flags := starlark.MakeInt(0)
	if err := starlark.UnpackArgs(fn, args, kwargs, "pattern", &expr, "string", &text, "flags?", &flags); err != nil {
		return err
	}
	if err := requirePattern(fn, expr); err != nil {
		return err
	}
	return requireRegexText(fn, "string", text)
}

func unpackSubArgs(fn string, args starlark.Tuple, kwargs []starlark.Tuple) error {
	var expr, repl, text starlark.Value
	count := starlark.MakeInt(0)
	flags := starlark.MakeInt(0)
	if err := starlark.UnpackArgs(fn, args, kwargs, "pattern", &expr, "repl", &repl, "string", &text, "count?", &count, "flags?", &flags); err != nil {
		return err
	}
	if err := requirePattern(fn, expr); err != nil {
		return err
	}
	if err := requireReplacement(fn, repl); err != nil {
		return err
	}
	return requireRegexText(fn, "string", text)
}

func requirePattern(fn string, value starlark.Value) error {
	if _, ok := asPattern(value); ok {
		return nil
	}
	return requireRegexText(fn, "pattern", value)
}

func newPattern(expr starlark.Value, flags starlark.Int) *starlark.Dict {
	pattern := starlark.NewDict(6)
	mustSet(pattern, "kind", starlark.String("re.Pattern"))
	mustSet(pattern, "pattern", expr)
	mustSet(pattern, "flags", flags)
	mustSet(pattern, "groups", starlark.MakeInt(0))
	mustSet(pattern, "groupindex", starlark.NewDict(0))
	mustSet(pattern, "attrs", stringList(patternAttrNames))
	pattern.Freeze()
	return pattern
}

func asPattern(value starlark.Value) (*starlark.Dict, bool) {
	dict, ok := value.(*starlark.Dict)
	if !ok {
		return nil, false
	}
	kind, found, err := dict.Get(starlark.String("kind"))
	if err != nil || !found || kind != starlark.String("re.Pattern") {
		return nil, false
	}
	return dict, true
}

func requireRegexText(fn, param string, value starlark.Value) error {
	switch value.(type) {
	case starlark.String, starlark.Bytes:
		return nil
	default:
		return fmt.Errorf("%s: %s must be str or bytes, got %s", fn, param, value.Type())
	}
}

func requireReplacement(fn string, value starlark.Value) error {
	if _, ok := value.(starlark.Callable); ok {
		return nil
	}
	return requireRegexText(fn, "repl", value)
}

func notImplemented(name string) error {
	return fmt.Errorf("%s is not implemented", name)
}
