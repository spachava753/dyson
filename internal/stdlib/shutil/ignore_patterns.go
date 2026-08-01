package shutil

import (
	"fmt"

	"go.starlark.net/starlark"
)

const ignorePatternFuncName = ModuleName + ".ignore_patterns.<locals>._ignore"

func (m moduleFunctions) ignorePatterns(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(kwargs) != 0 {
		return nil, fmt.Errorf("%s: unexpected keyword argument %s", fn.Name(), kwargs[0][0])
	}
	patterns := make([]string, len(args))
	for i, value := range args {
		pattern, ok := starlark.AsString(value)
		if !ok {
			return nil, fmt.Errorf("%s: pattern %d must be a string", fn.Name(), i)
		}
		patterns[i] = pattern
	}

	return starlark.NewBuiltin(ignorePatternFuncName, func(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var path starlark.Value
		var names starlark.Iterable
		if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path, "names", &names); err != nil {
			return nil, err
		}
		_ = path
		var ignored []starlark.Value
		iterator := names.Iterate()
		defer iterator.Done()
		var value starlark.Value
		for iterator.Next(&value) {
			name, ok := starlark.AsString(value)
			if !ok {
				return nil, fmt.Errorf("%s: name must be a string", fn.Name())
			}
			for _, pattern := range patterns {
				if matchGlob(name, pattern) {
					ignored = append(ignored, value)
					break
				}
			}
		}
		return starlark.NewList(ignored), nil
	}), nil
}

func matchGlob(name, pattern string) bool {
	matched := make([]bool, len(pattern)+1)
	matched[0] = true
	for j := 1; j <= len(pattern); j++ {
		matched[j] = matched[j-1] && pattern[j-1] == '*'
	}
	for i := 1; i <= len(name); i++ {
		diagonal := matched[0]
		matched[0] = false
		for j := 1; j <= len(pattern); j++ {
			above := matched[j]
			switch pattern[j-1] {
			case '*':
				matched[j] = matched[j-1] || above
			case '?':
				matched[j] = diagonal
			default:
				matched[j] = diagonal && pattern[j-1] == name[i-1]
			}
			diagonal = above
		}
	}
	return matched[len(pattern)]
}
