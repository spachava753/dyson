package os

import (
	"fmt"
	"sync"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

const ModuleName = "os"

const pathModuleName = ModuleName + ".path"

var module = sync.OnceValue(func() starlark.StringDict {
	pathMembers := starlark.StringDict{
		"sep":                        starlark.String("/"),
		"pathsep":                    starlark.String(":"),
		"curdir":                     starlark.String("."),
		"pardir":                     starlark.String(".."),
		"extsep":                     starlark.String("."),
		"altsep":                     starlark.None,
		"defpath":                    starlark.String("/bin:/usr/bin"),
		"devnull":                    starlark.String("/dev/null"),
		"supports_unicode_filenames": starlark.True,
	}
	addBuiltins(pathMembers, pathModuleName, pathFunctions)
	pathModule := &starlarkstruct.Module{Name: pathModuleName, Members: pathMembers}

	members := starlark.StringDict{
		"path": pathModule,

		"name":    starlark.String("posix"),
		"sep":     starlark.String("/"),
		"pathsep": starlark.String(":"),
		"curdir":  starlark.String("."),
		"pardir":  starlark.String(".."),
		"extsep":  starlark.String("."),
		"altsep":  starlark.None,
		"linesep": starlark.String("\n"),
		"defpath": starlark.String("/bin:/usr/bin"),
		"devnull": starlark.String("/dev/null"),

		"environ":                  starlark.NewDict(0),
		"environb":                 starlark.NewDict(0),
		"supports_bytes_environ":   starlark.True,
		"supports_dir_fd":          starlark.NewSet(0),
		"supports_effective_ids":   starlark.NewSet(0),
		"supports_fd":              starlark.NewSet(0),
		"supports_follow_symlinks": starlark.NewSet(0),
	}
	addConstants(members, constants)
	addBuiltins(members, ModuleName, functions)
	addBuiltins(members, ModuleName, execFunctions)
	addBuiltins(members, ModuleName, spawnFunctions)
	addBuiltins(members, ModuleName, waitStatusFunctions)
	addBuiltins(members, ModuleName, dysonConvenienceFunctions)

	module := &starlarkstruct.Module{Name: ModuleName, Members: members}
	module.Freeze()
	return starlark.StringDict{ModuleName: module}
})

var pathFunctions = []string{
	"abspath", "basename", "commonpath", "commonprefix", "dirname", "exists", "expanduser", "expandvars",
	"getatime", "getctime", "getmtime", "getsize", "isabs", "isdir", "isfile", "islink", "ismount",
	"join", "lexists", "normcase", "normpath", "realpath", "relpath", "samefile", "sameopenfile",
	"samestat", "split", "splitdrive", "splitext",
}

var functions = []string{
	"_exit", "abort", "access", "chdir", "chflags", "chmod", "chown", "chroot", "close", "closerange",
	"confstr", "cpu_count", "ctermid", "device_encoding", "dup", "dup2", "fchdir", "fchmod", "fchown",
	"fdatasync", "fdopen", "fork", "forkpty", "fpathconf", "fsdecode", "fsencode", "fspath", "fstat",
	"fstatvfs", "fsync", "ftruncate", "fwalk", "get_blocking", "get_exec_path", "get_inheritable",
	"get_terminal_size", "getcwd", "getcwdb", "getegid", "getenv", "getenvb", "geteuid", "getgid",
	"getgrouplist", "getgroups", "getloadavg", "getlogin", "getpgid", "getpgrp", "getpid", "getppid",
	"getpriority", "getsid", "getuid", "initgroups", "isatty", "kill", "killpg", "lchflags", "lchmod",
	"lchown", "link", "listdir", "lockf", "lseek", "lstat", "major", "makedev", "makedirs", "minor",
	"mkdir", "mkfifo", "mknod", "nice", "open", "openpty", "pathconf", "pipe", "popen", "pread",
	"preadv", "putenv", "pwrite", "pwritev", "read", "readlink", "readv", "register_at_fork", "remove",
	"removedirs", "rename", "renames", "replace", "rmdir", "scandir", "sched_get_priority_max",
	"sched_get_priority_min", "sched_yield", "sendfile", "set_blocking", "set_inheritable", "setegid", "seteuid",
	"setgid", "setgroups", "setpgid", "setpgrp", "setpriority", "setregid", "setreuid", "setsid", "setuid",
	"stat", "statvfs", "strerror", "symlink", "sync", "sysconf", "system", "tcgetpgrp", "tcsetpgrp",
	"times", "truncate", "ttyname", "umask", "uname", "unlink", "unsetenv", "urandom", "utime", "wait",
	"wait3", "wait4", "waitpid", "waitstatus_to_exitcode", "walk", "write", "writev",
}

var execFunctions = []string{"execl", "execle", "execlp", "execlpe", "execv", "execve", "execvp", "execvpe"}

var spawnFunctions = []string{"spawnl", "spawnle", "spawnlp", "spawnlpe", "spawnv", "spawnve", "spawnvp", "spawnvpe", "posix_spawn", "posix_spawnp"}

var waitStatusFunctions = []string{"WCOREDUMP", "WEXITSTATUS", "WIFCONTINUED", "WIFEXITED", "WIFSIGNALED", "WIFSTOPPED", "WSTOPSIG", "WTERMSIG"}

var dysonConvenienceFunctions = []string{"read_text", "write_text"}

var constants = map[string]int{
	"F_OK": 0, "R_OK": 4, "W_OK": 2, "X_OK": 1,
	"SEEK_SET": 0, "SEEK_CUR": 1, "SEEK_END": 2,
	"O_RDONLY": 0, "O_WRONLY": 1, "O_RDWR": 2, "O_APPEND": 8, "O_CREAT": 512, "O_EXCL": 2048,
	"O_TRUNC": 1024, "O_CLOEXEC": 16777216, "O_NONBLOCK": 4, "O_NOFOLLOW": 256,
	"P_WAIT": 0, "P_NOWAIT": 1, "P_NOWAITO": 1,
	"P_PID": 0, "P_PGID": 2, "P_ALL": 1,
	"WNOHANG": 1, "WUNTRACED": 2, "WCONTINUED": 16, "WEXITED": 4, "WNOWAIT": 32, "WSTOPPED": 2,
	"EX_OK": 0, "EX_USAGE": 64, "EX_DATAERR": 65, "EX_NOINPUT": 66, "EX_NOUSER": 67, "EX_NOHOST": 68,
	"EX_UNAVAILABLE": 69, "EX_SOFTWARE": 70, "EX_OSERR": 71, "EX_OSFILE": 72, "EX_CANTCREAT": 73,
	"EX_IOERR": 74, "EX_TEMPFAIL": 75, "EX_PROTOCOL": 76, "EX_NOPERM": 77, "EX_CONFIG": 78,
	"PRIO_PROCESS": 0, "PRIO_PGRP": 1, "PRIO_USER": 2,
	"SCHED_OTHER": 1, "SCHED_FIFO": 4, "SCHED_RR": 2,
	"ST_RDONLY": 1, "ST_NOSUID": 2,
	"F_LOCK": 1, "F_TLOCK": 2, "F_ULOCK": 0, "F_TEST": 3,
	"POSIX_SPAWN_OPEN": 0, "POSIX_SPAWN_CLOSE": 1, "POSIX_SPAWN_DUP2": 2,
	"TMP_MAX": 308915776, "NGROUPS_MAX": 16,
	"CLD_EXITED": 1, "CLD_KILLED": 2, "CLD_DUMPED": 3, "CLD_TRAPPED": 4, "CLD_STOPPED": 5, "CLD_CONTINUED": 6,
}

// LoadModule returns Dyson's Python-compatible os module.
func LoadModule() (starlark.StringDict, error) {
	return module(), nil
}

func addBuiltins(members starlark.StringDict, prefix string, names []string) {
	for _, name := range names {
		members[name] = starlark.NewBuiltin(prefix+"."+name, notImplemented)
	}
}

func addConstants(members starlark.StringDict, values map[string]int) {
	for name, value := range values {
		members[name] = starlark.MakeInt(value)
	}
}

func notImplemented(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return nil, fmt.Errorf("%s: not implemented", fn.Name())
}
