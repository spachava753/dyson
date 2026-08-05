package xos

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Env controls environment reads and writes exposed to Starlark.
type Env interface {
	Environ() []string
	LookupEnv(key string) (string, bool)
	Setenv(key, value string) error
	Unsetenv(key string) error
	UserHomeDir() (string, error)
	ExpandEnv(path string) string
}

// Process controls process identity and signaling primitives.
type Process interface {
	Getpid() int
	Getppid() int
	Kill(pid, signal int) error
	Getuid() int
	Geteuid() int
	Getgid() int
	Getegid() int
	Getgroups() ([]int, error)
	Umask(mask int) int
}

// Command describes one subprocess execution request.
type Command struct {
	Args  []string
	Shell bool
	Input []byte
	Env   []string
	// Dir is the command working directory. An empty value leaves default
	// directory selection to the caller of CommandRunner.
	Dir    string
	Stdin  StreamMode
	Stdout StreamMode
	Stderr StreamMode
}

// StreamMode describes how a subprocess standard stream is connected.
type StreamMode int

const (
	// StreamInherit leaves the stream connected to the parent process.
	StreamInherit StreamMode = iota
	// StreamPipe supplies stdin from Command.Input or captures output in CommandResult.
	StreamPipe
	// StreamDiscard connects the stream to the operating system's null device.
	StreamDiscard
	// StreamStdout redirects stderr to the command's stdout destination.
	StreamStdout
)

// CommandResult is the completed result of a subprocess execution. When
// execution is canceled, Stdout and Stderr may contain partial captured output.
type CommandResult struct {
	ReturnCode int
	Stdout     []byte
	Stderr     []byte
}

// CommandRunner executes subprocess requests and must honor context cancellation.
// On cancellation, implementations should return already captured output in the
// CommandResult together with the context error.
type CommandRunner interface {
	RunCommand(ctx context.Context, command Command) (CommandResult, error)
}

// Clock provides wall time, monotonic elapsed time, and sleeping.
type Clock interface {
	Now() time.Time
	Sleep(ctx context.Context, duration time.Duration) error
}

// WorkingDir controls working-directory-like operations.
type WorkingDir interface {
	Getwd() (string, error)
	Chdir(path string) error
}

// Terminal controls terminal-size queries.
type Terminal interface {
	TerminalSize() (columns, lines int, err error)
}

// OpenFlags contains integer flags consumed by filesystem OpenFile methods.
type OpenFlags struct {
	ReadOnly  int
	WriteOnly int
	ReadWrite int
	Append    int
	Create    int
	Exclusive int
	Sync      int
	Truncate  int
}

// Platform contains OS constants visible to Starlark modules.
type Platform struct {
	OSName            string
	PathListSeparator string
	DevNull           string
	OpenFlags         OpenFlags
}

// Platformer supplies host platform constants.
type Platformer interface {
	Platform() Platform
}

// Host exposes host OS environment, process, working-directory, and platform behavior.
type Host struct{}

// Environ returns the host process environment.
func (Host) Environ() []string { return os.Environ() }

// LookupEnv looks up a host environment variable.
func (Host) LookupEnv(key string) (string, bool) { return os.LookupEnv(key) }

// Setenv sets a host environment variable.
func (Host) Setenv(key, value string) error { return os.Setenv(key, value) }

// Unsetenv unsets a host environment variable.
func (Host) Unsetenv(key string) error { return os.Unsetenv(key) }

// UserHomeDir returns the host user's home directory.
func (Host) UserHomeDir() (string, error) { return os.UserHomeDir() }

// ExpandEnv expands host environment variables in path.
func (Host) ExpandEnv(path string) string { return os.ExpandEnv(path) }

// Getpid returns the host process id.
func (Host) Getpid() int { return os.Getpid() }

// Getppid returns the parent host process id.
func (Host) Getppid() int { return os.Getppid() }

// Kill sends sig to pid using host process semantics.
func (Host) Kill(pid, sig int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.Signal(sig))
}

// Getuid returns the host user id.
func (Host) Getuid() int { return os.Getuid() }

// Geteuid returns the host effective user id.
func (Host) Geteuid() int { return os.Geteuid() }

// Getgid returns the host group id.
func (Host) Getgid() int { return os.Getgid() }

// Getegid returns the host effective group id.
func (Host) Getegid() int { return os.Getegid() }

// Getgroups returns the host supplementary group ids.
func (Host) Getgroups() ([]int, error) { return os.Getgroups() }

