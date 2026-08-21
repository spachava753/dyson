package os

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spachava753/dyson/internal/xos"
	"github.com/spachava753/starlarkx/starlark"
)

// Environment implements environment-related os module functions using explicit host capabilities.
type Environment struct {
	env      xos.Env
	platform xos.Platform
}

var environMethods = map[string]*starlark.Builtin{
	"get": starlark.NewBuiltin(ModuleName+".environ.get", environGet),
}

// environValue provides the shared callable and attribute surface for
// os.environ. A configured environment wraps it with mappedEnvironValue.
type environValue struct {
	environment Environment
}

type mappedEnvironValue struct {
	*environValue
}

func newEnvironValue(environment Environment) starlark.Value {
	value := &environValue{environment: environment}
	if environment.env == nil {
		return value
	}
	return &mappedEnvironValue{environValue: value}
}

func (e *environValue) Name() string { return ModuleName + ".environ" }

func (e *environValue) String() string { return "<os.environ>" }

func (e *environValue) Type() string { return "os._Environ" }

// Freeze leaves the configured host capability live; environValue contains no
// mutable Starlark state of its own.
func (e *environValue) Freeze() {}

func (e *environValue) Truth() starlark.Bool { return starlark.True }

func (e *environValue) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable: %s", e.Type())
}

func (e *environValue) Attr(name string) (starlark.Value, error) {
	if method, ok := environMethods[name]; ok {
		return method.BindReceiver(e), nil
	}
	return nil, nil
}

func (e *environValue) AttrNames() []string { return []string{"get"} }

// CallInternal preserves os.environ() as an explicit dictionary snapshot.
func (e *environValue) CallInternal(_ *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(e.Name(), args, kwargs); err != nil {
		return nil, err
	}
	return e.environment.snapshot(e.Name())
}

// Get performs live indexing and membership lookups against the configured
// environment instead of an earlier enumeration snapshot.
func (e *mappedEnvironValue) Get(key starlark.Value) (starlark.Value, bool, error) {
	name, ok := key.(starlark.String)
	if !ok {
		return nil, false, fmt.Errorf("%s: key must be string, got %s", e.Name(), key.Type())
	}
	value, found := e.environment.env.LookupEnv(string(name))
	if !found {
		return nil, false, nil
	}
	return starlark.String(value), true, nil
}

// Iterate snapshots keys so one traversal remains stable if the host
// environment changes while Starlark consumes the iterator.
func (e *mappedEnvironValue) Iterate() starlark.Iterator {
	items := environmentItems(e.environment.env)
	keys := make([]starlark.Value, len(items))
	for i, item := range items {
		keys[i] = item[0]
	}
	return starlark.NewList(keys).Iterate()
}

func (e *mappedEnvironValue) Items() []starlark.Tuple {
	return environmentItems(e.environment.env)
}

func (e *mappedEnvironValue) Len() int {
	return len(environmentItems(e.environment.env))
}

func (f Environment) configuredEnv(fn string) (xos.Env, error) {
	if f.env == nil {
		return nil, fmt.Errorf("%s: environment operations are not configured", fn)
	}
	return f.env, nil
}

// getExecPath returns the configured executable search path, allowing a supplied
// environment mapping to override PATH and using Python's default path when the
// effective value is empty.
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

func (f Environment) snapshot(fn string) (starlark.Value, error) {
	fsys, err := f.configuredEnv(fn)
	if err != nil {
		return nil, err
	}
	items := environmentItems(fsys)
	dict := starlark.NewDict(len(items))
	for _, item := range items {
		if err := dict.SetKey(item[0], item[1]); err != nil {
			return nil, err
		}
	}
	return dict, nil
}

func environmentItems(env xos.Env) []starlark.Tuple {
	environ := env.Environ()
	items := make([]starlark.Tuple, 0, len(environ))
	indices := make(map[string]int, len(environ))
	for _, entry := range environ {
		key, value, _ := strings.Cut(entry, "=")
		if index, ok := indices[key]; ok {
			items[index][1] = starlark.String(value)
			continue
		}
		indices[key] = len(items)
		items = append(items, starlark.Tuple{starlark.String(key), starlark.String(value)})
	}
	return items
}

// environGet performs a fresh capability lookup so putenv and unsetenv changes
// are visible without rebuilding the module or a snapshot.
func environGet(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	environ, ok := fn.Receiver().(*environValue)
	if !ok {
		return nil, fmt.Errorf("%s: receiver is %T, want os._Environ", fn.Name(), fn.Receiver())
	}
	var key string
	var def starlark.Value = starlark.None
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "key", &key, "default?", &def); err != nil {
		return nil, err
	}
	fsys, err := environ.environment.configuredEnv(fn.Name())
	if err != nil {
		return nil, err
	}
	if value, ok := fsys.LookupEnv(key); ok {
		return starlark.String(value), nil
	}
	return def, nil
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
