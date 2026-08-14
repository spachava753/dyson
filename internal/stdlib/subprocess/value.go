package subprocess

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spachava753/starlarkx/starlark"
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

// String returns the Python-style CompletedProcess representation.
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

// Type returns the Starlark type name for completed processes.
func (p *completedProcessValue) Type() string { return completedProcessTypeName }

// Freeze recursively freezes values retained by the completed process.
func (p *completedProcessValue) Freeze() {
	p.args.Freeze()
	p.stdout.Freeze()
	p.stderr.Freeze()
}

// Truth reports completed process values as true.
func (p *completedProcessValue) Truth() starlark.Bool { return starlark.True }

// Hash reports that completed process values are not hashable.
func (p *completedProcessValue) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable: %s", p.Type())
}

// Attr returns a completed process field or bound method, or nil for an unknown attribute.
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

// AttrNames returns the fields and methods exposed by a completed process.
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