// Umask changes the host process umask and returns the previous value.
func (Host) Umask(mask int) int { return hostUmask(mask) }

// RunCommand executes a host subprocess and captures requested streams.
func (Host) RunCommand(ctx context.Context, command Command) (CommandResult, error) {
	if len(command.Args) == 0 {
		return CommandResult{}, exec.ErrNotFound
	}

	var cmd *exec.Cmd
	if command.Shell {
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(ctx, "cmd", append([]string{"/C", command.Args[0]}, command.Args[1:]...)...)
		} else {
			cmd = exec.CommandContext(ctx, "/bin/sh", append([]string{"-c", command.Args[0]}, command.Args[1:]...)...)
		}
	} else {
		path := command.Args[0]
		if command.Env != nil && !strings.ContainsAny(path, `/\\`) {
			resolved, err := lookPathInEnv(path, command.Env)
			if err != nil {
				return CommandResult{}, err
			}
			path = resolved
		}
		cmd = exec.CommandContext(ctx, path, command.Args[1:]...)
	}
	if command.Dir != "" {
		cmd.Dir = command.Dir
	}
	if command.Env != nil {
		cmd.Env = command.Env
	}
	streams, err := prepareCommandIO(cmd, command)
	if err != nil {
		return CommandResult{}, err
	}
	if err := cmd.Start(); err != nil {
		streams.closeAll()
		if ctx.Err() != nil {
			return CommandResult{}, ctx.Err()
		}
		return CommandResult{}, err
	}
	streams.start()
	waitErr := cmd.Wait()
	streamErr, streamCanceled := streams.wait(ctx)
	result := CommandResult{}
	if streams.stdout != nil {
		result.Stdout = streams.stdout.buffer.Bytes()
	}
	if streams.stderr != nil {
		result.Stderr = streams.stderr.buffer.Bytes()
	}
	if ctx.Err() != nil && (waitErr != nil || streamCanceled) {
		return result, ctx.Err()
	}

	if waitErr != nil {
		if exit, ok := waitErr.(*exec.ExitError); ok {
			result.ReturnCode = exit.ExitCode()
		} else {
			return CommandResult{}, waitErr
		}
	}
	if streamErr != nil {
		return CommandResult{}, streamErr
	}
	return result, nil
}

// lookPathInEnv searches only the PATH supplied in env and returns the first
// non-directory candidate with any executable bit set.
func lookPathInEnv(file string, env []string) (string, error) {
	pathList := ""
	for _, kv := range env {
		key, value, found := strings.Cut(kv, "=")
		if found && key == "PATH" {
			pathList = value
			break
		}
	}
	for _, dir := range filepath.SplitList(pathList) {
		if dir == "" {
			dir = "."
		}
		path := filepath.Join(dir, file)
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return path, nil
		}
	}
	return "", exec.ErrNotFound
}

// Getwd returns the current host working directory.
func (Host) Getwd() (string, error) { return os.Getwd() }

// Chdir changes the current host working directory.
func (Host) Chdir(path string) error { return os.Chdir(path) }

// TerminalSize returns terminal dimensions from COLUMNS and LINES when present.
func (Host) TerminalSize() (int, int, error) {
	columns, err := strconv.Atoi(os.Getenv("COLUMNS"))
	if err != nil || columns <= 0 {
		return 0, 0, fmt.Errorf("terminal size is not available")
	}
	lines, err := strconv.Atoi(os.Getenv("LINES"))
	if err != nil || lines <= 0 {
		return 0, 0, fmt.Errorf("terminal size is not available")
	}
	return columns, lines, nil
}

// Now returns the current host time.
func (Host) Now() time.Time { return time.Now() }

// Sleep blocks for duration or until ctx ends.
func (Host) Sleep(ctx context.Context, duration time.Duration) error {
	select {
	case <-time.After(duration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Platform returns host platform constants.
func (Host) Platform() Platform {
	osName := "posix"
	if runtime.GOOS == "windows" {
		osName = "nt"
	}
	return Platform{
		OSName:            osName,
		PathListSeparator: string(os.PathListSeparator),
		DevNull:           os.DevNull,
		OpenFlags: OpenFlags{
			ReadOnly:  os.O_RDONLY,
			WriteOnly: os.O_WRONLY,
			ReadWrite: os.O_RDWR,
			Append:    os.O_APPEND,
			Create:    os.O_CREATE,
			Exclusive: os.O_EXCL,
			Sync:      os.O_SYNC,
			Truncate:  os.O_TRUNC,
		},
	}
}
