package re

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spachava753/dyson/internal/xctx"
	"go.starlark.net/starlark"
)

// compile implements the Starlark re.compile builtin, returning a compiled
// pattern for a string or bytes pattern and returning an existing compiled
// pattern unchanged when no new flags are supplied.
//
// It mirrors the supported subset of Python's re.compile:
// https://docs.python.org/3/library/re.html#re.compile
func compile(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var expr starlark.Value
	flags := starlark.MakeInt(0)
	if err := starlark.UnpackArgs("re.compile", args, kwargs, "pattern", &expr, "flags?", &flags); err != nil {
		return nil, err
	}

	if compiled, ok := expr.(*patternValue); ok {
		if flags.Sign() != 0 {
			return nil, fmt.Errorf("re.compile: cannot process flags argument with a compiled pattern")
		}
		return compiled, nil
	}
	if err := xctx.Check(thread); err != nil {
		return nil, err
	}
	return newPattern("re.compile", expr, flags)
}

// search implements the Starlark re.search builtin, scanning the string for the
// first match and returning either a Match value or None.
//
// It mirrors the supported subset of Python's re.search:
// https://docs.python.org/3/library/re.html#re.search
func search(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	pattern, text, err := unpackPatternStringFlags("re.search", args, kwargs)
	if err != nil {
		return nil, err
	}
	if err := xctx.Check(thread); err != nil {
		return nil, err
	}
	index := pattern.re.FindStringSubmatchIndex(text.text)
	if index == nil {
		return starlark.None, nil
	}
	shiftIndex(index, text.offset)
	return &matchValue{pattern: pattern, input: text, endpos: len(text.text), index: index}, nil
}

// match implements the Starlark re.match builtin, matching only at the start of
// the string and returning either a Match value or None.
//
// It mirrors the supported subset of Python's re.match:
// https://docs.python.org/3/library/re.html#re.match
func match(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	pattern, text, err := unpackPatternStringFlags("re.match", args, kwargs)
	if err != nil {
		return nil, err
	}
	if err := xctx.Check(thread); err != nil {
		return nil, err
	}
	index := pattern.re.FindStringSubmatchIndex(text.text)
	if index == nil || index[0] != 0 {
		return starlark.None, nil
	}
	shiftIndex(index, text.offset)
	return &matchValue{pattern: pattern, input: text, endpos: len(text.text), index: index}, nil
}

// fullMatch implements the Starlark re.fullmatch builtin, requiring the entire
// string to match the pattern and returning either a Match value or None.
//
// It mirrors the supported subset of Python's re.fullmatch:
// https://docs.python.org/3/library/re.html#re.fullmatch
func fullMatch(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	pattern, text, err := unpackPatternStringFlags("re.fullmatch", args, kwargs)
	if err != nil {
		return nil, err
	}
	if err := xctx.Check(thread); err != nil {
		return nil, err
	}
	index := pattern.re.FindStringSubmatchIndex(text.text)
	if index == nil || index[0] != 0 || index[1] != len(text.text) {
		return starlark.None, nil
	}
	shiftIndex(index, text.offset)
	return &matchValue{pattern: pattern, input: text, endpos: len(text.text), index: index}, nil
}

// split implements the Starlark re.split builtin, splitting the string by the
// pattern and including captured separators in the result when the pattern has
// capturing groups.
//
// It mirrors the supported subset of Python's re.split:
// https://docs.python.org/3/library/re.html#re.split
func split(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	pattern, text, maxsplit, err := unpackSplitArgs("re.split", args, kwargs)
	if err != nil {
		return nil, err
	}
	if err := xctx.Check(thread); err != nil {
		return nil, err
	}
	return splitPattern(thread, pattern, text, maxsplit)
}

// findAll implements the Starlark re.findall builtin, returning all non-
// overlapping matches as strings, bytes, or tuples depending on the number of
// capturing groups in the pattern.
//
// It mirrors the supported subset of Python's re.findall:
// https://docs.python.org/3/library/re.html#re.findall
func findAll(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	pattern, text, err := unpackPatternStringFlags("re.findall", args, kwargs)
	if err != nil {
		return nil, err
	}
	if err := xctx.Check(thread); err != nil {
		return nil, err
	}
	return findallPattern(thread, pattern, text)
}

