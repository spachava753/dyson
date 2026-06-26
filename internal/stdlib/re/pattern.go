package re

import (
	"fmt"
	"strings"

	"github.com/spachava753/dyson/internal/xctx"
	"go.starlark.net/starlark"
)

// String returns a Python-like representation of the compiled pattern value.
func (p *patternValue) String() string { return "re.compile(" + p.pattern.value.String() + ")" }

// Type reports the Starlark-visible type name for compiled patterns.
func (p *patternValue) Type() string { return "re.Pattern" }

// Freeze marks the pattern immutable for Starlark's shared-value semantics.
func (p *patternValue) Freeze() { p.frozen = true }

// Truth reports that compiled patterns are always truthy.
func (p *patternValue) Truth() starlark.Bool { return starlark.True }

// Hash rejects hashing because Python re.Pattern objects are not hashable in
// this compatibility layer.
func (p *patternValue) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: re.Pattern") }

// Attr exposes Python-compatible Pattern attributes and bound methods.
//
// It mirrors the supported subset of Python's re.Pattern API:
// https://docs.python.org/3/library/re.html#re.Pattern
func (p *patternValue) Attr(name string) (starlark.Value, error) {
	switch name {
	case "pattern":
		return p.pattern.value, nil
	case "flags":
		return starlark.MakeInt(p.flags), nil
	case "groups":
		return starlark.MakeInt(p.groups), nil
	case "groupindex":
		return p.groupIndex, nil
	case "search", "match", "fullmatch", "split", "findall", "finditer", "sub", "subn":
		return starlark.NewBuiltin("re.Pattern."+name, p.method(name)), nil
	}
	return nil, nil
}

// AttrNames returns the names discoverable on compiled Pattern values.
func (p *patternValue) AttrNames() []string { return patternAttrNames }

// method builds the Starlark builtin implementation for a Python-compatible
// Pattern method name.
func (p *patternValue) method(name string) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		switch name {
		case "search", "match", "fullmatch":
			text, pos, endpos, err := unpackPatternMethodArgs(fn.Name(), args, kwargs)
			if err != nil {
				return nil, err
			}
			if err := xctx.Check(thread); err != nil {
				return nil, err
			}
			switch name {
			case "search":
				return p.search(text, pos, endpos), nil
			case "match":
				return p.match(text, pos, endpos), nil
			default:
				return p.fullmatch(text, pos, endpos), nil
			}
		case "split":
			text, maxsplit, err := unpackPatternSplitArgs(fn.Name(), args, kwargs)
			if err != nil {
				return nil, err
			}
			if err := xctx.Check(thread); err != nil {
				return nil, err
			}
			return p.split(thread, text, maxsplit)
		case "findall", "finditer":
			text, pos, endpos, err := unpackPatternMethodArgs(fn.Name(), args, kwargs)
			if err != nil {
				return nil, err
			}
			if err := xctx.Check(thread); err != nil {
				return nil, err
			}
			if name == "findall" {
				return p.findall(thread, text.window(pos, endpos))
			}
			return p.finditer(thread, text, pos, endpos)
		case "sub", "subn":
			repl, text, count, err := unpackPatternSubArgs(fn.Name(), args, kwargs)
			if err != nil {
				return nil, err
			}
			value, n, err := p.sub(thread, repl, text, count)
			if err != nil {
				return nil, err
			}
			if name == "subn" {
				return starlark.Tuple{value, starlark.MakeInt(n)}, nil
			}
			return value, nil
		}
		return nil, nil
	}
}

// search implements Pattern.search, scanning the requested window for the first
// match and returning either a Match value or None.
//
// It mirrors the supported subset of Python's Pattern.search:
// https://docs.python.org/3/library/re.html#re.Pattern.search
func (p *patternValue) search(text regexText, pos, endpos int) starlark.Value {
	index := p.re.FindStringSubmatchIndex(text.text[pos:endpos])
	if index == nil {
		return starlark.None
	}
	shiftIndex(index, text.offset+pos)
	return &matchValue{pattern: p, input: text, pos: pos, endpos: endpos, index: index}
}

// match implements Pattern.match, requiring a match at the beginning of the
// requested window and returning either a Match value or None.
//
// It mirrors the supported subset of Python's Pattern.match:
// https://docs.python.org/3/library/re.html#re.Pattern.match
func (p *patternValue) match(text regexText, pos, endpos int) starlark.Value {
	index := p.re.FindStringSubmatchIndex(text.text[pos:endpos])
	if index == nil || index[0] != 0 {
		return starlark.None
	}
	shiftIndex(index, text.offset+pos)
	return &matchValue{pattern: p, input: text, pos: pos, endpos: endpos, index: index}
}

