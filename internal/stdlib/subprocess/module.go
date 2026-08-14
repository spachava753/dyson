package subprocess

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/spachava753/dyson/internal/xctx"
	"github.com/spachava753/dyson/internal/xos"
	"github.com/spachava753/starlarkx/starlark"
	"github.com/spachava753/starlarkx/starlarkstruct"
)

// ModuleName is the Starlark stdlib module name for Dyson's subprocess compatibility module.
const ModuleName = "subprocess"

const (
	pipeValue    = -1
	stdoutValue  = -2
	devnullValue = -3
)

var errCommandTimeout = errors.New("subprocess command timeout")

// Module is the default fail-closed Starlark module namespace exposed by
// load("subprocess.star", "subprocess"). Use MakeModule with an explicit
// runner to allow command execution.
var Module = MakeModule(nil)

// MakeModule returns a subprocess module backed by runner. A nil runner keeps
// the namespace loadable but fails closed when command execution is requested.
func MakeModule(runner xos.CommandRunner) *starlarkstruct.Module {
	m := &starlarkstruct.Module{
		Name: ModuleName,
		Members: starlark.StringDict{
			"run":              starlark.NewBuiltin(ModuleName+".run", runBuiltin(runner)),
			"getoutput":        starlark.NewBuiltin(ModuleName+".getoutput", getoutputBuiltin(runner)),
			"getstatusoutput":  starlark.NewBuiltin(ModuleName+".getstatusoutput", getstatusoutputBuiltin(runner)),
			"CompletedProcess": starlark.NewBuiltin(ModuleName+".CompletedProcess", completedProcessBuiltin),
			"PIPE":             starlark.MakeInt(pipeValue),
			"STDOUT":           starlark.MakeInt(stdoutValue),
			"DEVNULL":          starlark.MakeInt(devnullValue),
		},
	}
	m.Freeze()
	return m
}

// runBuiltin returns subprocess.run backed by runner. It validates the supported
// Python arguments, derives an optional timeout from the active context, resolves
// stream and text modes, and shapes captured output into a CompletedProcess value.
func runBuiltin(runner xos.CommandRunner) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var argv starlark.Value
		var stdin starlark.Value
		var input starlark.Value = starlark.None
		var stdout starlark.Value = starlark.None
		var stderr starlark.Value = starlark.None
		captureOutput := false
		shell := false
		var cwd starlark.Value = starlark.None
		var timeout starlark.Value = starlark.None
		check := false
		text := false
		var encoding starlark.Value = starlark.None
		var env starlark.Value = starlark.None
		if err := starlark.UnpackArgs(
			fn.Name(), args, kwargs,
			"args", &argv,
			"stdin?", &stdin,
			"input?", &input,
			"stdout?", &stdout,
			"stderr?", &stderr,
			"capture_output?", &captureOutput,
			"shell?", &shell,
			"cwd?", &cwd,
			"timeout?", &timeout,
			"check?", &check,
			"text?", &text,
			"encoding?", &encoding,
			"env?", &env,
		); err != nil {
			return nil, err
		}
		timeoutDuration, hasTimeout, err := parseTimeout(timeout)
		if err != nil {
			return nil, fmt.Errorf("%s: timeout: %w", fn.Name(), err)
		}
		if encoding != starlark.None {
			text = true
		}
		if captureOutput && (stdout != starlark.None || stderr != starlark.None) {
			return nil, fmt.Errorf("%s: stdout and stderr arguments may not be used with capture_output", fn.Name())
		}
		if input != starlark.None && stdin != nil {
			return nil, fmt.Errorf("%s: stdin and input may not both be used", fn.Name())
		}

		command, err := commandArgs(fn.Name(), argv, shell)
		if err != nil {
			return nil, err
		}
		inputBytes, err := inputBytes(fn.Name(), input, text)
		if err != nil {
			return nil, err
		}
		environ, err := envList(fn.Name(), env)
		if err != nil {
			return nil, err
		}

		stdinMode, err := stdinMode(fn.Name(), stdin)
		if err != nil {
			return nil, err
		}
		stdoutMode, err := outputMode(fn.Name(), "stdout", stdout, false)
		if err != nil {
			return nil, err
		}
		stderrMode, err := outputMode(fn.Name(), "stderr", stderr, true)
		if err != nil {
			return nil, err
		}
		if captureOutput {
			stdoutMode = xos.StreamPipe
			stderrMode = xos.StreamPipe
		}
		if inputBytes != nil {
			stdinMode = xos.StreamPipe
		}

		cwdPath, err := optionalString(fn.Name(), "cwd", cwd)
		if err != nil {
			return nil, err
		}
		ctx := threadContext(thread)
		if hasTimeout {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeoutCause(ctx, timeoutDuration, errCommandTimeout)
			defer cancel()
		}
		result, err := runCommand(ctx, fn.Name(), runner, xos.Command{
			Args:   command,
			Shell:  shell,
			Input:  inputBytes,
			Env:    environ,
			Dir:    cwdPath,
			Stdin:  stdinMode,
			Stdout: stdoutMode,
			Stderr: stderrMode,
		})
		timeoutExpired := hasTimeout && errors.Is(context.Cause(ctx), errCommandTimeout)
		if timeoutExpired {
			return nil, newTimeoutExpiredError(
				argv,
				timeout,
				result.Stdout,
				result.Stderr,
				stdoutMode == xos.StreamPipe,
				stderrMode == xos.StreamPipe,
			)
		}
		if err != nil {
			return nil, err
		}
		stdoutValue := streamValue(result.Stdout, stdoutMode == xos.StreamPipe, text)
		stderrValue := streamValue(result.Stderr, stderrMode == xos.StreamPipe, text)
		completed := &completedProcessValue{args: argv, returncode: result.ReturnCode, stdout: stdoutValue, stderr: stderrValue}
		if check && result.ReturnCode != 0 {
			return nil, fmt.Errorf("%s: command exited with status %d", fn.Name(), result.ReturnCode)
		}
		return completed, nil
	}
}