// findIter implements the Starlark re.finditer builtin, returning a list of
// Match values for all non-overlapping matches.
//
// TODO: Return a Starlark iterator instead of a list; Python's re.finditer
// returns an iterator.
//
// It mirrors the supported subset of Python's re.finditer:
// https://docs.python.org/3/library/re.html#re.finditer
func findIter(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	pattern, text, err := unpackPatternStringFlags("re.finditer", args, kwargs)
	if err != nil {
		return nil, err
	}
	if err := xctx.Check(thread); err != nil {
		return nil, err
	}
	return finditerPattern(thread, pattern, text, 0, len(text.text))
}

// sub implements the Starlark re.sub builtin, replacing non-overlapping matches
// with either an expanded replacement template or the result of a replacement
// callable.
//
// It mirrors the supported subset of Python's re.sub:
// https://docs.python.org/3/library/re.html#re.sub
func sub(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	pattern, repl, text, count, err := unpackSubArgs("re.sub", args, kwargs)
	if err != nil {
		return nil, err
	}
	value, _, err := subPattern(thread, pattern, repl, text, count)
	return value, err
}

// subn implements the Starlark re.subn builtin, returning a tuple containing the
// substituted text and the number of replacements made.
//
// It mirrors the supported subset of Python's re.subn:
// https://docs.python.org/3/library/re.html#re.subn
func subn(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	pattern, repl, text, count, err := unpackSubArgs("re.subn", args, kwargs)
	if err != nil {
		return nil, err
	}
	value, n, err := subPattern(thread, pattern, repl, text, count)
	if err != nil {
		return nil, err
	}
	return starlark.Tuple{value, starlark.MakeInt(n)}, nil
}

// escape implements the Starlark re.escape builtin, escaping regex metacharacters
// in a string or bytes pattern so the result can be matched literally.
//
// It mirrors the supported subset of Python's re.escape:
// https://docs.python.org/3/library/re.html#re.escape
func escape(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var expr starlark.Value
	if err := starlark.UnpackArgs("re.escape", args, kwargs, "pattern", &expr); err != nil {
		return nil, err
	}
	text, err := regexTextFromValue("re.escape", "pattern", expr)
	if err != nil {
		return nil, err
	}
	if err := xctx.Check(thread); err != nil {
		return nil, err
	}
	return text.starlarkValue(regexp.QuoteMeta(text.text)), nil
}

// purge implements the Starlark re.purge builtin. Dyson does not keep a regex
// cache, so this compatibility hook only validates that no arguments were passed
// and returns None.
//
// It mirrors the observable behavior of Python's re.purge:
// https://docs.python.org/3/library/re.html#re.purge
func purge(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("re.purge", args, kwargs); err != nil {
		return nil, err
	}
	return starlark.None, nil
}

// unpackPatternStringFlags decodes the common module-level argument shape
// (pattern, string, flags=0), compiling a string or bytes pattern while reusing
// an existing compiled pattern when Python permits it.
func unpackPatternStringFlags(fn string, args starlark.Tuple, kwargs []starlark.Tuple) (*patternValue, regexText, error) {
	var expr, textValue starlark.Value
	flags := starlark.MakeInt(0)
	if err := starlark.UnpackArgs(fn, args, kwargs, "pattern", &expr, "string", &textValue, "flags?", &flags); err != nil {
		return nil, regexText{}, err
	}
	pattern, err := compileOrUsePattern(fn, expr, flags)
	if err != nil {
		return nil, regexText{}, err
	}
	text, err := regexTextFromValue(fn, "string", textValue)
	if err != nil {
		return nil, regexText{}, err
	}
	return pattern, text, nil
}

// unpackSplitArgs decodes re.split's module-level arguments, including the
// Python-compatible maxsplit and flags parameters.
func unpackSplitArgs(fn string, args starlark.Tuple, kwargs []starlark.Tuple) (*patternValue, regexText, int, error) {
	var expr, textValue starlark.Value
	maxsplitValue := starlark.MakeInt(0)
	flags := starlark.MakeInt(0)
	if err := starlark.UnpackArgs(fn, args, kwargs, "pattern", &expr, "string", &textValue, "maxsplit?", &maxsplitValue, "flags?", &flags); err != nil {
		return nil, regexText{}, 0, err
	}
	pattern, err := compileOrUsePattern(fn, expr, flags)
	if err != nil {
		return nil, regexText{}, 0, err
	}
	text, err := regexTextFromValue(fn, "string", textValue)
	if err != nil {
		return nil, regexText{}, 0, err
	}
	maxsplit, err := intFromStarlark(fn, "maxsplit", maxsplitValue)
	if err != nil {
		return nil, regexText{}, 0, err
	}
	return pattern, text, maxsplit, nil
}

