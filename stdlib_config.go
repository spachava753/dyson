package dyson

import (
	"context"
	"io/fs"

	stdlibsubprocess "github.com/spachava753/dyson/internal/stdlib/subprocess"
	"github.com/spachava753/dyson/internal/stdlibfs"
	"github.com/spachava753/dyson/internal/xhttp"
	"github.com/spachava753/dyson/internal/xos"
	"github.com/spf13/afero"
)

// Environment controls environment reads and writes exposed to Starlark.
type Environment = xos.Env

// Clock provides wall time, monotonic elapsed time, and sleeping.
type Clock = xos.Clock

// Process provides process identity and signaling without command execution.
type Process = xos.Process

// WorkingDirectory controls working-directory-like operations.
type WorkingDirectory = xos.WorkingDir

// Terminal controls terminal-size queries.
type Terminal = xos.Terminal

// OpenFlags contains integer flags consumed by filesystem OpenFile methods.
type OpenFlags = xos.OpenFlags

// Platform contains OS constants visible to Starlark modules.
type Platform = xos.Platform

// Command describes a subprocess request made by the Starlark standard library.
// For a config returned by HostStdlibConfig, NewStdlib normalizes an empty
// Dir to the configured filesystem root before invoking CommandRunner.
type Command = xos.Command

// StreamMode describes how a subprocess standard stream is connected.
type StreamMode = xos.StreamMode

const (
	// StreamInherit leaves the stream connected to the parent process.
	StreamInherit = xos.StreamInherit
	// StreamPipe supplies stdin from Command.Input or captures output in CommandResult.
	StreamPipe = xos.StreamPipe
	// StreamDiscard connects the stream to the operating system's null device.
	StreamDiscard = xos.StreamDiscard
	// StreamStdout redirects stderr to the command's stdout destination.
	StreamStdout = xos.StreamStdout
)

// CommandResult is the completed result of a subprocess execution. When
// execution is canceled, Stdout and Stderr may contain partial captured output.
type CommandResult = xos.CommandResult

// SubprocessTimeoutExpiredError reports that subprocess.run exceeded its
// timeout. It is wrapped by the Starlark evaluation error returned from Eval.
type SubprocessTimeoutExpiredError = stdlibsubprocess.TimeoutExpiredError

// CommandRunner executes normalized subprocess requests and must honor context
// cancellation. On cancellation, implementations should return already captured
// output in CommandResult together with the context error. See Command.Dir for
// HostStdlibConfig's default-directory rule.
type CommandRunner = xos.CommandRunner

// HTTPRequest is the normalized request passed to an HTTPClient.
type HTTPRequest = xhttp.Request

// HTTPResponse is the fully buffered response returned by an HTTPClient.
type HTTPResponse = xhttp.Response

// HTTPBodyLimitError reports that a buffered response exceeded the host limit.
type HTTPBodyLimitError = xhttp.BodyLimitError

// HTTPClient performs buffered HTTP requests and must honor context cancellation.
type HTTPClient = xhttp.Client

// FromIOFS adapts a standard read-only filesystem to Afero with Python-style
// leading ./ handling, source-level fs.ReadDirFS traversal, strict no-follow
// inspection when fs.ReadLinkFS is available, and rejection of non-read-only
// descriptor flags.
func FromIOFS(fsys fs.FS) afero.Fs {
	return stdlibfs.NewIOFS(fsys)
}

// StdlibConfig declares the host capabilities shared by standard-library
// modules. Except for Platform's host-derived constants, nil or zero capabilities
// remain unavailable rather than falling back to ambient host access.
type StdlibConfig struct {
	// FS is the Afero filesystem shared by open, os, glob, shutil, and tempfile.
	// Nil keeps filesystem operations unavailable.
	FS afero.Fs

	// Env is shared by os, shutil, and tempfile. Nil disables environment access.
	Env Environment

	// Process supplies identity, signaling, group, and umask operations to os.
	// It does not grant command execution.
	Process Process

	// WorkingDirectory supplies os.getcwd and os.chdir.
	WorkingDirectory WorkingDirectory

	// Terminal supplies shutil.get_terminal_size.
	Terminal Terminal

	// Platform supplies path separators, device names, and open flags. Its zero
	// value selects the current Go platform's constants without granting access.
	Platform Platform

	// CommandRunner supplies os.system and subprocess command execution. Nil
	// keeps both modules fail-closed. For configs returned by HostStdlibConfig,
	// commands with an empty Dir are passed to the runner with Dir set to root.
	CommandRunner CommandRunner

	// HTTPClient supplies requests.request network access. Nil keeps the requests
	// module loadable but fail-closed.
	HTTPClient HTTPClient

	// Clock supplies current time and sleeping to time, plus implicit timestamps
	// such as os.utime(path). Nil keeps those operations fail-closed.
	Clock Clock
}

type defaultDirectoryCommandRunner struct {
	runner CommandRunner
	dir    string
}

// RunCommand applies the configured default directory before delegating the command.
func (r defaultDirectoryCommandRunner) RunCommand(ctx context.Context, command Command) (CommandResult, error) {
	if command.Dir == "" {
		command.Dir = r.dir
	}
	return r.runner.RunCommand(ctx, command)
}

// configuredCommandRunner applies the rooted Afero host backend's default
// directory without overriding an explicit subprocess cwd.
func (c StdlibConfig) configuredCommandRunner() CommandRunner {
	if c.CommandRunner == nil {
		return nil
	}
	fsys, ok := c.FS.(stdlibfs.Host)
	if !ok {
		return c.CommandRunner
	}
	return defaultDirectoryCommandRunner{runner: c.CommandRunner, dir: fsys.Root}
}

// HostStdlibConfig grants broad access to the host filesystem, mutable process
// environment, process identity and signaling, working directory, terminal,
// platform constants, and clock. root is only the base for relative filesystem
// paths, not a containment boundary: absolute paths and parent traversal retain
// normal host semantics. Command execution and HTTP access remain disabled
// unless CommandRunner and HTTPClient are assigned explicitly; commands without
// an explicit working directory then start in root.
func HostStdlibConfig(root string) StdlibConfig {
	host := xos.Host{}
	return StdlibConfig{
		FS:               stdlibfs.NewHost(afero.NewOsFs(), root),
		Env:              host,
		Process:          host,
		WorkingDirectory: host,
		Terminal:         host,
		Platform:         host.Platform(),
		Clock:            host,
	}
}

// HostHTTPClient returns a client that may make arbitrary HTTP requests with
// the current user's network authority. Assign it only when the evaluated
// Starlark code is trusted to access the network.
func HostHTTPClient() HTTPClient {
	return xhttp.HostClient{}
}

// HostCommandRunner returns a runner that may execute arbitrary commands on the
// host. Assign it only when the evaluated Starlark code is trusted to launch
// processes with the current user's authority. When assigned to a config from
// HostStdlibConfig, commands that omit cwd start in that config's root.
func HostCommandRunner() CommandRunner {
	return xos.Host{}
}
