package dyson

import (
	"github.com/spachava753/dyson/internal/xfs"
	"github.com/spachava753/dyson/internal/xos"
)

// FileSystem is the minimum filesystem capability used by standard-library modules.
type FileSystem = xfs.FS

// MutableFileSystem optionally adds filesystem mutation operations.
type MutableFileSystem = xfs.MutFS

// File is a descriptor-style file handle.
type File = xfs.File

// OpenFileSystem optionally adds descriptor-style file I/O.
type OpenFileSystem = xfs.OpenFS

// PathFileSystem optionally adds absolute and canonical path resolution.
type PathFileSystem = xfs.PathFS

// SameFileSystem optionally adds path identity checks.
type SameFileSystem = xfs.SameFileFS

// DiskUsage describes filesystem capacity around a path.
type DiskUsage = xfs.Usage

// DiskUsageFileSystem optionally adds filesystem capacity reporting.
type DiskUsageFileSystem = xfs.UsageFS

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
type Command = xos.Command

// StreamMode describes how a subprocess standard stream is connected.
type StreamMode = xos.StreamMode

const (
	StreamInherit = xos.StreamInherit
	StreamPipe    = xos.StreamPipe
	StreamDiscard = xos.StreamDiscard
	StreamStdout  = xos.StreamStdout
)

// CommandResult is the completed result of a subprocess execution.
type CommandResult = xos.CommandResult

// CommandRunner executes subprocess requests and must honor context cancellation.
type CommandRunner = xos.CommandRunner

// StdlibConfig declares the host capabilities shared by standard-library
// modules. Except for Platform's portable constants, nil or zero capabilities
// remain unavailable rather than falling back to ambient host access.
type StdlibConfig struct {
	// FS is shared by os, glob, shutil, and tempfile. The concrete filesystem
	// owns path resolution and containment; optional interfaces add mutation,
	// descriptor I/O, canonical paths, identity, and disk usage.
	FS FileSystem

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
	// value selects portable POSIX-like constants without granting host access.
	Platform Platform

	// CommandRunner supplies os.system and subprocess command execution. Nil
	// keeps both modules fail-closed.
	CommandRunner CommandRunner

	// Clock supplies current time and sleeping to time, plus implicit timestamps
	// such as os.utime(path). Nil keeps those operations fail-closed.
	Clock Clock
}

// HostStdlibConfig grants broad access to the host filesystem, mutable process
// environment, process identity and signaling, working directory, terminal,
// platform constants, and clock. root is only the base for relative filesystem
// paths, not a containment boundary: absolute paths and parent traversal retain
// normal host semantics. Command execution remains disabled unless a
// CommandRunner is assigned explicitly.
func HostStdlibConfig(root string) StdlibConfig {
	host := xos.Host{}
	return StdlibConfig{
		FS:               xfs.HostFS{Root: root},
		Env:              host,
		Process:          host,
		WorkingDirectory: host,
		Terminal:         host,
		Platform:         host.Platform(),
		Clock:            host,
	}
}

// HostCommandRunner returns a runner that may execute arbitrary commands on the
// host. Assign it only when the evaluated Starlark code is trusted to launch
// processes with the current user's authority.
func HostCommandRunner() CommandRunner {
	return xos.Host{}
}
