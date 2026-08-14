package os

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spachava753/dyson/internal/xctx"
	"github.com/spachava753/dyson/internal/xos"
	"github.com/spachava753/starlarkx/starlark"
)

// Process implements process-related os module functions using explicit host capabilities.
type Process struct {
	process       xos.Process
	commandRunner xos.CommandRunner
}

// WorkingDirectory implements working-directory os module functions using an explicit host capability.
type WorkingDirectory struct {
	dir xos.WorkingDir
}

func (f WorkingDirectory) workingDir(fn string) (xos.WorkingDir, error) {
	if f.dir == nil {
		return nil, fmt.Errorf("%s: working directory operations are not configured", fn)
	}
	return f.dir, nil
}

func (f Process) configuredProcess(fn string) (xos.Process, error) {
	if f.process == nil {
		return nil, fmt.Errorf("%s: process operations are not configured", fn)
	}
	return f.process, nil
}

func (f WorkingDirectory) getcwd(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if _, err := none(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	fsys, err := f.workingDir(fn.Name())
	if err != nil {
		return nil, err
	}
	cwd, err := fsys.Getwd()
	if err != nil {
		return nil, err
	}
	return starlark.String(filepath.ToSlash(cwd)), nil
}

func (f WorkingDirectory) chdir(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	fsys, err := f.workingDir(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.None, fsys.Chdir(path)
}

func (f Process) getpid(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if _, err := none(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	fsys, err := f.configuredProcess(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.MakeInt(fsys.Getpid()), nil
}

func (f Process) getppid(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if _, err := none(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	fsys, err := f.configuredProcess(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.MakeInt(fsys.Getppid()), nil
}

func (f Process) kill(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	pidVal, sigVal := starlark.MakeInt(0), starlark.MakeInt(0)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "pid", &pidVal, "sig", &sigVal); err != nil {
		return nil, err
	}
	pid, err := intArg(fn.Name(), "pid", pidVal)
	if err != nil {
		return nil, err
	}
	sig, err := intArg(fn.Name(), "sig", sigVal)
	if err != nil {
		return nil, err
	}
	fsys, err := f.configuredProcess(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.None, fsys.Kill(pid, sig)
}

func (f Process) system(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var command string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "command", &command); err != nil {
		return nil, err
	}
	if f.commandRunner == nil {
		return nil, fmt.Errorf("%s: subprocess execution is not configured", fn.Name())
	}
	ctx := xctx.FromLocal(thread)
	if ctx == nil {
		ctx = context.Background()
	}
	result, err := f.commandRunner.RunCommand(ctx, xos.Command{Args: []string{command}, Shell: true})
	if err != nil {
		return nil, err
	}
	return starlark.MakeInt(result.ReturnCode), nil
}

func (f Process) getuid(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if _, err := none(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	fsys, err := f.configuredProcess(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.MakeInt(fsys.Getuid()), nil
}

func (f Process) geteuid(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if _, err := none(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	fsys, err := f.configuredProcess(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.MakeInt(fsys.Geteuid()), nil
}

func (f Process) getgid(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if _, err := none(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	fsys, err := f.configuredProcess(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.MakeInt(fsys.Getgid()), nil
}

func (f Process) getegid(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if _, err := none(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	fsys, err := f.configuredProcess(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.MakeInt(fsys.Getegid()), nil
}

func (f Process) getgroups(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if _, err := none(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	fsys, err := f.configuredProcess(fn.Name())
	if err != nil {
		return nil, err
	}
	groups, err := fsys.Getgroups()
	if err != nil {
		return nil, err
	}
	items := make([]starlark.Value, len(groups))
	for i, group := range groups {
		items[i] = starlark.MakeInt(group)
	}
	return starlark.NewList(items), nil
}

func (f Process) umask(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	maskVal := starlark.MakeInt(0)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "mask", &maskVal); err != nil {
		return nil, err
	}
	mask, err := intArg(fn.Name(), "mask", maskVal)
	if err != nil {
		return nil, err
	}
	fsys, err := f.configuredProcess(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.MakeInt(fsys.Umask(mask)), nil
}
