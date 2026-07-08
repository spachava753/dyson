# Tests for Dyson's Python-like shutil compatibility module.
#
# These chunks cover source-backed orchestration over the injected os module and
# host-facing primitives that use xfs/xos capabilities from the Go harness.

---
# The module exposes implemented shutil functions as Starlark functions or host builtins.
load("assert.star", "assert")
load("shutil.star", "shutil")

assert.eq(type(shutil.copyfile), "function")
assert.eq(type(shutil.copy), "function")
assert.eq(type(shutil.copy2), "function")
assert.eq(type(shutil.copytree), "function")
assert.eq(type(shutil.rmtree), "function")
assert.eq(type(shutil.move), "function")
assert.eq(type(shutil.which), "function")
assert.eq(type(shutil.disk_usage), "function")
assert.eq(type(shutil.chown), "function")
assert.eq(type(shutil.get_terminal_size), "function")
assert.eq(type(shutil.ignore_patterns), "function")

---
# copyfile, copy, and copy2 copy file contents through the injected xfs.OpenFS.
load("assert.star", "assert")
load("os.star", "os")
load("shutil.star", "shutil")

assert.eq(shutil.copyfile("src.txt", "copyfile.txt"), "copyfile.txt")
fd = os.open("copyfile.txt", os.O_RDONLY)
assert.eq(os.read(fd, 20), b"hello")
os.close(fd)

os.mkdir("copies")
assert.eq(shutil.copy("src.txt", "copies"), "copies/src.txt")
read_fd = os.open("copies/src.txt", os.O_RDONLY)
assert.eq(os.read(read_fd, 20), b"hello")
os.close(read_fd)

assert.eq(shutil.copy2("src.txt", "copy2.txt"), "copy2.txt")
assert.eq(os.path.getsize("copy2.txt"), 5)

---
# copytree recursively copies directories and ignore_patterns can skip names.
load("assert.star", "assert")
load("os.star", "os")
load("shutil.star", "shutil")

ignore = shutil.ignore_patterns("*.tmp")
assert.eq(shutil.copytree("tree", "tree-copy", ignore=ignore), "tree-copy")
assert.eq(os.path.isfile("tree-copy/a.txt"), True)
assert.eq(os.path.isfile("tree-copy/sub/b.txt"), True)
assert.eq(os.path.exists("tree-copy/skip.tmp"), False)

---
# rmtree removes nested directory trees.
load("assert.star", "assert")
load("os.star", "os")
load("shutil.star", "shutil")

assert.eq(os.path.isdir("tree-copy"), True)
assert.eq(shutil.rmtree("tree-copy"), None)
assert.eq(os.path.exists("tree-copy"), False)
assert.eq(shutil.rmtree("missing", ignore_errors=True), None)

---
# move renames files and returns the final destination path.
load("assert.star", "assert")
load("os.star", "os")
load("shutil.star", "shutil")

assert.eq(shutil.move("move-src.txt", "move-dst.txt"), "move-dst.txt")
assert.eq(os.path.exists("move-src.txt"), False)
assert.eq(os.path.isfile("move-dst.txt"), True)

os.mkdir("move-dir")
assert.eq(shutil.move("move-dst.txt", "move-dir"), "move-dir/move-dst.txt")
assert.eq(os.path.isfile("move-dir/move-dst.txt"), True)

---
# which searches explicit path entries and terminal size honors environment overrides.
load("assert.star", "assert")
load("shutil.star", "shutil")

assert.eq(shutil.which("tool", path="bin"), "bin/tool")
assert.eq(shutil.which("missing", path="bin"), None)
assert.eq(shutil.get_terminal_size(), (120, 40))

---
# disk_usage returns a plain durable tuple of capacity numbers.
load("assert.star", "assert")
load("shutil.star", "shutil")

usage = shutil.disk_usage(".")
assert.eq(type(usage), "tuple")
assert.eq(len(usage), 3)
assert.eq(usage[0] >= usage[1], True)
assert.eq(usage[0] >= usage[2], True)

---
# Unsupported policy variants fail with stable module-qualified messages.
load("shutil.star", "shutil")

shutil.copyfile("src.txt", "x", follow_symlinks=False) ### "shutil.copyfile: follow_symlinks=False is not supported"
