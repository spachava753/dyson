package a

import (
	"fmt"

	"example.com/codec"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

const registeredTypeName = "registered.Value"

var _ = codec.ValueCodec{Type: registeredTypeName}

type registeredValue struct{}

func (registeredValue) String() string        { return "registered" }
func (registeredValue) Type() string          { return registeredTypeName }
func (registeredValue) Freeze()               {}
func (registeredValue) Truth() starlark.Bool  { return starlark.True }
func (registeredValue) Hash() (uint32, error) { return 0, nil }

type customValue struct{}

func (customValue) String() string        { return "custom" }
func (customValue) Type() string          { return "custom.Value" }
func (customValue) Freeze()               {}
func (customValue) Truth() starlark.Bool  { return starlark.True }
func (customValue) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable") }

func encodableString(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlark.String("ok"), nil
}

func encodableTuple(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlark.Tuple{starlark.String("ok"), starlark.MakeInt(1)}, nil
}

func registeredCustom(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return &registeredValue{}, nil
}

func directCustom(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return &customValue{}, nil // want "starlark builtin returns"
}

func structValue(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlarkstruct.FromStringDict(starlark.String("custom.struct"), starlark.StringDict{"x": starlark.MakeInt(1)}), nil // want "starlark builtin returns"
}

func listWithCustom(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	items := make([]starlark.Value, 1)
	items[0] = &customValue{}
	return starlark.NewList(items), nil // want "starlark builtin returns list containing"
}

func listWithCustomInLoop(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	items := make([]starlark.Value, 1)
	for i := range items {
		items[i] = &customValue{}
	}
	return starlark.NewList(items), nil // want "starlark builtin returns list containing"
}

func notBuiltin() starlark.Value {
	return &customValue{}
}
