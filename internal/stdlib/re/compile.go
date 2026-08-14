package re

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spachava753/starlarkx/starlark"
)

func compileRegex(fn string, text regexText, flags int) (*regexp.Regexp, []string, *starlark.Dict, error) {
	if flags&flagLocale != 0 {
		return nil, nil, nil, fmt.Errorf("%s: LOCALE is not supported", fn)
	}
	if flags&flagDebug != 0 {
		return nil, nil, nil, fmt.Errorf("%s: DEBUG is not supported", fn)
	}

	expr := translatePattern(text.text, flags)
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%s: %v", fn, err)
	}

	names := re.SubexpNames()
	groupIndex := starlark.NewDict(len(names))
	for i, name := range names {
		if i == 0 || name == "" {
			continue
		}
		mustSet(groupIndex, name, starlark.MakeInt(i))
	}
	groupIndex.Freeze()

	return re, names, groupIndex, nil
}

func translatePattern(pattern string, flags int) string {
	prefix := ""
	if flags&flagIgnoreCase != 0 {
		prefix += "i"
	}
	if flags&flagMultiline != 0 {
		prefix += "m"
	}
	if flags&flagDotAll != 0 {
		prefix += "s"
	}
	if prefix != "" {
		pattern = "(?" + prefix + ")" + pattern
	}
	if flags&flagVerbose != 0 {
		pattern = stripVerbosePattern(pattern)
	}
	return pattern
}

// stripVerbosePattern removes unescaped whitespace and # comments outside
// character classes while preserving escapes and class contents.
func stripVerbosePattern(pattern string) string {
	var out strings.Builder
	inClass := false
	escaped := false
	comment := false

	for _, r := range pattern {
		if comment {
			if r == '\n' || r == '\r' {
				comment = false
			}
			continue
		}
		if escaped {
			out.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			out.WriteRune(r)
			escaped = true
			continue
		}
		if inClass {
			out.WriteRune(r)
			if r == ']' {
				inClass = false
			}
			continue
		}
		switch r {
		case '[':
			inClass = true
			out.WriteRune(r)
		case '#':
			comment = true
		case ' ', '\t', '\n', '\r', '\f', '\v':
			continue
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}

func regexTextFromValue(fn, param string, value starlark.Value) (regexText, error) {
	switch v := value.(type) {
	case starlark.String:
		return regexText{value: value, text: string(v)}, nil
	case starlark.Bytes:
		return regexText{value: value, text: string(v), bytes: true}, nil
	default:
		return regexText{}, fmt.Errorf("%s: %s must be str or bytes, got %s", fn, param, value.Type())
	}
}

func intFromStarlark(fn, param string, value starlark.Int) (int, error) {
	i, ok := value.Int64()
	if !ok || int64(int(i)) != i {
		return 0, fmt.Errorf("%s: %s is out of range", fn, param)
	}
	if i < 0 {
		return 0, fmt.Errorf("%s: %s must be >= 0", fn, param)
	}
	return int(i), nil
}
