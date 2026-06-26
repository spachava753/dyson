package re

import (
	"fmt"
	"regexp"
	"strings"

	"go.starlark.net/starlark"
)

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
	return newPattern("re.compile", expr, flags)
}

func search(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	pattern, text, err := unpackPatternStringFlags("re.search", args, kwargs)
	if err != nil {
		return nil, err
	}
	return pattern.search(text, 0, len(text.text)), nil
}

func match(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	pattern, text, err := unpackPatternStringFlags("re.match", args, kwargs)
	if err != nil {
		return nil, err
	}
	return pattern.match(text, 0, len(text.text)), nil
}

func fullMatch(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	pattern, text, err := unpackPatternStringFlags("re.fullmatch", args, kwargs)
	if err != nil {
		return nil, err
	}
	return pattern.fullmatch(text, 0, len(text.text)), nil
}

func split(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	pattern, text, maxsplit, err := unpackSplitArgs("re.split", args, kwargs)
	if err != nil {
		return nil, err
	}
	return pattern.split(text, maxsplit), nil
}

func findAll(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	pattern, text, err := unpackPatternStringFlags("re.findall", args, kwargs)
	if err != nil {
		return nil, err
	}
	return pattern.findall(text), nil
}

func findIter(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	pattern, text, err := unpackPatternStringFlags("re.finditer", args, kwargs)
	if err != nil {
		return nil, err
	}
	return pattern.finditer(text, 0, len(text.text)), nil
}

func sub(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	pattern, repl, text, count, err := unpackSubArgs("re.sub", args, kwargs)
	if err != nil {
		return nil, err
	}
	value, _, err := pattern.sub(thread, repl, text, count)
	return value, err
}

func subn(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	pattern, repl, text, count, err := unpackSubArgs("re.subn", args, kwargs)
	if err != nil {
		return nil, err
	}
	value, n, err := pattern.sub(thread, repl, text, count)
	if err != nil {
		return nil, err
	}
	return starlark.Tuple{value, starlark.MakeInt(n)}, nil
}

func escape(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var expr starlark.Value
	if err := starlark.UnpackArgs("re.escape", args, kwargs, "pattern", &expr); err != nil {
		return nil, err
	}
	text, err := regexTextFromValue("re.escape", "pattern", expr)
	if err != nil {
		return nil, err
	}
	return text.starlarkValue(regexp.QuoteMeta(text.text)), nil
}

func purge(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs("re.purge", args, kwargs); err != nil {
		return nil, err
	}
	return starlark.None, nil
}

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

func compileOrUsePattern(fn string, expr starlark.Value, flags starlark.Int) (*patternValue, error) {
	if compiled, ok := expr.(*patternValue); ok {
		if flags.Sign() != 0 {
			return nil, fmt.Errorf("%s: cannot process flags argument with a compiled pattern", fn)
		}
		return compiled, nil
	}
	return newPattern(fn, expr, flags)
}

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
