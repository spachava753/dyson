# Host-backed os module behavior that needs the Afero host backend and xos.Host.

---
# Host filesystem primitives mutate only the test temp directory selected by Go.
load("assert.star", "assert")
load("os.star", "os")

os.mkdir("dir")
os.makedirs("dir/nested/leaf")
fd = os.open("dir/nested/leaf/file.txt", os.O_CREAT | os.O_RDWR | os.O_TRUNC, 0o666)
assert.eq(os.write(fd, b"hello world"), 11)
os.ftruncate(fd, 5)
os.fsync(fd)
os.close(fd)

fd2 = os.open("dir/nested/leaf/file.txt", os.O_RDONLY)
assert.eq(os.read(fd2, 10), b"hello")
os.close(fd2)
assert.eq(os.path.getsize("dir/nested/leaf/file.txt"), 5)

os.rename("dir/nested/leaf/file.txt", "dir/file.txt")
os.link("dir/file.txt", "dir/link.txt")
assert.eq(os.path.samefile("dir/file.txt", "dir/link.txt"), True)
os.symlink("dir/file.txt", "sym.txt")
assert.eq(os.path.islink("sym.txt"), True)
assert.eq(os.readlink("sym.txt"), "dir/file.txt")

os.replace("dir/link.txt", "dir/replaced.txt")
os.truncate("dir/replaced.txt", 2)
assert.eq(os.path.getsize("dir/replaced.txt"), 2)
os.utime("dir/replaced.txt", (1, 2))
assert.eq(os.path.getmtime("dir/replaced.txt"), 2.0)

os.renames("dir/replaced.txt", "new/parent/replaced.txt")
assert.eq(os.path.exists("new/parent/replaced.txt"), True)
os.remove("new/parent/replaced.txt")
os.removedirs("new/parent")
os.unlink("dir/file.txt")
os.rmdir("dir/nested/leaf")

assert.eq(os.path.abspath("dir").endswith("/dir"), True)
assert.eq(os.path.realpath("dir").endswith("/dir"), True)

---
# Environment and process primitives come from the configured xos host domain.
load("assert.star", "assert")
load("os.star", "os")

assert.eq(type(os.getpid()), "int")
assert.eq(type(os.getppid()), "int")
assert.eq(type(os.getuid()), "int")
assert.eq(type(os.geteuid()), "int")
assert.eq(type(os.getgid()), "int")
assert.eq(type(os.getegid()), "int")
assert.eq(type(os.getgroups()), "list")
assert.eq(os.getenv("DYSON_OS_TEST"), "before")
assert.eq(os.getenv("DYSON_OS_MISSING", "fallback"), "fallback")
assert.eq(os.environ()["DYSON_OS_TEST"], "before")
os.putenv("DYSON_OS_TEST", "after")
assert.eq(os.getenv("DYSON_OS_TEST"), "after")
os.unsetenv("DYSON_OS_TEST")
assert.eq(os.getenv("DYSON_OS_TEST"), None)
assert.eq(os.get_exec_path({"PATH": "a" + os.pathsep + "b"}), ["a", "b"])
