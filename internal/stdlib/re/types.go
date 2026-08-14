package re

import (
	"regexp"

	"github.com/spachava753/starlarkx/starlark"
)

type regexText struct {
	value  starlark.Value
	text   string
	bytes  bool
	offset int
}

func (t regexText) window(pos, endpos int) regexText {
	return regexText{value: t.value, text: t.text[pos:endpos], bytes: t.bytes, offset: t.offset + pos}
}

func (t regexText) starlarkValue(s string) starlark.Value {
	if t.bytes {
		return starlark.Bytes(s)
	}
	return starlark.String(s)
}

type patternValue struct {
	pattern    regexText
	flags      int
	expr       string
	re         *regexp.Regexp
	groups     int
	groupNames []string
	groupIndex *starlark.Dict
	frozen     bool
}

type matchValue struct {
	pattern *patternValue
	input   regexText
	pos     int
	endpos  int
	index   []int
	frozen  bool
}
