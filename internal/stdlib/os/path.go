package os

import (
	"fmt"
	pathpkg "path"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

func makePathModule(primitives *starlarkstruct.Module) *starlarkstruct.Module {
	members := starlark.StringDict{
		"abspath": primitives.Members["path_abspath"], "basename": pathBuiltin("basename", pathBasename),
		"dirname": pathBuiltin("dirname", pathDirname), "exists": primitives.Members["path_exists"],
		"lexists": primitives.Members["path_lexists"], "expanduser": primitives.Members["path_expanduser"],
		"expandvars": primitives.Members["path_expandvars"], "getatime": primitives.Members["path_getatime"],
		"getmtime": primitives.Members["path_getmtime"], "getctime": primitives.Members["path_getctime"],
		"getsize": primitives.Members["path_getsize"], "isabs": pathBuiltin("isabs", pathIsAbs),
		"isdir": primitives.Members["path_isdir"], "isfile": primitives.Members["path_isfile"],
		"islink": primitives.Members["path_islink"], "ismount": primitives.Members["path_ismount"],
		"join": pathBuiltin("join", pathJoin), "normpath": pathBuiltin("normpath", pathNormpath),
		"realpath": primitives.Members["path_realpath"], "relpath": pathBuiltin("relpath", pathRelpath),
		"samefile": primitives.Members["path_samefile"], "split": pathBuiltin("split", pathSplit),
		"splitdrive": pathBuiltin("splitdrive", pathSplitdrive), "splitext": pathBuiltin("splitext", pathSplitext),
		"commonpath": pathBuiltin("commonpath", pathCommonpath), "supports_unicode_filenames": starlark.True,
	}
	module := &starlarkstruct.Module{Name: ModuleName + ".path", Members: members}
	module.Freeze()
	return module
}

func pathBuiltin(name string, implementation func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error)) *starlark.Builtin {
	return starlark.NewBuiltin(ModuleName+".path."+name, implementation)
}

func unpackPath(fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (string, error) {
	var value string
	err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &value)
	return value, err
}

func pathIsAbs(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	value, err := unpackPath(fn, args, kwargs)
	return starlark.Bool(strings.HasPrefix(value, "/")), err
}

// pathJoin joins POSIX-style path components without cleaning them. A later
// absolute component discards the accumulated prefix, matching os.path.join.
func pathJoin(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) == 0 {
		var first string
		if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &first); err != nil {
			return nil, err
		}
		return starlark.String(first), nil
	}
	if len(kwargs) != 0 {
		var first string
		if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &first); err != nil {
			return nil, err
		}
	}
	parts := make([]string, len(args))
	for i, value := range args {
		part, ok := starlark.AsString(value)
		if !ok {
			return nil, fmt.Errorf("%s: got %s, want string", fn.Name(), value.Type())
		}
		parts[i] = part
	}
	joined := parts[0]
	for _, part := range parts[1:] {
		switch {
		case strings.HasPrefix(part, "/"):
			joined = part
		case joined == "" || strings.HasSuffix(joined, "/"):
			joined += part
		default:
			joined += "/" + part
		}
	}
	return starlark.String(joined), nil
}

func splitPathText(value string) (string, string) {
	index := strings.LastIndexByte(value, '/') + 1
	head, tail := value[:index], value[index:]
	if head != "" && strings.Trim(head, "/") != "" {
		head = strings.TrimRight(head, "/")
	}
	return head, tail
}

func pathSplit(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	value, err := unpackPath(fn, args, kwargs)
	if err != nil {
		return nil, err
	}
	head, tail := splitPathText(value)
	return starlark.Tuple{starlark.String(head), starlark.String(tail)}, nil
}

func pathBasename(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	value, err := unpackPath(fn, args, kwargs)
	if err != nil {
		return nil, err
	}
	_, tail := splitPathText(value)
	return starlark.String(tail), nil
}

func pathDirname(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	value, err := unpackPath(fn, args, kwargs)
	if err != nil {
		return nil, err
	}
	head, _ := splitPathText(value)
	return starlark.String(head), nil
}

func pathSplitdrive(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	value, err := unpackPath(fn, args, kwargs)
	if err != nil {
		return nil, err
	}
	return starlark.Tuple{starlark.String(""), starlark.String(value)}, nil
}

func pathSplitext(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	value, err := unpackPath(fn, args, kwargs)
	if err != nil {
		return nil, err
	}
	separator, dot := strings.LastIndexByte(value, '/'), strings.LastIndexByte(value, '.')
	if dot <= separator+1 {
		return starlark.Tuple{starlark.String(value), starlark.String("")}, nil
	}
	return starlark.Tuple{starlark.String(value[:dot]), starlark.String(value[dot:])}, nil
}

func pathNormpath(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	value, err := unpackPath(fn, args, kwargs)
	if err != nil {
		return nil, err
	}
	return starlark.String(pathpkg.Clean(value)), nil
}

func pathRelpath(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var value string
	start := "."
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &value, "start?", &start); err != nil {
		return nil, err
	}
	if pathpkg.IsAbs(value) != pathpkg.IsAbs(start) {
		return nil, fmt.Errorf("%s: path and start must both be absolute or both be relative", fn.Name())
	}
	valueParts := pathParts(pathpkg.Clean(value))
	startParts := pathParts(pathpkg.Clean(start))
	common := 0
	for common < len(valueParts) && common < len(startParts) && valueParts[common] == startParts[common] {
		common++
	}
	relative := make([]string, len(startParts)-common, len(startParts)-common+len(valueParts)-common)
	for i := range relative {
		relative[i] = ".."
	}
	relative = append(relative, valueParts[common:]...)
	if len(relative) == 0 {
		return starlark.String("."), nil
	}
	return starlark.String(strings.Join(relative, "/")), nil
}

func pathParts(value string) []string {
	value = strings.Trim(value, "/")
	if value == "" || value == "." {
		return nil
	}
	return strings.Split(value, "/")
}

// pathCommonpath returns the shared leading path components after cleaning every
// input. It rejects empty input and mixtures of absolute and relative paths.
func pathCommonpath(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var paths starlark.Iterable
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "paths", &paths); err != nil {
		return nil, err
	}
	var values []string
	iterator := paths.Iterate()
	defer iterator.Done()
	var item starlark.Value
	for iterator.Next(&item) {
		value, ok := starlark.AsString(item)
		if !ok {
			return nil, fmt.Errorf("%s: path is not a string", fn.Name())
		}
		values = append(values, value)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("%s: arg is an empty sequence", fn.Name())
	}
	components := make([][]string, len(values))
	absolute := false
	for i, value := range values {
		if i == 0 {
			absolute = pathpkg.IsAbs(value)
		} else if pathpkg.IsAbs(value) != absolute {
			return nil, fmt.Errorf("%s: can't mix absolute and relative paths", fn.Name())
		}
		cleaned := strings.Trim(pathpkg.Clean(value), "/")
		if cleaned != "" && cleaned != "." {
			components[i] = strings.Split(cleaned, "/")
		}
	}
	common := components[0]
	for _, parts := range components[1:] {
		limit := min(len(common), len(parts))
		i := 0
		for i < limit && common[i] == parts[i] {
			i++
		}
		common = common[:i]
	}
	result := strings.Join(common, "/")
	if absolute {
		result = "/" + result
	}
	return starlark.String(result), nil
}
