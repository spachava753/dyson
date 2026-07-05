package xos

import (
	"os"
	"os/exec"
	"runtime"
	"syscall"
)

// Env controls environment reads and writes exposed by os and os.path.
type Env interface {
	Environ() []string
	LookupEnv(key string) (string, bool)
	Setenv(key, value string) error
	Unsetenv(key string) error
	UserHomeDir() (string, error)
	ExpandEnv(path string) string
}

// Process controls process and shell primitives exposed by os.
type Process interface {
	Getpid() int
	Getppid() int
	Kill(pid, sig int) error
	System(command string) (int, error)
	Getuid() int
	Geteuid() int
	Getgid() int
	Getegid() int
	Getgroups() ([]int, error)
	Umask(mask int) int
}

// WorkingDir controls process working-directory-like operations.
type WorkingDir interface {
	Getwd() (string, error)
	Chdir(path string) error
}

// OpenFlags are the integer constants consumed by filesystem OpenFile implementations.
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

// Platform describes the OS constants visible through the Starlark os module.
type Platform struct {
	OSName            string
	PathListSeparator string
	DevNull           string
	OpenFlags         OpenFlags
}

// Platformer is implemented by hosts that provide platform constants.
type Platformer interface {
	Platform() Platform
}

// PortablePlatform is used when no host platform is configured. It is sufficient
// for contained filesystems that do not expose descriptor-style host I/O.
var PortablePlatform = Platform{
	OSName:            "posix",
	PathListSeparator: ":",
	DevNull:           "/dev/null",
	OpenFlags: OpenFlags{
		ReadOnly:  0,
		WriteOnly: 1,
		ReadWrite: 2,
		Append:    0x400,
		Create:    0x40,
		Exclusive: 0x80,
		Sync:      0x101000,
		Truncate:  0x200,
	},
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

// System runs command through the host shell and returns its exit code.
func (Host) System(command string) (int, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("/bin/sh", "-c", command)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode(), nil
		}
		return 0, err
	}
	return 0, nil
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
func (Host) Umask(mask int) int { return syscall.Umask(mask) }

// Getwd returns the current host working directory.
func (Host) Getwd() (string, error) { return os.Getwd() }

// Chdir changes the current host working directory.
func (Host) Chdir(path string) error { return os.Chdir(path) }

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
