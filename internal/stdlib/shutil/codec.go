package shutil

import (
	"fmt"

	"github.com/spachava753/dyson/internal/codec"
	"go.starlark.net/starlark"
)

// RegisterCodecs adds durable support for callbacks returned by ignore_patterns.
func RegisterCodecs(registry codec.Registry) {
	registry.Register(codec.ValueCodec{
		Type:    ignorePatternTypeName,
		Version: 1,
		Serialize: func(value starlark.Value) (codec.SerializedVal, error) {
			ignore, ok := value.(*ignorePatternValue)
			if !ok {
				return codec.SerializedVal{}, fmt.Errorf("dyson: got %T for %s codec", value, ignorePatternTypeName)
			}
			patterns := make(starlark.Tuple, len(ignore.patterns))
			for i, pattern := range ignore.patterns {
				patterns[i] = starlark.String(pattern)
			}
			payload, err := registry.Serialize(patterns)
			if err != nil {
				return codec.SerializedVal{}, err
			}
			return codec.SerializedVal{List: []codec.SerializedVal{payload}}, nil
		},
		Restore: func(value codec.SerializedVal) (starlark.Value, error) {
			if len(value.List) != 1 {
				return nil, fmt.Errorf("dyson: invalid %s payload", ignorePatternTypeName)
			}
			restored, err := registry.Restore(value.List[0])
			if err != nil {
				return nil, err
			}
			patterns, ok := restored.(starlark.Tuple)
			if !ok {
				return nil, fmt.Errorf("dyson: %s patterns must restore to tuple", ignorePatternTypeName)
			}
			result := make([]string, len(patterns))
			for i, value := range patterns {
				pattern, ok := starlark.AsString(value)
				if !ok {
					return nil, fmt.Errorf("dyson: %s pattern %d must restore to string", ignorePatternTypeName, i)
				}
				result[i] = pattern
			}
			return &ignorePatternValue{patterns: result}, nil
		},
	})
}
