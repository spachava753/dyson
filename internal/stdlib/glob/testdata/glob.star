# Tests for Dyson's Python-like glob compatibility module.
#
# Filesystem-backed glob expansion intentionally waits for os/os.path support,
# but source-backed module loading and pure pattern escaping are live now.

---
# The module exposes Python's public glob helpers as callable functions.
load("assert.star", "assert")
load("glob.star", "glob")

assert.eq(type(glob.glob), "function")
assert.eq(type(glob.iglob), "function")
assert.eq(type(glob.escape), "function")

---
# escape quotes glob metacharacters using Python's bracket escaping convention.
load("assert.star", "assert")
load("glob.star", "glob")

assert.eq(glob.escape("*.py"), "[*].py")
assert.eq(glob.escape("file?.[ch]"), "file[?].[[]ch]")
assert.eq(glob.escape("plain/name"), "plain/name")

---
# dir_fd and root_dir depend on lower-level filesystem policy that Dyson has not
# exposed yet, so they fail explicitly rather than silently doing the wrong thing.
load("glob.star", "glob")

glob.glob("*.py", dir_fd=1) ### "glob.iglob: dir_fd is not supported"

---
load("glob.star", "glob")

glob.glob("*.py", root_dir="src") ### "glob.iglob: root_dir is not supported"
