# Tests for Dyson's Python-like os compatibility module.
#
# This file is chunked by lines containing "---". Each chunk executes as an
# independent Starlark file against a small filesystem supplied by the Go test
# harness.

---
# listdir returns sorted entry names and defaults to the current directory.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.listdir("."), root_entries)
assert.eq(os.listdir(), root_entries)

---
# os.path.isdir follows stat semantics and returns False for files or missing
# paths instead of raising.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.path.isdir("dir"), True)
assert.eq(os.path.isdir("file.txt"), False)
assert.eq(os.path.isdir("missing"), False)

---
# os.path.lexists reports whether a path exists according to the backing
# filesystem's lstat behavior.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.path.lexists("file.txt"), True)
assert.eq(os.path.lexists("missing"), False)

---
# Callers choose the filesystem path policy used by os builtins.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(os.listdir(sibling_dir), ["file.txt"])
assert.eq(os.path.lexists(sibling_dir + "/file.txt"), True)
assert.eq(os.path.isdir(sibling_dir), True)
