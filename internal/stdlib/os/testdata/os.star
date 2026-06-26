# Tests for Dyson's Python-like os compatibility module.
#
# This file is chunked by lines containing "---". Each chunk executes as an
# independent Starlark file so related assertions stay small and failures point
# at a focused behavior area. Tests assert the desired Python-like os behavior.
# The Go harness provides TEST_TMPDIR, TEST_FILE, TEST_DIR, and TEST_MISSING
# globals for filesystem scenarios.

---
# The module is imported as a namespace symbol, matching Dyson stdlib load
# semantics and Python-style call sites.
load("assert.star", "assert")
load("os.star", "os")

assert.true("path" in dir(os), "path")
assert.true("sep" in dir(os), "sep")
assert.true("getcwd" in dir(os), "getcwd")
assert.true("join" not in dir(os), "direct os.path members are not exported on os")

---
# Platform constants expose the host flavor. These tests currently target the
# POSIX/Darwin development environment used by Dyson.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.name, "posix")
assert.eq(os.sep, "/")
assert.eq(os.pathsep, ":")
assert.eq(os.curdir, ".")
assert.eq(os.pardir, "..")
assert.eq(os.extsep, ".")
assert.eq(os.altsep, None)
assert.eq(os.linesep, "\n")
assert.eq(os.defpath, "/bin:/usr/bin")
assert.eq(os.devnull, "/dev/null")

---
# os.path exposes the same path constants.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.path.sep, "/")
assert.eq(os.path.pathsep, ":")
assert.eq(os.path.curdir, ".")
assert.eq(os.path.pardir, "..")
assert.eq(os.path.extsep, ".")
assert.eq(os.path.altsep, None)
assert.eq(os.path.defpath, "/bin:/usr/bin")
assert.eq(os.path.devnull, "/dev/null")
assert.eq(os.path.supports_unicode_filenames, True)

---
# environ reflects the host process environment and is mutable through os
# helpers.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.getenv("DYSON_TEST_ENV"), "initial")
assert.eq(os.environ["DYSON_TEST_ENV"], "initial")
os.putenv("DYSON_TEST_ENV", "changed")
assert.eq(os.getenv("DYSON_TEST_ENV"), "changed")
assert.eq(os.environ["DYSON_TEST_ENV"], "changed")
os.unsetenv("DYSON_TEST_ENV")
assert.eq(os.getenv("DYSON_TEST_ENV", "fallback"), "fallback")

---
# fspath follows Python's simple coercion rule for strings and bytes.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.fspath("a/b"), "a/b")
assert.eq(os.fspath(b"a/b"), b"a/b")

---
# fsencode converts strings to UTF-8 bytes and leaves bytes unchanged.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.fsencode("snowman-☃"), b"snowman-\xe2\x98\x83")
assert.eq(os.fsencode(b"already-bytes"), b"already-bytes")

---
# fsdecode converts UTF-8 bytes to strings and leaves strings unchanged.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.fsdecode(b"snowman-\xe2\x98\x83"), "snowman-☃")
assert.eq(os.fsdecode("already-text"), "already-text")

---
# strerror returns stable POSIX-style messages for common errno values.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.strerror(2), "No such file or directory")
assert.eq(os.strerror(13), "Permission denied")

---
# os.path.join uses POSIX path semantics and resets earlier segments when an
# absolute segment appears.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.path.join("a", "b", "c"), "a/b/c")
assert.eq(os.path.join("a/", "b"), "a/b")
assert.eq(os.path.join("a", "/b", "c"), "/b/c")
assert.eq(os.path.join("", "b"), "b")

---
# basename, dirname, and split follow Python's posixpath behavior.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.path.basename("/a/b.txt"), "b.txt")
assert.eq(os.path.basename("/a/b/"), "")
assert.eq(os.path.dirname("/a/b.txt"), "/a")
assert.eq(os.path.split("/a/b.txt"), ("/a", "b.txt"))
assert.eq(os.path.split("b.txt"), ("", "b.txt"))

