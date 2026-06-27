package builtins

import (
	"fmt"

	"go.starlark.net/starlark"
)

// rangeBuiltin implements the Starlark range builtin, returning a list of ints
// using Python's start, stop, and step semantics.
//
// TODO: Return a lazy range value instead of a list; Python's range returns an
// immutable range object.
//
// It mirrors the supported subset of Python's range:
// https://docs.python.org/3/library/functions.html#func-range
func rangeBuiltin(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(kwargs) != 0 {
		return nil, fmt.Errorf("%s: unexpected keyword argument %s", fn.Name(), kwargs[0][0])
	}
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("%s: got %d arguments, want 1 to 3", fn.Name(), len(args))
	}
	var start, stop int64
	step := int64(1)
	var err error
	if len(args) == 1 {
		stop, err = int64Value(fn.Name(), "stop", args[0])
	} else {
		start, err = int64Value(fn.Name(), "start", args[0])
		if err == nil {
			stop, err = int64Value(fn.Name(), "stop", args[1])
		}
	}
	if err != nil {
		return nil, err
	}
	if len(args) == 3 {
		step, err = int64Value(fn.Name(), "step", args[2])
		if err != nil {
			return nil, err
		}
	}
	if step == 0 {
		return nil, fmt.Errorf("%s: step argument must not be zero", fn.Name())
	}
	values := []starlark.Value{}
	if step > 0 {
		for i := start; i < stop; i += step {
			values = append(values, starlark.MakeInt64(i))
		}
	} else {
		for i := start; i > stop; i += step {
			values = append(values, starlark.MakeInt64(i))
		}
	}
	return starlark.NewList(values), nil
}

// reversedBuiltin implements the Starlark reversed builtin, returning a list of
// the input iterable's values in reverse order.
//
// TODO: Return an iterator instead of a list; Python's reversed returns a
// reverse iterator.
//
// It mirrors the supported subset of Python's reversed:
// https://docs.python.org/3/library/functions.html#reversed
func reversedBuiltin(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var value starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "sequence", &value); err != nil {
		return nil, err
	}
	values, err := iterableValues(fn.Name(), value)
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(values)-1; i < j; i, j = i+1, j-1 {
		values[i], values[j] = values[j], values[i]
	}
	return starlark.NewList(values), nil
}

// sortedBuiltin implements the Starlark sorted builtin, returning a new sorted
// list and supporting Python-compatible key and reverse parameters.
//
// It mirrors the supported subset of Python's sorted:
// https://docs.python.org/3/library/functions.html#sorted
func sortedBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var iterable starlark.Value
	key := starlark.Value(starlark.None)
	reverse := false
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "iterable", &iterable, "key?", &key, "reverse?", &reverse); err != nil {
		return nil, err
	}
	if key != starlark.None {
		if _, ok := key.(starlark.Callable); !ok {
			return nil, fmt.Errorf("%s: key must be callable or None, got %s", fn.Name(), key.Type())
		}
	}
	values, err := iterableValues(fn.Name(), iterable)
	if err != nil {
		return nil, err
	}
	if err := sortValues(thread, fn.Name(), values, key, reverse); err != nil {
		return nil, err
	}
	return starlark.NewList(values), nil
}

// enumerateBuiltin implements the Starlark enumerate builtin, returning a list
// of (index, value) tuples starting at the optional start index.
//
// TODO: Return an iterator instead of a list; Python's enumerate returns an
// enumerate object.
//
// It mirrors the supported subset of Python's enumerate:
// https://docs.python.org/3/library/functions.html#enumerate
func enumerateBuiltin(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var iterable starlark.Value
	var startValue starlark.Value = starlark.MakeInt(0)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "iterable", &iterable, "start?", &startValue); err != nil {
		return nil, err
	}
	start, err := int64Value(fn.Name(), "start", startValue)
	if err != nil {
		return nil, err
	}
	values, err := iterableValues(fn.Name(), iterable)
	if err != nil {
		return nil, err
	}
	out := make([]starlark.Value, len(values))
	for i, value := range values {
		out[i] = starlark.Tuple{starlark.MakeInt64(start + int64(i)), value}
	}
	return starlark.NewList(out), nil
}