// fullmatch implements Pattern.fullmatch, requiring the requested window to be
// fully matched and returning either a Match value or None.
//
// It mirrors the supported subset of Python's Pattern.fullmatch:
// https://docs.python.org/3/library/re.html#re.Pattern.fullmatch
func (p *patternValue) fullmatch(text regexText, pos, endpos int) starlark.Value {
	index := p.re.FindStringSubmatchIndex(text.text[pos:endpos])
	if index == nil || index[0] != 0 || index[1] != endpos-pos {
		return starlark.None
	}
	shiftIndex(index, text.offset+pos)
	return &matchValue{pattern: p, input: text, pos: pos, endpos: endpos, index: index}
}

// split implements Pattern.split, splitting text around non-overlapping matches
// and including captured groups in the result.
//
// It mirrors the supported subset of Python's Pattern.split:
// https://docs.python.org/3/library/re.html#re.Pattern.split
func (p *patternValue) split(thread *starlark.Thread, text regexText, maxsplit int) (*starlark.List, error) {
	matches := p.re.FindAllStringSubmatchIndex(text.text, -1)
	items := []starlark.Value{}
	last := 0
	splits := 0
	for _, index := range matches {
		if err := xctx.Check(thread); err != nil {
			return nil, err
		}
		if maxsplit > 0 && splits >= maxsplit {
			break
		}
		items = append(items, text.starlarkValue(text.text[last:index[0]]))
		for group := 1; group <= p.groups; group++ {
			start, end := index[group*2], index[group*2+1]
			if start < 0 {
				items = append(items, starlark.None)
			} else {
				items = append(items, text.starlarkValue(text.text[start:end]))
			}
		}
		last = index[1]
		splits++
	}
	items = append(items, text.starlarkValue(text.text[last:]))
	return starlark.NewList(items), nil
}

// findall implements Pattern.findall, returning all non-overlapping matches as
// strings, bytes, or tuples depending on the pattern's capturing groups.
//
// It mirrors the supported subset of Python's Pattern.findall:
// https://docs.python.org/3/library/re.html#re.Pattern.findall
func (p *patternValue) findall(thread *starlark.Thread, text regexText) (*starlark.List, error) {
	matches := p.re.FindAllStringSubmatchIndex(text.text, -1)
	items := make([]starlark.Value, 0, len(matches))
	for _, index := range matches {
		if err := xctx.Check(thread); err != nil {
			return nil, err
		}
		switch p.groups {
		case 0:
			items = append(items, text.starlarkValue(text.text[index[0]:index[1]]))
		case 1:
			start, end := index[2], index[3]
			if start < 0 {
				items = append(items, text.starlarkValue(""))
			} else {
				items = append(items, text.starlarkValue(text.text[start:end]))
			}
		default:
			groups := make(starlark.Tuple, p.groups)
			for group := 1; group <= p.groups; group++ {
				start, end := index[group*2], index[group*2+1]
				if start < 0 {
					groups[group-1] = text.starlarkValue("")
				} else {
					groups[group-1] = text.starlarkValue(text.text[start:end])
				}
			}
			items = append(items, groups)
		}
	}
	return starlark.NewList(items), nil
}

// finditer implements Pattern.finditer. Dyson returns a list of Match values
// for Starlark consumption rather than a lazy Python iterator.
//
// TODO: Return a Starlark iterator instead of a list; Python's Pattern.finditer
// returns an iterator.
//
// It mirrors the supported subset of Python's Pattern.finditer:
// https://docs.python.org/3/library/re.html#re.Pattern.finditer
func (p *patternValue) finditer(thread *starlark.Thread, text regexText, pos, endpos int) (*starlark.List, error) {
	window := text.window(pos, endpos)
	matches := p.re.FindAllStringSubmatchIndex(window.text, -1)
	items := make([]starlark.Value, 0, len(matches))
	for _, index := range matches {
		if err := xctx.Check(thread); err != nil {
			return nil, err
		}
		shiftIndex(index, window.offset)
		items = append(items, &matchValue{pattern: p, input: window, pos: pos, endpos: endpos, index: index})
	}
	return starlark.NewList(items), nil
}