---
# splitext separates the final extension without treating leading dots as an
# extension.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.path.splitext("archive.tar.gz"), ("archive.tar", ".gz"))
assert.eq(os.path.splitext(".bashrc"), (".bashrc", ""))
assert.eq(os.path.splitext("/a/.b.txt"), ("/a/.b", ".txt"))

---
# normpath collapses redundant separators and dot segments using POSIX rules.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.path.normpath("a//b/./c"), "a/b/c")
assert.eq(os.path.normpath("a/b/../c"), "a/c")
assert.eq(os.path.normpath("/../a"), "/a")
assert.eq(os.path.normpath(""), ".")

---
# abspath and isabs use the host current working directory.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.path.isabs(TEST_TMPDIR), True)
assert.eq(os.path.isabs("relative"), False)
assert.eq(os.path.abspath("."), os.getcwd())

---
# Filesystem predicates observe the host filesystem.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.path.exists(TEST_FILE), True)
assert.eq(os.path.isfile(TEST_FILE), True)
assert.eq(os.path.isdir(TEST_FILE), False)
assert.eq(os.path.exists(TEST_DIR), True)
assert.eq(os.path.isdir(TEST_DIR), True)
assert.eq(os.path.exists(TEST_MISSING), False)

---
# Directory listing returns names in the host directory.
load("assert.star", "assert")
load("os.star", "os")

names = sorted(os.listdir(TEST_TMPDIR))
assert.true("sample.txt" in names, "sample.txt")
assert.true("subdir" in names, "subdir")

---
# stat exposes common file metadata fields.
load("assert.star", "assert")
load("os.star", "os")

info = os.stat(TEST_FILE)
assert.true(info.st_size > 0, "st_size")
assert.true(info.st_mode > 0, "st_mode")
assert.true(info.st_mtime >= 0.0, "st_mtime")

---
# mkdir, makedirs, rename, replace, remove, unlink, and rmdir mutate the host
# filesystem.
load("assert.star", "assert")
load("os.star", "os")

base = os.path.join(TEST_TMPDIR, "mutations")
os.makedirs(os.path.join(base, "nested"))
assert.eq(os.path.isdir(os.path.join(base, "nested")), True)
created = os.path.join(base, "created.txt")
renamed = os.path.join(base, "renamed.txt")
replaced = os.path.join(base, "replaced.txt")
os.write_text(created, "created")
os.rename(created, renamed)
assert.eq(os.path.exists(created), False)
assert.eq(os.path.exists(renamed), True)
os.write_text(replaced, "old")
os.replace(renamed, replaced)
assert.eq(os.path.exists(renamed), False)
assert.eq(os.read_text(replaced), "created")
os.remove(replaced)
assert.eq(os.path.exists(replaced), False)
empty = os.path.join(base, "empty")
os.mkdir(empty)
os.rmdir(empty)
assert.eq(os.path.exists(empty), False)

---
# walk yields directory traversal tuples of (dirpath, dirnames, filenames).
load("assert.star", "assert")
load("os.star", "os")

seen = list(os.walk(TEST_TMPDIR))
assert.true(len(seen) >= 1, "walk entries")
root = seen[0]
assert.eq(root[0], TEST_TMPDIR)
assert.true("subdir" in root[1], "walk dirnames")
assert.true("sample.txt" in root[2], "walk filenames")

---
# system executes a shell command and returns its exit status.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.system("true"), 0)

---
# popen executes a command and exposes readable output.
load("assert.star", "assert")
load("os.star", "os")

pipe = os.popen("printf hello")
assert.eq(pipe.read(), "hello")
assert.eq(pipe.close(), None)

---
# Pure helper argument validation should be function-local and deterministic.
load("os.star", "os")

os.path.join("a", 1) ### "os.path.join: path must be str or bytes, got int"

---
# The broad Python os callable surface is present as module members, even while
# many functions are still red implementation tests.
load("assert.star", "assert")
load("os.star", "os")

