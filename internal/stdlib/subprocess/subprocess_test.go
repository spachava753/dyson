package subprocess

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spachava753/dyson/internal/chunkedfile"
	"github.com/spachava753/dyson/internal/xos"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/starlarktest"
	"go.starlark.net/syntax"
)

func TestSubprocessTestdata(t *testing.T) {
	runSubprocessTestdata(t, MakeModule(fakeRunner{}), filepath.Join("testdata", "subprocess.star"))
}

func TestSubprocessUnconfiguredTestdata(t *testing.T) {
	runSubprocessTestdata(t, MakeModule(nil), filepath.Join("testdata", "unconfigured.star"))
}

func runSubprocessTestdata(t *testing.T, module *starlarkstruct.Module, filename string) {
	t.Helper()
	for i, chunk := range chunkedfile.Read(filename, t) {
		t.Run(fmt.Sprintf("chunk_%02d", i+1), func(t *testing.T) {
			thread := newTestThread(t, module)
			_, err := starlark.ExecFileOptions(&syntax.FileOptions{}, thread, filename, chunk.Source, nil)
			if err != nil {
				chunk.GotErrorAnyLine(err.Error())
			}
			chunk.Done()
		})
	}
}

func newTestThread(t *testing.T, module *starlarkstruct.Module) *starlark.Thread {
	thread := &starlark.Thread{
		Name: "test",
		Load: func(thread *starlark.Thread, name string) (starlark.StringDict, error) {
			switch name {
			case ModuleName + ".star":
				return starlark.StringDict{ModuleName: module}, nil
			case "assert.star":
				return starlarktest.LoadAssertModule()
			default:
				return nil, fmt.Errorf("unknown module %q", name)
			}
		},
	}
	starlarktest.SetReporter(thread, t)
	return thread
}

type fakeRunner struct{}

func (fakeRunner) RunCommand(_ context.Context, command xos.Command) (xos.CommandResult, error) {
	if command.Shell {
		stdout := []byte("shell:" + command.Args[0] + "\n")
		returncode := 0
		switch command.Args[0] {
		case "status output":
			returncode = 5
			stdout = []byte("status output\n")
		case "plain output":
			stdout = []byte("plain output\n")
		case "double newline":
			stdout = []byte("a\n\n")
		case `echo "$0:$1"`:
			stdout = []byte(command.Args[1] + ":" + command.Args[2] + "\n")
		}
		return streamResult(command, returncode, stdout, nil), nil
	}

	switch command.Args[0] {
	case "echo":
		return streamResult(command, 0, []byte("argv:"+strings.Join(command.Args, "|")+"\n"), nil), nil
	case "context":
		return streamResult(command, 0, []byte("cwd="+command.Dir+" env="+strings.Join(command.Env, ",")+"\n"), nil), nil
	case "input":
		return streamResult(command, 0, command.Input, nil), nil
	case "fail":
		return streamResult(command, 7, []byte("bad\n"), []byte("err\n")), nil
	default:
		return streamResult(command, 127, nil, []byte("unknown command\n")), nil
	}
}

func streamResult(command xos.Command, returncode int, stdout, stderr []byte) xos.CommandResult {
	result := xos.CommandResult{ReturnCode: returncode}
	if command.Stdout == xos.StreamPipe {
		result.Stdout = stdout
		if command.Stderr == xos.StreamStdout {
			result.Stdout = append(result.Stdout, stderr...)
		}
	}
	if command.Stderr == xos.StreamPipe {
		result.Stderr = stderr
	}
	return result
}
