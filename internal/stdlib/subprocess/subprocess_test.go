package subprocess

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson/internal/chunkedfile"
	"github.com/spachava753/dyson/internal/xctx"
	"github.com/spachava753/dyson/internal/xos"
	"github.com/spachava753/starlarkx/starlark"
	"github.com/spachava753/starlarkx/starlarkstruct"
	"github.com/spachava753/starlarkx/starlarktest"
	"github.com/spachava753/starlarkx/syntax"
)

func TestSubprocessTestdata(t *testing.T) {
	runSubprocessTestdata(t, MakeModule(fakeRunner{}), filepath.Join("testdata", "subprocess.star"))
}

func TestSubprocessUnconfiguredTestdata(t *testing.T) {
	runSubprocessTestdata(t, MakeModule(nil), filepath.Join("testdata", "unconfigured.star"))
}

func TestParseTimeout(t *testing.T) {
	hugeNegative := starlark.MakeBigInt(new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 4096)))
	for _, test := range []struct {
		name     string
		value    starlark.Value
		duration time.Duration
		set      bool
	}{
		{name: "none", value: starlark.None},
		{name: "integer", value: starlark.MakeInt(2), duration: 2 * time.Second, set: true},
		{name: "float", value: starlark.Float(0.25), duration: 250 * time.Millisecond, set: true},
		{name: "zero", value: starlark.MakeInt(0), set: true},
		{name: "negative", value: starlark.MakeInt(-1), set: true},
		{name: "arbitrary precision negative", value: hugeNegative, set: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			duration, set, err := parseTimeout(test.value)
			be.Err(t, err, nil)
			be.Equal(t, duration, test.duration)
			be.Equal(t, set, test.set)
		})
	}

	for _, value := range []starlark.Value{
		starlark.String("1"),
		starlark.Float(math.NaN()),
		starlark.Float(math.Inf(1)),
		starlark.MakeBigInt(new(big.Int).Lsh(big.NewInt(1), 4096)),
		starlark.Float(1e20),
	} {
		_, _, err := parseTimeout(value)
		if err == nil {
			t.Fatalf("parseTimeout(%s) succeeded", value)
		}
	}
}

type timeoutRunner struct {
	deadline time.Time
	cause    error
	result   xos.CommandResult
	err      error
}

func (r *timeoutRunner) RunCommand(ctx context.Context, _ xos.Command) (xos.CommandResult, error) {
	r.deadline, _ = ctx.Deadline()
	<-ctx.Done()
	r.cause = context.Cause(ctx)
	if r.err != nil {
		return r.result, r.err
	}
	return r.result, ctx.Err()
}

func TestRunTimeoutCancelsCommand(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		runner := &timeoutRunner{}
		module := MakeModule(runner)
		start := time.Now()

		_, err := starlark.Call(
			newTestThread(t, module),
			module.Members["run"],
			starlark.Tuple{starlark.NewList([]starlark.Value{starlark.String("block")})},
			[]starlark.Tuple{{starlark.String("timeout"), starlark.Float(0.25)}},
		)
		if err == nil || !strings.Contains(err.Error(), "subprocess.run: command timed out after 0.25 seconds") {
			t.Fatalf("subprocess.run error = %v, want timeout", err)
		}
		be.Equal(t, time.Since(start), 250*time.Millisecond)
		be.Equal(t, runner.deadline.Sub(start), 250*time.Millisecond)
		be.Equal(t, errors.Is(runner.cause, errCommandTimeout), true)
	})
}

func TestRunTimeoutUsesContextCauseForRunnerError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		runnerErr := errors.New("process terminated")
		runner := &timeoutRunner{err: runnerErr}
		module := MakeModule(runner)

		_, err := starlark.Call(
			newTestThread(t, module),
			module.Members["run"],
			starlark.Tuple{starlark.NewList([]starlark.Value{starlark.String("block")})},
			[]starlark.Tuple{{starlark.String("timeout"), starlark.Float(0.25)}},
		)
		if err == nil || !strings.Contains(err.Error(), "subprocess.run: command timed out after 0.25 seconds") {
			t.Fatalf("subprocess.run error = %v, want timeout", err)
		}
		be.Equal(t, errors.Is(err, runnerErr), false)
		be.Equal(t, errors.Is(runner.cause, errCommandTimeout), true)
	})
}

func TestRunTimeoutPreservesPartialOutput(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		args := starlark.NewList([]starlark.Value{starlark.String("block")})
		runner := &timeoutRunner{result: xos.CommandResult{
			Stdout: []byte("partial stdout"),
			Stderr: []byte("partial stderr"),
		}}
		module := MakeModule(runner)

		_, err := starlark.Call(
			newTestThread(t, module),
			module.Members["run"],
			starlark.Tuple{args},
			[]starlark.Tuple{
				{starlark.String("capture_output"), starlark.True},
				{starlark.String("text"), starlark.True},
				{starlark.String("timeout"), starlark.Float(0.25)},
			},
		)
		timeoutErr, ok := errors.AsType[*TimeoutExpiredError](err)
		if !ok {
			t.Fatalf("subprocess.run error = %T %v, want *TimeoutExpiredError", err, err)
		}
		if timeoutErr.Cmd != args {
			t.Fatalf("timeout command = %v, want original args", timeoutErr.Cmd)
		}
		be.Equal(t, timeoutErr.Timeout.String(), "0.25")
		be.Equal(t, string(timeoutErr.Stdout), "partial stdout")
		be.Equal(t, string(timeoutErr.Stderr), "partial stderr")
	})
}

func TestRunTimeoutPreservesParentCancellation(t *testing.T) {
	runner := &timeoutRunner{}
	module := MakeModule(runner)
	parentCause := errors.New("parent canceled")
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(parentCause)
	thread := newTestThread(t, module)
	xctx.WithContext(thread, ctx)

	_, err := starlark.Call(
		thread,
		module.Members["run"],
		starlark.Tuple{starlark.NewList([]starlark.Value{starlark.String("block")})},
		[]starlark.Tuple{{starlark.String("timeout"), starlark.MakeInt(10)}},
	)
	if err == nil {
		t.Fatal("subprocess.run succeeded after parent cancellation")
	}
	be.Equal(t, errors.Is(err, context.Canceled), true)
	be.Equal(t, strings.Contains(err.Error(), "timed out"), false)
	be.Equal(t, errors.Is(runner.cause, parentCause), true)
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

// RunCommand emulates shell and argv commands used by subprocess compatibility tests.
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
