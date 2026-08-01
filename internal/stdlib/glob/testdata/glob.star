# Tests for Dyson's Python-like glob compatibility module.
#
# Filesystem-backed glob expansion uses Dyson's narrow os/os.path primitives while
# tests exercise the public module through Starlark.

---
# The module exposes Python's public glob helpers as callable builtins.
load("assert.star", "assert")
load("glob.star", "glob")

assert.eq(type(glob.glob), "builtin_function_or_method")
assert.eq(type(glob.iglob), "builtin_function_or_method")
assert.eq(type(glob.escape), "builtin_function_or_method")

---
# escape quotes glob metacharacters using Python's bracket escaping convention.
load("assert.star", "assert")
load("glob.star", "glob")

assert.eq(glob.escape("*.py"), "[*].py")
assert.eq(glob.escape("file?.[ch]"), "file[?].[[]ch]")
assert.eq(glob.escape("plain/name"), "plain/name")

---
# Filesystem-backed glob expansion uses os.listdir, os.path.lexists, and
# os.path.isdir while preserving hidden-file and recursive matching behavior.
load("assert.star", "assert")
load("glob.star", "glob")

assert.eq(glob.glob(root + sep + "*.txt"), [root + sep + "a.txt"])
assert.eq(glob.glob(root + sep + "**" + sep + "*.txt", recursive=True), [root + sep + "a.txt", root + sep + "subdir" + sep + "nested.txt"])
assert.eq(glob.glob(root + sep + "*"), [root + sep + "a.txt", root + sep + "b.py", root + sep + "subdir"])
assert.eq(glob.glob(root + sep + "*", include_hidden=True), [root + sep + ".hidden", root + sep + "a.txt", root + sep + "b.py", root + sep + "subdir"])

---
# dir_fd and root_dir depend on lower-level filesystem policy that Dyson has not
# exposed yet, so they fail explicitly rather than silently doing the wrong thing.
load("glob.star", "glob")

glob.glob("*.py", dir_fd=1) ### "glob.iglob: dir_fd is not supported"

---
load("glob.star", "glob")

glob.glob("*.py", root_dir="src") ### "glob.iglob: root_dir is not supported"
