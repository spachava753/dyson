package builtins

import (
	"fmt"
	"sort"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// iterableValues consumes a Starlark iterable into a Go slice. It treats
// Starlark strings as Python-like iterables of one-code-point strings because
// Go Starlark strings are indexable but not directly iterable.
func iterableValues(name string, value starlark.Value) ([]starlark.Value, error) {
	if s, ok := value.(starlark.String); ok {
		values := make([]starlark.Value, 0, len(string(s)))
		for _, r := range string(s) {
			values = append(values, starlark.String(string(r)))
		}
		return values, nil
	}
	iterable, ok := value.(starlark.Iterable)
	if !ok {
		return nil, fmt.Errorf("%s: %s object is not iterable", name, value.Type())
	}
	iter := iterable.Iterate()
	defer iter.Done()
	var values []starlark.Value
	var item starlark.Value
	for iter.Next(&item) {
		values = append(values, item)
	}
	return values, nil
}

// intValue validates that value is a Starlark int and formats any type error
// with the calling builtin's function and parameter names.
func intValue(name, arg string, value starlark.Value) (starlark.Int, error) {
	v, ok := value.(starlark.Int)
	if !ok {
		return starlark.Int{}, fmt.Errorf("%s: %s must be int, got %s", name, arg, value.Type())
	}
	return v, nil
}

// int64Value converts a Starlark int to int64 for builtins whose supported
// subset requires host-sized loop bounds or code points.
func int64Value(name, arg string, value starlark.Value) (int64, error) {
	v, err := intValue(name, arg, value)
	if err != nil {
		return 0, err
	}
	i, ok := v.Int64()
	if !ok {
		return 0, fmt.Errorf("%s: %s is out of range", name, arg)
	}
	return i, nil
}

// sortValues performs stable in-place sorting for sortedBuiltin, optionally
// comparing key(value) results and reversing the final order.
func sortValues(thread *starlark.Thread, name string, values []starlark.Value, key starlark.Value, reverse bool) error {
	type item struct {
		value starlark.Value
		key   starlark.Value
	}
	items := make([]item, len(values))
	for i, value := range values {
		items[i] = item{value: value, key: value}
		if key != starlark.None {
			keyValue, err := starlark.Call(thread, key, starlark.Tuple{value}, nil)
			if err != nil {
				return err
			}
			items[i].key = keyValue
		}
	}
	var sortErr error
	sort.SliceStable(items, func(i, j int) bool {
		if sortErr != nil {
			return false
		}
		less, err := starlark.Compare(syntax.LT, items[i].key, items[j].key)
		if err != nil {
			sortErr = fmt.Errorf("%s: %w", name, err)
			return false
		}
		if reverse {
			return !less
		}
		return less
	})
	if sortErr != nil {
		return sortErr
	}
	for i, item := range items {
		values[i] = item.value
	}
	return nil
}
