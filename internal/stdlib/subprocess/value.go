package subprocess

import (
	"fmt"
	"sort"
	"strings"

	"go.starlark.net/starlark"
)

const completedProcessTypeName = "subprocess.CompletedProcess"

var completedProcessMethods = map[string]*starlark.Builtin{
	"check_returncode": starlark.NewBuiltin("subprocess.CompletedProcess.check_returncode", completedProcessCheckReturncode),
}

type completedProcessValue struct {
	args       starlark.Value
	returncode int
	stdout     starlark.Value
	stderr     starlark.Value
}

func (p *completedProcessValue) String() string {
	parts := []string{fmt.Sprintf("args=%s", p.args), fmt.Sprintf("returncode=%d", p.returncode)}
	if p.stdout != starlark.None {
		parts = append(parts, fmt.Sprintf("stdout=%s", p.stdout))
	}
	if p.stderr != starlark.None {
		parts = append(parts, fmt.Sprintf("stderr=%s", p.stderr))
	}
	return "CompletedProcess(" + strings.Join(parts, ", ") + ")"
}

func (p *completedProcessValue) Type() string { return completedProcessTypeName }

func (p *completedProcessValue) Freeze() {
	p.args.Freeze()
	p.stdout.Freeze()
	p.stderr.Freeze()
}

func (p *completedProcessValue) Truth() starlark.Bool { return starlark.True }

func (p *completedProcessValue) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable: %s", p.Type())
}

func (p *completedProcessValue) Attr(name string) (starlark.Value, error) {
	switch name {
	case "args":
		return p.args, nil
	case "returncode":
		return starlark.MakeInt(p.returncode), nil
	case "stdout":
		return p.stdout, nil
	case "stderr":
		return p.stderr, nil
	}
	if method, ok := completedProcessMethods[name]; ok {
		return method.BindReceiver(p), nil
	}
	return nil, nil
}

func (p *completedProcessValue) AttrNames() []string {
	names := []string{"args", "returncode", "stderr", "stdout"}
	for name := range completedProcessMethods {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func completedProcessCheckReturncode(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	p, ok := fn.Receiver().(*completedProcessValue)
	if !ok {
		return nil, fmt.Errorf("%s: receiver is %T, want %s", fn.Name(), fn.Receiver(), completedProcessTypeName)
	}
	if p.returncode != 0 {
		return nil, fmt.Errorf("subprocess.CompletedProcess.check_returncode: command exited with status %d", p.returncode)
	}
	return starlark.None, nil
}
