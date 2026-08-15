// Package json provides a small Python-compatible JSON module for Starlark.
package json

import (
	"fmt"
	"strings"

	starlarkjson "github.com/spachava753/starlarkx/lib/json"
	"github.com/spachava753/starlarkx/starlark"
	"github.com/spachava753/starlarkx/starlarkstruct"
)

// ModuleName is the Starlark stdlib module name for Dyson's JSON compatibility module.
const ModuleName = "json"

// Module is the Starlark namespace exposed by load("json.star", "json").
var Module = func() *starlarkstruct.Module {
	module := &starlarkstruct.Module{
		Name: ModuleName,
		Members: starlark.StringDict{
			"dumps": starlark.NewBuiltin(ModuleName+".dumps", dumps),
			"load":  starlark.NewBuiltin(ModuleName+".load", load),
		},
	}
	module.Freeze()
	return module
}()

func dumps(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var object starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "obj", &object); err != nil {
		return nil, err
	}

	encoded, err := starlark.Call(thread, starlarkjson.Module.Members["encode"], starlark.Tuple{object}, nil)
	if err != nil {
		return nil, codecError(fn.Name(), "json.encode", err)
	}
	return encoded, nil
}

func load(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var file starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "fp", &file); err != nil {
		return nil, err
	}

	attributes, ok := file.(starlark.HasAttrs)
	if !ok {
		return nil, fmt.Errorf("%s: %s has no read method", fn.Name(), file.Type())
	}
	read, err := attributes.Attr("read")
	if err != nil {
		return nil, fmt.Errorf("%s: accessing fp.read: %w", fn.Name(), err)
	}
	if read == nil {
		return nil, fmt.Errorf("%s: %s has no read method", fn.Name(), file.Type())
	}
	callable, ok := read.(starlark.Callable)
	if !ok {
		return nil, fmt.Errorf("%s: fp.read is %s, want callable", fn.Name(), read.Type())
	}

	content, err := starlark.Call(thread, callable, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: fp.read(): %w", fn.Name(), err)
	}
	var text string
	switch content := content.(type) {
	case starlark.String:
		text = string(content)
	case starlark.Bytes:
		text = string(content)
	default:
		return nil, fmt.Errorf("%s: fp.read() returned %s, want string or bytes", fn.Name(), content.Type())
	}

	decoded, err := starlark.Call(thread, starlarkjson.Module.Members["decode"], starlark.Tuple{starlark.String(text)}, nil)
	if err != nil {
		return nil, codecError(fn.Name(), "json.decode", err)
	}
	return decoded, nil
}

func codecError(name, codecName string, err error) error {
	message := err.Error()
	if rest, ok := strings.CutPrefix(message, codecName+": "); ok {
		message = rest
	}
	return fmt.Errorf("%s: %s", name, message)
}
