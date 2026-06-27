package builtins

import (
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// minBuiltin implements the Starlark min builtin for either one iterable or
// multiple positional values, with Python-compatible key and default handling.
//
// It mirrors the supported subset of Python's min:
// https://docs.python.org/3/library/functions.html#min
func minBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return extremumBuiltin(thread, fn, args, kwargs, syntax.LT)
}

// maxBuiltin implements the Starlark max builtin for either one iterable or
// multiple positional values, with Python-compatible key and default handling.
//
// It mirrors the supported subset of Python's max:
// https://docs.python.org/3/library/functions.html#max
func maxBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return extremumBuiltin(thread, fn, args, kwargs, syntax.GT)
}

// extremumBuiltin contains the shared min/max implementation, applying the
// selected comparison operator to either raw values or key(value) results.
func extremumBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple, op syntax.Token) (starlark.Value, error) {
	key := starlark.Value(starlark.None)
	defaultValue := starlark.Value(nil)
	if len(kwargs) > 0 {
		filtered := make([]starlark.Tuple, 0, len(kwargs))
		for _, kw := range kwargs {
			name := string(kw[0].(starlark.String))
			switch name {
			case "key":
				key = kw[1]
			case "default":
				defaultValue = kw[1]
			default:
				filtered = append(filtered, kw)
			}
		}
		if len(filtered) > 0 {
			return nil, fmt.Errorf("%s: unexpected keyword argument %s", fn.Name(), filtered[0][0])
		}
	}
	if key != starlark.None {
		if _, ok := key.(starlark.Callable); !ok {
			return nil, fmt.Errorf("%s: key must be callable or None, got %s", fn.Name(), key.Type())
		}
	}
	var values []starlark.Value
	if len(args) == 0 {
		return nil, fmt.Errorf("%s: expected at least 1 argument, got 0", fn.Name())
	}
	if len(args) == 1 {
		var err error
		values, err = iterableValues(fn.Name(), args[0])
		if err != nil {
			return nil, err
		}
	} else {
		if defaultValue != nil {
			return nil, fmt.Errorf("%s: default can only be used with a single iterable argument", fn.Name())
		}
		values = append(values, args...)
	}
	if len(values) == 0 {
		if defaultValue != nil {
			return defaultValue, nil
		}
		return nil, fmt.Errorf("%s: arg is an empty sequence", fn.Name())
	}
	best := values[0]
	bestKey := best
	if key != starlark.None {
		var err error
		bestKey, err = starlark.Call(thread, key, starlark.Tuple{best}, nil)
		if err != nil {
			return nil, err
		}
	}
	for _, value := range values[1:] {
		valueKey := value
		if key != starlark.None {
			var err error
			valueKey, err = starlark.Call(thread, key, starlark.Tuple{value}, nil)
			if err != nil {
				return nil, err
			}
		}
		better, err := starlark.Compare(op, valueKey, bestKey)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fn.Name(), err)
		}
		if better {
			best = value
			bestKey = valueKey
		}
	}
	return best, nil
}
