package os

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spachava753/dyson/internal/xos"
	"go.starlark.net/starlark"
)

// Environment implements environment-related os module functions using explicit host capabilities.
type Environment struct {
	env      xos.Env
	platform xos.Platform
}

func (f Environment) configuredEnv(fn string) (xos.Env, error) {
	if f.env == nil {
		return nil, fmt.Errorf("%s: environment operations are not configured", fn)
	}
	return f.env, nil
}

func (f Environment) getExecPath(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var env starlark.Value = starlark.None
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "env?", &env); err != nil {
		return nil, err
	}
	pathList := ""
	if f.env != nil {
		pathList, _ = f.env.LookupEnv("PATH")
	}
	if env != starlark.None {
		if dict, ok := env.(*starlark.Dict); ok {
			value, found, err := dict.Get(starlark.String("PATH"))
			if err != nil {
				return nil, err
			}
			if found {
				if s, ok := starlark.AsString(value); ok {
					pathList = s
				}
			}
		}
	}
	if pathList == "" {
		pathList = "/bin:/usr/bin"
	}
	parts := strings.Split(pathList, f.platform.PathListSeparator)
	items := make([]starlark.Value, len(parts))
	for i, part := range parts {
		items[i] = starlark.String(part)
	}
	return starlark.NewList(items), nil
}

func (f Environment) environ(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if _, err := none(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	fsys, err := f.configuredEnv(fn.Name())
	if err != nil {
		return nil, err
	}
	environ := fsys.Environ()
	dict := starlark.NewDict(len(environ))
	for _, kv := range environ {
		key, value, _ := strings.Cut(kv, "=")
		if err := dict.SetKey(starlark.String(key), starlark.String(value)); err != nil {
			return nil, err
		}
	}
	return dict, nil
}

func (f Environment) getenv(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var key string
	var def starlark.Value = starlark.None
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "key", &key, "default?", &def); err != nil {
		return nil, err
	}
	fsys, err := f.configuredEnv(fn.Name())
	if err != nil {
		return nil, err
	}
	if value, ok := fsys.LookupEnv(key); ok {
		return starlark.String(value), nil
	}
	return def, nil
}

func (f Environment) putenv(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var key, value string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "key", &key, "value", &value); err != nil {
		return nil, err
	}
	fsys, err := f.configuredEnv(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.None, fsys.Setenv(key, value)
}

func (f Environment) unsetenv(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var key string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "key", &key); err != nil {
		return nil, err
	}
	fsys, err := f.configuredEnv(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.None, fsys.Unsetenv(key)
}

func (f Environment) pathExpanduser(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		if f.env != nil {
			if home, err := f.env.UserHomeDir(); err == nil {
				return starlark.String(filepath.ToSlash(home) + strings.TrimPrefix(path, "~")), nil
			}
		}
	}
	return starlark.String(path), nil
}

func (f Environment) pathExpandvars(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	if f.env != nil {
		return starlark.String(f.env.ExpandEnv(path)), nil
	}
	return starlark.String(path), nil
}