// sub implements Pattern.sub and the shared replacement work for Pattern.subn,
// returning the substituted value together with the replacement count.
//
// It mirrors the supported subset of Python's Pattern.sub:
// https://docs.python.org/3/library/re.html#re.Pattern.sub
func (p *patternValue) sub(thread *starlark.Thread, repl starlark.Value, text regexText, count int) (starlark.Value, int, error) {
	matches := p.re.FindAllStringSubmatchIndex(text.text, -1)
	if count > 0 && len(matches) > count {
		matches = matches[:count]
	}
	var out strings.Builder
	last := 0
	for _, index := range matches {
		if err := xctx.Check(thread); err != nil {
			return nil, 0, err
		}
		out.WriteString(text.text[last:index[0]])
		match := &matchValue{pattern: p, input: text, endpos: len(text.text), index: index}
		if callable, ok := repl.(starlark.Callable); ok {
			value, err := starlark.Call(thread, callable, starlark.Tuple{match}, nil)
			if err != nil {
				return nil, 0, err
			}
			replText, err := regexTextFromValue("re.sub", "repl", value)
			if err != nil {
				return nil, 0, err
			}
			out.WriteString(replText.text)
		} else {
			replText, _ := regexTextFromValue("re.sub", "repl", repl)
			out.WriteString(expandReplacement(replText.text, match))
		}
		last = index[1]
	}
	out.WriteString(text.text[last:])
	return text.starlarkValue(out.String()), len(matches), nil
}

// unpackPatternMethodArgs decodes Pattern search-like method arguments,
// including Python-compatible pos and endpos bounds.
func unpackPatternMethodArgs(fn string, args starlark.Tuple, kwargs []starlark.Tuple) (regexText, int, int, error) {
	var textValue starlark.Value
	posValue := starlark.MakeInt(0)
	var endposValue starlark.Value = starlark.None
	if err := starlark.UnpackArgs(fn, args, kwargs, "string", &textValue, "pos?", &posValue, "endpos?", &endposValue); err != nil {
		return regexText{}, 0, 0, err
	}
	text, err := regexTextFromValue(fn, "string", textValue)
	if err != nil {
		return regexText{}, 0, 0, err
	}
	pos, err := intFromStarlark(fn, "pos", posValue)
	if err != nil {
		return regexText{}, 0, 0, err
	}
	endpos := len(text.text)
	if endposValue != starlark.None {
		value, ok := endposValue.(starlark.Int)
		if !ok {
			return regexText{}, 0, 0, fmt.Errorf("%s: for parameter endpos: got %s, want int", fn, endposValue.Type())
		}
		endpos, err = intFromStarlark(fn, "endpos", value)
		if err != nil {
			return regexText{}, 0, 0, err
		}
	}
	pos = min(pos, len(text.text))
	endpos = min(endpos, len(text.text))
	if endpos < pos {
		endpos = pos
	}
	return text, pos, endpos, nil
}

// unpackPatternSplitArgs decodes Pattern.split arguments and validates maxsplit.
func unpackPatternSplitArgs(fn string, args starlark.Tuple, kwargs []starlark.Tuple) (regexText, int, error) {
	var textValue starlark.Value
	maxsplitValue := starlark.MakeInt(0)
	if err := starlark.UnpackArgs(fn, args, kwargs, "string", &textValue, "maxsplit?", &maxsplitValue); err != nil {
		return regexText{}, 0, err
	}
	text, err := regexTextFromValue(fn, "string", textValue)
	if err != nil {
		return regexText{}, 0, err
	}
	maxsplit, err := intFromStarlark(fn, "maxsplit", maxsplitValue)
	if err != nil {
		return regexText{}, 0, err
	}
	return text, maxsplit, nil
}

// unpackPatternSubArgs decodes Pattern.sub and Pattern.subn arguments,
// accepting either a replacement template or a callable replacement.
func unpackPatternSubArgs(fn string, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, regexText, int, error) {
	var repl, textValue starlark.Value
	countValue := starlark.MakeInt(0)
	if err := starlark.UnpackArgs(fn, args, kwargs, "repl", &repl, "string", &textValue, "count?", &countValue); err != nil {
		return nil, regexText{}, 0, err
	}
	if _, ok := repl.(starlark.Callable); !ok {
		if _, err := regexTextFromValue(fn, "repl", repl); err != nil {
			return nil, regexText{}, 0, err
		}
	}
	text, err := regexTextFromValue(fn, "string", textValue)
	if err != nil {
		return nil, regexText{}, 0, err
	}
	count, err := intFromStarlark(fn, "count", countValue)
	if err != nil {
		return nil, regexText{}, 0, err
	}
	return repl, text, count, nil
}

// shiftIndex translates regexp match offsets from a sliced search window back to
// offsets in the original input text.
func shiftIndex(index []int, offset int) {
	for i, value := range index {
		if value >= 0 {
			index[i] = value + offset
		}
	}
}