func getoutputBuiltin(runner xos.CommandRunner) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var command string
		if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "cmd", &command); err != nil {
			return nil, err
		}
		statusOutput, err := runShellText(threadContext(thread), fn.Name(), runner, command)
		if err != nil {
			return nil, err
		}
		return starlark.String(statusOutput.output), nil
	}
}

func getstatusoutputBuiltin(runner xos.CommandRunner) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var command string
		if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "cmd", &command); err != nil {
			return nil, err
		}
		statusOutput, err := runShellText(threadContext(thread), fn.Name(), runner, command)
		if err != nil {
			return nil, err
		}
		return starlark.Tuple{starlark.MakeInt(statusOutput.status), starlark.String(statusOutput.output)}, nil
	}
}

func completedProcessBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var argv starlark.Value
	returncodeVal := starlark.MakeInt(0)
	var stdout starlark.Value = starlark.None
	var stderr starlark.Value = starlark.None
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "args", &argv, "returncode", &returncodeVal, "stdout?", &stdout, "stderr?", &stderr); err != nil {
		return nil, err
	}
	returncode, ok := returncodeVal.Int64()
	if !ok || int64(int(returncode)) != returncode {
		return nil, fmt.Errorf("%s: returncode is out of range", fn.Name())
	}
	return &completedProcessValue{args: argv, returncode: int(returncode), stdout: stdout, stderr: stderr}, nil
}

type statusOutput struct {
	status int
	output string
}

func runShellText(ctx context.Context, fn string, runner xos.CommandRunner, command string) (statusOutput, error) {
	result, err := runCommand(ctx, fn, runner, xos.Command{Args: []string{command}, Shell: true, Stdout: xos.StreamPipe, Stderr: xos.StreamStdout})
	if err != nil {
		return statusOutput{}, err
	}
	output := string(result.Stdout)
	output, _ = strings.CutSuffix(output, "\n")
	return statusOutput{status: result.ReturnCode, output: output}, nil
}

func runCommand(ctx context.Context, fn string, runner xos.CommandRunner, command xos.Command) (xos.CommandResult, error) {
	if runner == nil {
		return xos.CommandResult{}, fmt.Errorf("%s: subprocess execution is not configured", fn)
	}
	return runner.RunCommand(ctx, command)
}

func threadContext(thread *starlark.Thread) context.Context {
	if ctx := xctx.FromLocal(thread); ctx != nil {
		return ctx
	}
	return context.Background()
}

func parseTimeout(value starlark.Value) (time.Duration, bool, error) {
	if value == starlark.None {
		return 0, false, nil
	}
	if integer, ok := value.(starlark.Int); ok {
		if integer.Sign() <= 0 {
			return 0, true, nil
		}
		seconds, ok := integer.Int64()
		maxSeconds := int64((time.Duration(1<<63 - 1)) / time.Second)
		if !ok || seconds > maxSeconds {
			return 0, false, fmt.Errorf("must fit in a time.Duration")
		}
		return time.Duration(seconds) * time.Second, true, nil
	}
	float, ok := value.(starlark.Float)
	if !ok {
		return 0, false, fmt.Errorf("must be a finite number of seconds or None")
	}
	seconds := float64(float)
	if math.IsInf(seconds, 0) || math.IsNaN(seconds) {
		return 0, false, fmt.Errorf("must be a finite number of seconds or None")
	}
	if seconds <= 0 {
		return 0, true, nil
	}
	nanoseconds := seconds * float64(time.Second)
	if nanoseconds >= float64(uint64(1)<<63) {
		return 0, false, fmt.Errorf("must fit in a time.Duration")
	}
	return time.Duration(nanoseconds), true, nil
}

