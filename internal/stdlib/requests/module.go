package requests

import (
	"fmt"

	"github.com/spachava753/dyson/internal/xhttp"
	"github.com/spachava753/starlarkx/starlark"
	"github.com/spachava753/starlarkx/starlarkstruct"
)

// ModuleName is the Starlark stdlib module name for Dyson's requests compatibility module.
const ModuleName = "requests"

// Module is the default fail-closed namespace exposed by load("requests.star", "requests").
var Module = MakeModule(nil)

// MakeModule returns a requests module backed by client. A nil client keeps the
// module loadable but denies HTTP operations.
func MakeModule(client xhttp.Client) *starlarkstruct.Module {
	request := starlark.NewBuiltin(ModuleName+".request", requestBuiltin(client))
	module := &starlarkstruct.Module{
		Name: ModuleName,
		Members: starlark.StringDict{
			"request": request,
			"get":     newRequestHelper(request, "get", parameter("url"), optionalParameter("params")),
			"options": newRequestHelper(request, "options", parameter("url")),
			"head":    newHeadHelper(request),
			"post":    newRequestHelper(request, "post", parameter("url"), optionalParameter("data"), optionalParameter("json")),
			"put":     newRequestHelper(request, "put", parameter("url"), optionalParameter("data")),
			"patch":   newRequestHelper(request, "patch", parameter("url"), optionalParameter("data")),
			"delete":  newRequestHelper(request, "delete", parameter("url")),
		},
	}
	module.Freeze()
	return module
}

type helperParameter struct {
	name         string
	defaultValue starlark.Value
}

func parameter(name string) helperParameter {
	return helperParameter{name: name}
}

func optionalParameter(name string) helperParameter {
	return helperParameter{name: name, defaultValue: starlark.None}
}

func newHeadHelper(request *starlark.Builtin) *starlark.Builtin {
	helper := newRequestHelper(request, "head", parameter("url"))
	return starlark.NewBuiltin(ModuleName+".head", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if !hasKeyword(kwargs, "allow_redirects") {
			kwargs = append(kwargs, starlark.Tuple{starlark.String("allow_redirects"), starlark.Bool(false)})
		}
		return helper.CallInternal(thread, args, kwargs)
	})
}

func newRequestHelper(request *starlark.Builtin, method string, parameters ...helperParameter) *starlark.Builtin {
	name := ModuleName + "." + method
	return starlark.NewBuiltin(name, func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		values, forwarded, err := bindHelperArgs(fn.Name(), parameters, args, kwargs)
		if err != nil {
			return nil, err
		}

		requestArgs := starlark.Tuple{starlark.String(method), values[0]}
		requestKwargs := make([]starlark.Tuple, 0, len(parameters)-1+len(forwarded))
		for i := 1; i < len(parameters); i++ {
			requestKwargs = append(requestKwargs, starlark.Tuple{starlark.String(parameters[i].name), values[i]})
		}
		requestKwargs = append(requestKwargs, forwarded...)
		return request.CallInternal(thread, requestArgs, requestKwargs)
	})
}

// bindHelperArgs binds the method helper's declared positional and keyword
// parameters, applies defaults, and forwards unknown keywords to request.
func bindHelperArgs(fn string, parameters []helperParameter, args starlark.Tuple, kwargs []starlark.Tuple) ([]starlark.Value, []starlark.Tuple, error) {
	if len(args) > len(parameters) {
		argument := "arguments"
		if len(parameters) == 1 {
			argument = "argument"
		}
		return nil, nil, fmt.Errorf("%s: accepts %d positional %s (%d given)", fn, len(parameters), argument, len(args))
	}

	values := make([]starlark.Value, len(parameters))
	defined := make([]bool, len(parameters))
	for i, value := range args {
		values[i] = value
		defined[i] = true
	}

	forwarded := make([]starlark.Tuple, 0, len(kwargs))
	for _, kwarg := range kwargs {
		name := string(kwarg[0].(starlark.String))
		index := -1
		for i, parameter := range parameters {
			if parameter.name == name {
				index = i
				break
			}
		}
		if index < 0 {
			forwarded = append(forwarded, kwarg)
			continue
		}
		if defined[index] {
			return nil, nil, fmt.Errorf("%s: got multiple values for keyword argument %s", fn, name)
		}
		values[index] = kwarg[1]
		defined[index] = true
	}

	for i, parameter := range parameters {
		if defined[i] {
			continue
		}
		if parameter.defaultValue == nil {
			return nil, nil, fmt.Errorf("%s: missing argument for %s", fn, parameter.name)
		}
		values[i] = parameter.defaultValue
	}
	return values, forwarded, nil
}

func hasKeyword(kwargs []starlark.Tuple, name string) bool {
	for _, kwarg := range kwargs {
		if string(kwarg[0].(starlark.String)) == name {
			return true
		}
	}
	return false
}
