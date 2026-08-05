package subprocess

import (
	"bytes"
	"fmt"

	"go.starlark.net/starlark"
)

// TimeoutExpiredError reports that subprocess.run exceeded its timeout.
// Cmd and Timeout retain the original Starlark arguments. Stdout and Stderr
// contain partial captured bytes, or nil when no output was captured.
type TimeoutExpiredError struct {
	Cmd     starlark.Value
	Timeout starlark.Value
	Stdout  []byte
	Stderr  []byte
}

// Error returns the Starlark-visible timeout message.
func (e *TimeoutExpiredError) Error() string {
	return fmt.Sprintf("subprocess.run: command timed out after %s seconds", e.Timeout)
}

func newTimeoutExpiredError(
	cmd, timeout starlark.Value,
	stdout, stderr []byte,
	stdoutCaptured, stderrCaptured bool,
) *TimeoutExpiredError {
	err := &TimeoutExpiredError{Cmd: cmd, Timeout: timeout}
	if stdoutCaptured && len(stdout) != 0 {
		err.Stdout = bytes.Clone(stdout)
	}
	if stderrCaptured && len(stderr) != 0 {
		err.Stderr = bytes.Clone(stderr)
	}
	return err
}