func commandArgs(fn string, val starlark.Value, shell bool) ([]string, error) {
	if s, ok := starlark.AsString(val); ok {
		return []string{s}, nil
	}
	indexable, ok := val.(starlark.Indexable)
	if !ok {
		return nil, fmt.Errorf("%s: args must be a string or sequence of strings", fn)
	}
	if indexable.Len() == 0 {
		return nil, fmt.Errorf("%s: args sequence must not be empty", fn)
	}
	args := make([]string, indexable.Len())
	for i := range indexable.Len() {
		item := indexable.Index(i)
		s, ok := starlark.AsString(item)
		if !ok {
			return nil, fmt.Errorf("%s: args item %d must be a string", fn, i)
		}
		args[i] = s
	}
	if shell {
		return args, nil
	}
	return args, nil
}

func stdinMode(fn string, val starlark.Value) (xos.StreamMode, error) {
	if val == nil || val == starlark.None {
		return xos.StreamInherit, nil
	}
	if isSpecial(val, pipeValue) {
		return xos.StreamPipe, nil
	}
	if isSpecial(val, devnullValue) {
		return xos.StreamDiscard, nil
	}
	return xos.StreamInherit, fmt.Errorf("%s: stdin must be PIPE, DEVNULL, or None", fn)
}

func outputMode(fn, name string, val starlark.Value, allowStdout bool) (xos.StreamMode, error) {
	if val == starlark.None {
		return xos.StreamInherit, nil
	}
	if isSpecial(val, pipeValue) {
		return xos.StreamPipe, nil
	}
	if isSpecial(val, devnullValue) {
		return xos.StreamDiscard, nil
	}
	if allowStdout && isSpecial(val, stdoutValue) {
		return xos.StreamStdout, nil
	}
	if allowStdout {
		return xos.StreamInherit, fmt.Errorf("%s: %s must be PIPE, STDOUT, DEVNULL, or None", fn, name)
	}
	return xos.StreamInherit, fmt.Errorf("%s: %s must be PIPE, DEVNULL, or None", fn, name)
}

func optionalString(fn, name string, val starlark.Value) (string, error) {
	if val == starlark.None {
		return "", nil
	}
	s, ok := starlark.AsString(val)
	if !ok {
		return "", fmt.Errorf("%s: %s must be a string or None", fn, name)
	}
	return s, nil
}

func inputBytes(fn string, val starlark.Value, text bool) ([]byte, error) {
	if val == starlark.None {
		return nil, nil
	}
	if text {
		if s, ok := starlark.AsString(val); ok {
			return []byte(s), nil
		}
		return nil, fmt.Errorf("%s: text input must be a string", fn)
	}
	if b, ok := val.(starlark.Bytes); ok {
		return []byte(string(b)), nil
	}
	return nil, fmt.Errorf("%s: binary input must be bytes", fn)
}

func envList(fn string, val starlark.Value) ([]string, error) {
	if val == starlark.None {
		return nil, nil
	}
	dict, ok := val.(*starlark.Dict)
	if !ok {
		return nil, fmt.Errorf("%s: env must be a dict", fn)
	}
	items := dict.Items()
	environ := make([]string, len(items))
	for i, item := range items {
		key, ok := starlark.AsString(item[0])
		if !ok {
			return nil, fmt.Errorf("%s: env keys must be strings", fn)
		}
		value, ok := starlark.AsString(item[1])
		if !ok {
			return nil, fmt.Errorf("%s: env values must be strings", fn)
		}
		environ[i] = key + "=" + value
	}
	return environ, nil
}

func isSpecial(val starlark.Value, want int) bool {
	intVal, ok := val.(starlark.Int)
	if !ok {
		return false
	}
	got, ok := intVal.Int64()
	return ok && got == int64(want)
}

func streamValue(data []byte, captured, text bool) starlark.Value {
	if !captured {
		return starlark.None
	}
	if text {
		return starlark.String(string(data))
	}
	return starlark.Bytes(string(data))
}
