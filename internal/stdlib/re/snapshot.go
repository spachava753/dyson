package re

import (
	"fmt"

	"github.com/spachava753/dyson/snapshot"
	"go.starlark.net/starlark"
)

func init() {
	snapshot.RegisterRestorer("re.Pattern", restorePattern)
	snapshot.RegisterRestorer("re.Match", restoreMatch)
}

func (p *patternValue) ToValue() (starlark.Value, error) {
	return starlark.Tuple{
		p.pattern.value,
		starlark.MakeInt(p.flags),
	}, nil
}

func restorePattern(value starlark.Value) (starlark.Value, error) {
	tuple, ok := value.(starlark.Tuple)
	if !ok || tuple.Len() != 2 {
		return nil, fmt.Errorf("expected (pattern, flags) tuple")
	}
	flags, ok := tuple[1].(starlark.Int)
	if !ok {
		return nil, fmt.Errorf("flags must be int")
	}
	return newPattern("re.compile", tuple[0], flags)
}

func (m *matchValue) ToValue() (starlark.Value, error) {
	indexes := make(starlark.Tuple, len(m.index))
	for i, index := range m.index {
		indexes[i] = starlark.MakeInt(index)
	}
	return starlark.Tuple{
		m.pattern,
		m.input.value,
		starlark.MakeInt(m.pos),
		starlark.MakeInt(m.endpos),
		indexes,
	}, nil
}

func restoreMatch(value starlark.Value) (starlark.Value, error) {
	tuple, ok := value.(starlark.Tuple)
	if !ok || tuple.Len() != 5 {
		return nil, fmt.Errorf("expected (pattern, string, pos, endpos, index) tuple")
	}
	pattern, ok := tuple[0].(*patternValue)
	if !ok {
		return nil, fmt.Errorf("pattern must be re.Pattern")
	}
	text, err := regexTextFromValue("re.Match", "string", tuple[1])
	if err != nil {
		return nil, err
	}
	posValue, ok := tuple[2].(starlark.Int)
	if !ok {
		return nil, fmt.Errorf("pos must be int")
	}
	pos, err := intFromStarlark("re.Match", "pos", posValue)
	if err != nil {
		return nil, err
	}
	endposValue, ok := tuple[3].(starlark.Int)
	if !ok {
		return nil, fmt.Errorf("endpos must be int")
	}
	endpos, err := intFromStarlark("re.Match", "endpos", endposValue)
	if err != nil {
		return nil, err
	}
	indexes, ok := tuple[4].(starlark.Tuple)
	if !ok {
		return nil, fmt.Errorf("index must be tuple")
	}
	index := make([]int, len(indexes))
	for i, item := range indexes {
		indexValue, ok := item.(starlark.Int)
		if !ok {
			return nil, fmt.Errorf("index item %d must be int", i)
		}
		converted, ok := indexValue.Int64()
		if !ok || int64(int(converted)) != converted {
			return nil, fmt.Errorf("index item %d is out of range", i)
		}
		index[i] = int(converted)
	}
	return &matchValue{pattern: pattern, input: text, pos: pos, endpos: endpos, index: index}, nil
}