// unpackSubArgs decodes re.sub and re.subn's module-level arguments, accepting a
// string or bytes replacement template or a Starlark callable replacement.
func unpackSubArgs(fn string, args starlark.Tuple, kwargs []starlark.Tuple) (*patternValue, starlark.Value, regexText, int, error) {
	var expr, repl, textValue starlark.Value
	countValue := starlark.MakeInt(0)
	flags := starlark.MakeInt(0)
	if err := starlark.UnpackArgs(fn, args, kwargs, "pattern", &expr, "repl", &repl, "string", &textValue, "count?", &countValue, "flags?", &flags); err != nil {
		return nil, nil, regexText{}, 0, err
	}
	pattern, err := compileOrUsePattern(fn, expr, flags)
	if err != nil {
		return nil, nil, regexText{}, 0, err
	}
	if _, ok := repl.(starlark.Callable); !ok {
		if _, err := regexTextFromValue(fn, "repl", repl); err != nil {
			return nil, nil, regexText{}, 0, err
		}
	}
	text, err := regexTextFromValue(fn, "string", textValue)
	if err != nil {
		return nil, nil, regexText{}, 0, err
	}
	count, err := intFromStarlark(fn, "count", countValue)
	if err != nil {
		return nil, nil, regexText{}, 0, err
	}
	return pattern, repl, text, count, nil
}

// compileOrUsePattern applies Python's module-level rule for pattern arguments:
// compiled patterns can be reused only when no additional flags are supplied;
// other pattern values are compiled with the provided flags.
func compileOrUsePattern(fn string, expr starlark.Value, flags starlark.Int) (*patternValue, error) {
	if compiled, ok := expr.(*patternValue); ok {
		if flags.Sign() != 0 {
			return nil, fmt.Errorf("%s: cannot process flags argument with a compiled pattern", fn)
		}
		return compiled, nil
	}
	return newPattern(fn, expr, flags)
}

// newPattern validates Starlark pattern and flag values, compiles them with the
// package's RE2-backed compiler, and packages the resulting metadata into a
// durable Pattern value.
func newPattern(fn string, expr starlark.Value, flagsValue starlark.Int) (*patternValue, error) {
	text, err := regexTextFromValue(fn, "pattern", expr)
	if err != nil {
		return nil, err
	}
	flags, err := intFromStarlark(fn, "flags", flagsValue)
	if err != nil {
		return nil, err
	}
	re, names, groupIndex, err := compileRegex(fn, text, flags)
	if err != nil {
		return nil, err
	}
	return &patternValue{pattern: text, flags: flags, expr: re.String(), re: re, groups: re.NumSubexp(), groupNames: names, groupIndex: groupIndex}, nil
}

// expandReplacement expands the Python-style replacement escapes supported by
// this module, including numbered references, named references, and common
// character escapes used by sub, subn, and Match.expand.
func expandReplacement(template string, match *matchValue) string {
	var out strings.Builder
	for i := 0; i < len(template); i++ {
		ch := template[i]
		if ch != '\\' || i+1 >= len(template) {
			out.WriteByte(ch)
			continue
		}
		i++
		next := template[i]
		switch {
		case next >= '1' && next <= '9':
			group := int(next - '0')
			out.WriteString(match.groupString(group))
		case next == 'g' && i+1 < len(template) && template[i+1] == '<':
			end := strings.IndexByte(template[i+2:], '>')
			if end < 0 {
				out.WriteString("\\g")
				continue
			}
			name := template[i+2 : i+2+end]
			i += end + 2
			if n, ok := parseGroupRef(name, match.pattern); ok {
				out.WriteString(match.groupString(n))
			}
		case next == 'n':
			out.WriteByte('\n')
		case next == 't':
			out.WriteByte('\t')
		case next == 'r':
			out.WriteByte('\r')
		default:
			out.WriteByte(next)
		}
	}
	return out.String()
}

// parseGroupRef resolves a replacement-template group reference such as "0",
// "1", or a named group to its numeric group index.
func parseGroupRef(ref string, pattern *patternValue) (int, bool) {
	if ref == "0" {
		return 0, true
	}
	var n int
	for _, r := range ref {
		if r < '0' || r > '9' {
			value, found, err := pattern.groupIndex.Get(starlark.String(ref))
			if err != nil || !found {
				return 0, false
			}
			group, err := starlark.AsInt32(value)
			return group, err == nil
		}
		n = n*10 + int(r-'0')
	}
	return n, n <= pattern.groups
}
