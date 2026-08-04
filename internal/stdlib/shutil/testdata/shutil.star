# Tests for Dyson's Python-like shutil compatibility module.
#
# These chunks cover Go builtin orchestration over the injected os module and
# host-facing Afero/xos capabilities from the Go harness.

---
# The module exposes each implemented shutil function directly as a host builtin.
load("assert.star", "assert")
load("shutil.star", "shutil")

assert.eq(type(shutil.copyfile), "builtin_function_or_method")
assert.eq(type(shutil.copymode), "builtin_function_or_method")
assert.eq(type(shutil.copystat), "builtin_function_or_method")
assert.eq(type(shutil.copy), "builtin_function_or_method")
assert.eq(type(shutil.copy2), "builtin_function_or_method")
assert.eq(type(shutil.copytree), "builtin_function_or_method")
assert.eq(type(shutil.rmtree), "builtin_function_or_method")
assert.eq(type(shutil.move), "builtin_function_or_method")
assert.eq(type(shutil.which), "builtin_function_or_method")
assert.eq(type(shutil.disk_usage), "builtin_function_or_method")
assert.eq(type(shutil.chown), "builtin_function_or_method")
assert.eq(type(shutil.get_terminal_size), "builtin_function_or_method")
assert.eq(type(shutil.ignore_patterns), "builtin_function_or_method")

---
# copyfile, copy, and copy2 copy file contents through afero.Fs.
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
# copytree invokes an explicitly supplied Starlark copy callback for each file.
load("assert.star", "assert")
load("os.star", "os")
load("shutil.star", "shutil")

copied = []
def custom_copy(src, dst):
    copied.append((src, dst))
    return shutil.copyfile(src, dst)

assert.eq(shutil.copytree("tree", "tree-callback-copy", copy_function=custom_copy), "tree-callback-copy")
assert.eq(len(copied), 3)
assert.eq(os.path.isfile("tree-callback-copy/sub/b.txt"), True)

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
# disk_usage returns a plain tuple of capacity numbers.
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
