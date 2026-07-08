package shutil

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/spachava753/dyson/internal/xfs"
	"github.com/spachava753/dyson/internal/xos"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

type primitives struct {
	fsys     xfs.FS
	env      xos.Env
	terminal xos.Terminal
	platform xos.Platform
}

func makePrimitiveModule(config ModuleConfig) *starlarkstruct.Module {
	p := primitives{fsys: config.FS, env: config.Env, terminal: config.Terminal, platform: config.Platform}
	return &starlarkstruct.Module{
		Name: ModuleName + "._primitive",
		Members: starlark.StringDict{
			"copyfile":          starlark.NewBuiltin(ModuleName+".copyfile", p.copyfile),
			"disk_usage":        starlark.NewBuiltin(ModuleName+".disk_usage", p.diskUsage),
			"get_terminal_size": starlark.NewBuiltin(ModuleName+".get_terminal_size", p.getTerminalSize),
			"getenv":            starlark.NewBuiltin(ModuleName+".getenv", p.getenv),
			"split_path":        starlark.NewBuiltin(ModuleName+".split_path", p.splitPath),
		},
	}
}

func (p primitives) openfs(fn string) (xfs.OpenFS, error) {
	fsys, ok := p.fsys.(xfs.OpenFS)
	if !ok {
		return nil, fmt.Errorf("%s: filesystem does not support file descriptors", fn)
	}
	return fsys, nil
}

func (p primitives) usagefs(fn string) (xfs.UsageFS, error) {
	fsys, ok := p.fsys.(xfs.UsageFS)
	if !ok {
		return nil, fmt.Errorf("%s: filesystem does not support disk usage", fn)
	}
	return fsys, nil
}

func (p primitives) copyfile(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var src, dst string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "src", &src, "dst", &dst); err != nil {
		return nil, err
	}
	fsys, err := p.openfs(fn.Name())
	if err != nil {
		return nil, err
	}
	srcFile, err := fsys.OpenFile(src, p.platform.OpenFlags.ReadOnly, 0)
	if err != nil {
		return nil, err
	}
	defer srcFile.Close()
	dstFile, err := fsys.OpenFile(dst, p.platform.OpenFlags.WriteOnly|p.platform.OpenFlags.Create|p.platform.OpenFlags.Truncate, fs.FileMode(0o666))
	if err != nil {
		return nil, err
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return nil, err
	}
	return starlark.String(filepath.ToSlash(dst)), nil
}

func (p primitives) diskUsage(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	fsys, err := p.usagefs(fn.Name())
	if err != nil {
		return nil, err
	}
	usage, err := fsys.DiskUsage(path)
	if err != nil {
		return nil, err
	}
	return starlark.Tuple{starlark.MakeUint64(usage.Total), starlark.MakeUint64(usage.Used), starlark.MakeUint64(usage.Free)}, nil
}

func (p primitives) getTerminalSize(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	if p.terminal == nil {
		return nil, fmt.Errorf("%s: terminal operations are not configured", fn.Name())
	}
	columns, lines, err := p.terminal.TerminalSize()
	if err != nil {
		return nil, err
	}
	return starlark.Tuple{starlark.MakeInt(columns), starlark.MakeInt(lines)}, nil
}

func (p primitives) getenv(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var key string
	var def starlark.Value = starlark.None
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "key", &key, "default?", &def); err != nil {
		return nil, err
	}
	if p.env == nil {
		return def, nil
	}
	if value, ok := p.env.LookupEnv(key); ok {
		return starlark.String(value), nil
	}
	return def, nil
}

func (p primitives) splitPath(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var value string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "value", &value); err != nil {
		return nil, err
	}
	if value == "" {
		return starlark.NewList(nil), nil
	}
	parts := filepath.SplitList(value)
	items := make([]starlark.Value, len(parts))
	for i, part := range parts {
		if part == "" {
			part = "."
		}
		items[i] = starlark.String(filepath.ToSlash(part))
	}
	return starlark.NewList(items), nil
}

func containsSlash(path string) bool {
	return strings.Contains(path, "/") || strings.Contains(path, "\\")
}