def assert_names(names, namespace):
    for name in names:
        assert.true(name in dir(namespace), name)

assert_names([
    "access", "chdir", "chmod", "chown", "close", "cpu_count", "dup", "dup2", "fork", "fsync",
    "getcwd", "getcwdb", "getpid", "getppid", "getuid", "geteuid", "getgid", "getegid", "kill",
    "link", "lseek", "lstat", "open", "pipe", "read", "readlink", "removedirs", "renames",
    "symlink", "truncate", "umask", "uname", "urandom", "utime", "waitpid", "write",
], os)

---
# Process execution families are present for future Python-compatible support.
load("assert.star", "assert")
load("os.star", "os")

def assert_names(names, namespace):
    for name in names:
        assert.true(name in dir(namespace), name)

assert_names([
    "execl", "execle", "execlp", "execlpe", "execv", "execve", "execvp", "execvpe",
    "spawnl", "spawnle", "spawnlp", "spawnlpe", "spawnv", "spawnve", "spawnvp", "spawnvpe",
    "posix_spawn", "posix_spawnp",
], os)

---
# Wait status helpers and wait constants are present.
load("assert.star", "assert")
load("os.star", "os")

def assert_names(names, namespace):
    for name in names:
        assert.true(name in dir(namespace), name)

assert_names(["WCOREDUMP", "WEXITSTATUS", "WIFCONTINUED", "WIFEXITED", "WIFSIGNALED", "WIFSTOPPED", "WSTOPSIG", "WTERMSIG"], os)
assert.true(os.WNOHANG >= 0, "WNOHANG")
assert.true(os.WUNTRACED >= 0, "WUNTRACED")
assert.true(os.WCONTINUED >= 0, "WCONTINUED")

---
# File descriptor, access, and seek constants are present.
load("assert.star", "assert")
load("os.star", "os")

assert.eq((os.F_OK, os.R_OK, os.W_OK, os.X_OK), (0, 4, 2, 1))
assert.eq((os.SEEK_SET, os.SEEK_CUR, os.SEEK_END), (0, 1, 2))
def assert_names(names, namespace):
    for name in names:
        assert.true(name in dir(namespace), name)

assert_names(["O_RDONLY", "O_WRONLY", "O_RDWR", "O_APPEND", "O_CREAT", "O_EXCL", "O_TRUNC", "O_CLOEXEC"], os)

---
# Exit status and process priority constants are present.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.EX_OK, 0)
def assert_names(names, namespace):
    for name in names:
        assert.true(name in dir(namespace), name)

assert_names([
    "EX_USAGE", "EX_DATAERR", "EX_NOINPUT", "EX_NOUSER", "EX_NOHOST", "EX_UNAVAILABLE",
    "EX_SOFTWARE", "EX_OSERR", "EX_OSFILE", "EX_CANTCREAT", "EX_IOERR", "EX_TEMPFAIL",
    "EX_PROTOCOL", "EX_NOPERM", "EX_CONFIG", "PRIO_PROCESS", "PRIO_PGRP", "PRIO_USER",
], os)

---
# Python os support-set globals are present as sets.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(type(os.supports_dir_fd), "set")
assert.eq(type(os.supports_effective_ids), "set")
assert.eq(type(os.supports_fd), "set")
assert.eq(type(os.supports_follow_symlinks), "set")
assert.eq(os.supports_bytes_environ, True)

---
# Additional os.path helpers are present for future Python-compatible support.
load("assert.star", "assert")
load("os.star", "os")

def assert_names(names, namespace):
    for name in names:
        assert.true(name in dir(namespace), name)

assert_names([
    "commonpath", "commonprefix", "expanduser", "expandvars", "getatime", "getctime", "getmtime",
    "getsize", "islink", "ismount", "lexists", "normcase", "realpath", "relpath", "samefile",
    "sameopenfile", "samestat", "splitdrive",
], os.path)
