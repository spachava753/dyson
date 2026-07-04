# Tests for Dyson's Python-like os compatibility module.
#
# Host-facing primitives are currently Go stubs. These chunks cover the
# source-backed module shape and deterministic Starlark path helpers.

---
# The module exposes the public os namespace and nested os.path namespace.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(type(os.listdir), "builtin_function_or_method")
assert.eq(type(os.stat), "builtin_function_or_method")
assert.eq(type(os.path), "module")
assert.eq(type(os.path.join), "function")
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
# The filesystem primitives currently implemented for glob use the injected xfs
# policy through os.listdir, os.path.isdir, and os.path.lexists.
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
# Pure path joining and normalization are implemented in Starlark.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.path.isabs("/tmp"), True)
assert.eq(os.path.isabs("tmp"), False)
assert.eq(os.path.join("a", "b", "c"), "a/b/c")
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
