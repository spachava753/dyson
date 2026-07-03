package re

import (
	"fmt"

	"github.com/spachava753/dyson/internal/codec"
	"go.starlark.net/starlark"
)

const (
	patternTypeName = "re.Pattern"
	matchTypeName   = "re.Match"
)

// RegisterCodecs installs durable serialization support for re.Pattern and
// re.Match values into a codec registry.
func RegisterCodecs(registry codec.Registry) {
	registry.Register(codec.ValueCodec{
		Type:    patternTypeName,
		Version: 1,
		Serialize: func(val starlark.Value) (codec.SerializedVal, error) {
			p, ok := val.(*patternValue)
			if !ok {
				return codec.SerializedVal{}, fmt.Errorf("dyson: got %T for re.Pattern codec", val)
			}
			pattern, err := registry.Serialize(p.pattern.value)
			if err != nil {
				return codec.SerializedVal{}, err
			}
			flags, err := registry.Serialize(starlark.MakeInt(p.flags))
			if err != nil {
				return codec.SerializedVal{}, err
			}
			return codec.SerializedVal{Type: patternTypeName, List: []codec.SerializedVal{pattern, flags}}, nil
		},
		Restore: func(val codec.SerializedVal) (starlark.Value, error) {
			if len(val.List) != 2 {
				return nil, fmt.Errorf("dyson: invalid re.Pattern payload length %d", len(val.List))
			}
			pattern, err := registry.Restore(val.List[0])
			if err != nil {
				return nil, err
			}
			flagsValue, err := registry.Restore(val.List[1])
			if err != nil {
				return nil, err
			}
			flags, ok := flagsValue.(starlark.Int)
			if !ok {
				return nil, fmt.Errorf("dyson: re.Pattern flags must restore to int, got %s", flagsValue.Type())
			}
			return newPattern(patternTypeName, pattern, flags)
		},
	})

	registry.Register(codec.ValueCodec{
		Type:    matchTypeName,
		Version: 1,
		Serialize: func(val starlark.Value) (codec.SerializedVal, error) {
			m, ok := val.(*matchValue)
			if !ok {
				return codec.SerializedVal{}, fmt.Errorf("dyson: got %T for re.Match codec", val)
			}
			pattern, err := registry.Serialize(m.pattern)
			if err != nil {
				return codec.SerializedVal{}, err
			}
			input, err := registry.Serialize(m.input.value)
			if err != nil {
				return codec.SerializedVal{}, err
			}
			pos, err := registry.Serialize(starlark.MakeInt(m.pos))
			if err != nil {
				return codec.SerializedVal{}, err
			}
			endpos, err := registry.Serialize(starlark.MakeInt(m.endpos))
			if err != nil {
				return codec.SerializedVal{}, err
			}
			indexes := make(starlark.Tuple, len(m.index))
			for i, index := range m.index {
				indexes[i] = starlark.MakeInt(index)
			}
			index, err := registry.Serialize(indexes)
			if err != nil {
				return codec.SerializedVal{}, err
			}
			return codec.SerializedVal{Type: matchTypeName, List: []codec.SerializedVal{pattern, input, pos, endpos, index}}, nil
		},
		Restore: func(val codec.SerializedVal) (starlark.Value, error) {
			if len(val.List) != 5 {
				return nil, fmt.Errorf("dyson: invalid re.Match payload length %d", len(val.List))
			}
			storedPattern, err := registry.Restore(val.List[0])
			if err != nil {
				return nil, err
			}
			pattern, ok := storedPattern.(*patternValue)
			if !ok {
				return nil, fmt.Errorf("dyson: re.Match pattern must restore to re.Pattern, got %s", storedPattern.Type())
			}
			inputValue, err := registry.Restore(val.List[1])
			if err != nil {
				return nil, err
			}
			input, err := regexTextFromValue(matchTypeName, "string", inputValue)
			if err != nil {
				return nil, err
			}
			pos, err := restoreInt(registry, val.List[2], "pos")
			if err != nil {
				return nil, err
			}
			endpos, err := restoreInt(registry, val.List[3], "endpos")
			if err != nil {
				return nil, err
			}
			indexesValue, err := registry.Restore(val.List[4])
			if err != nil {
				return nil, err
			}
			indexesTuple, ok := indexesValue.(starlark.Tuple)
			if !ok {
				return nil, fmt.Errorf("dyson: re.Match index must restore to tuple, got %s", indexesValue.Type())
			}
			indexes := make([]int, len(indexesTuple))
			for i, item := range indexesTuple {
				indexValue, ok := item.(starlark.Int)
				if !ok {
					return nil, fmt.Errorf("dyson: re.Match index item %d must restore to int, got %s", i, item.Type())
				}
				index, ok := indexValue.Int64()
				if !ok || int64(int(index)) != index {
					return nil, fmt.Errorf("dyson: re.Match index item %d is out of range", i)
				}
				indexes[i] = int(index)
			}
			return &matchValue{pattern: pattern, input: input, pos: pos, endpos: endpos, index: indexes}, nil
		},
	})
}

func restoreInt(registry codec.Registry, val codec.SerializedVal, name string) (int, error) {
	restored, err := registry.Restore(val)
	if err != nil {
		return 0, err
	}
	intValue, ok := restored.(starlark.Int)
	if !ok {
		return 0, fmt.Errorf("dyson: re.Match %s must restore to int, got %s", name, restored.Type())
	}
	converted, ok := intValue.Int64()
	if !ok || int64(int(converted)) != converted {
		return 0, fmt.Errorf("dyson: re.Match %s is out of range", name)
	}
	return int(converted), nil
}
