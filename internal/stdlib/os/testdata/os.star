# Tests for Dyson's Python-like os compatibility module.
#
# These chunks cover the source-backed module shape, deterministic Starlark path
# helpers, and read-only primitives that work against the test xfs.IOFS.

---
# The module exposes the public os namespace and nested os.path namespace.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(type(os.listdir), "builtin_function_or_method")
assert.eq(type(os.stat), "builtin_function_or_method")
assert.eq(type(os.path), "module")
assert.eq(type(os.path.join), "builtin_function_or_method")
assert.eq(type(os.path.isdir), "builtin_function_or_method")
assert.eq(os.name in ("posix", "nt"), True)
assert.eq(os.curdir, ".")
assert.eq(os.pardir, "..")
assert.eq(os.sep, "/")
assert.eq(os.extsep, ".")
assert.eq(os.linesep, "\n")
assert.eq(os.F_OK, 0)
assert.eq(os.R_OK, 4)
assert.eq(os.W_OK, 2)
assert.eq(os.X_OK, 1)

---
# Basic filesystem primitives use the injected xfs policy for directory listing
# and path existence/type checks.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.listdir("."), root_entries)
assert.eq(os.listdir(), root_entries)
assert.eq(os.path.isdir("dir"), True)
assert.eq(os.path.isdir("file.txt"), False)
assert.eq(os.path.isdir("missing"), False)
assert.eq(os.path.lexists("file.txt"), True)
assert.eq(os.path.lexists("missing"), False)
assert.eq(os.listdir(sibling_dir), ["file.txt"])
assert.eq(os.path.lexists(sibling_dir + "/file.txt"), True)
assert.eq(os.path.isdir(sibling_dir), True)

---
# Read-only filesystem primitives expose stat results, directory entries, and
# walk output through the injected xfs policy.
load("assert.star", "assert")
load("os.star", "os")

file_stat = os.stat("file.txt")
assert.eq(file_stat.st_size, 7)
assert.eq(file_stat.is_dir, False)
assert.eq(os.lstat("dir").is_dir, True)
assert.eq(os.access("file.txt", os.F_OK), True)
assert.eq(os.access("file.txt", os.R_OK), True)
assert.eq(os.access("missing", os.F_OK), False)
assert.eq(os.path.exists("file.txt"), True)
assert.eq(os.path.exists("missing"), False)
assert.eq(os.path.isfile("file.txt"), True)
assert.eq(os.path.isfile("dir"), False)
assert.eq(os.path.getsize("file.txt"), 7)
assert.eq(type(os.path.getmtime("file.txt")), "float")

entries = os.scandir(".")
assert.eq([entry.name for entry in entries], root_entries)
assert.eq(entries[0].path, "dir")
assert.eq(entries[0].is_dir(), True)
assert.eq(entries[0].is_file(), False)
assert.eq(entries[1].is_file(), True)
assert.eq(entries[1].stat().st_size, 7)

walk = os.walk(".")
assert.eq(walk[0], (".", ["dir", "sibling"], ["file.txt"]))
assert.eq(walk[1], ("dir", [], ["nested.txt"]))
assert.eq(walk[2], ("sibling", [], ["file.txt"]))

---
# Pure path splitting helpers follow Python-like POSIX behavior.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.path.split("a/b/c.txt"), ("a/b", "c.txt"))
assert.eq(os.path.split("file.txt"), ("", "file.txt"))
assert.eq(os.path.basename("a/b/c.txt"), "c.txt")
assert.eq(os.path.dirname("a/b/c.txt"), "a/b")
assert.eq(os.path.splitext("a/b/c.txt"), ("a/b/c", ".txt"))
assert.eq(os.path.splitext("a/.profile"), ("a/.profile", ""))
assert.eq(os.path.splitdrive("a/b"), ("", "a/b"))

---
# Pure path joining and normalization are exposed as Go builtins.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.path.isabs("/tmp"), True)
assert.eq(os.path.isabs("tmp"), False)
assert.eq(os.path.join("a", "b", "c"), "a/b/c")
assert.eq(os.path.join(path="a"), "a")
assert.eq(os.path.join("a", "/b", "c"), "/b/c")
assert.eq(os.path.normpath("a//b/./c"), "a/b/c")
assert.eq(os.path.normpath("a/b/../c"), "a/c")
assert.eq(os.path.normpath("/../a"), "/a")

---
# relpath and commonpath are pure path operations over normalized components.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.path.relpath("a/b/c", "a"), "b/c")
assert.eq(os.path.relpath("a/b", "a/b"), ".")
assert.eq(os.path.relpath("a/c", "a/b"), "../c")
assert.eq(os.path.commonpath(["a/b/c", "a/b/d"]), "a/b")
assert.eq(os.path.commonpath(["/a/b", "/a/c"]), "/a")

---
# Missing non-filesystem domains fail explicitly when only an xfs filesystem is configured.
load("assert.star", "assert")
load("os.star", "os")

os.getenv("PATH")  ### "os.getenv: environment operations are not configured"

---
# Command execution is a separate host capability from process metadata.
load("assert.star", "assert")
load("os.star", "os")

os.system("echo hidden")  ### "os.system: subprocess execution is not configured"

---
# Descriptor-style file I/O requires an xfs.OpenFS implementation.
load("assert.star", "assert")
load("os.star", "os")

os.open("file.txt", os.O_RDONLY)  ### "os.open: filesystem does not support file descriptors"

---
# Working-directory operations are separate from contained filesystem access.
load("assert.star", "assert")
load("os.star", "os")

os.getcwd()  ### "os.getcwd: working directory operations are not configured"

---
# Without an environment domain, expandvars is deterministic and leaves text unchanged.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.path.expandvars("$PATH"), "$PATH")
