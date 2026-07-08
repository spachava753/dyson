load("assert.star", "assert")
load("os.star", "os")
load("tempfile.star", "tempfile")

assert.eq(tempfile.gettempprefix(), "tmp")
assert.eq(tempfile.gettempprefixb(), b"tmp")
assert.eq(tempfile.gettempdir(), ".")
assert.eq(tempfile.gettempdirb(), b".")
assert.eq(tempfile.template, "tmp")
assert.true(tempfile.TMP_MAX > 0)

---

load("assert.star", "assert")
load("os.star", "os")
load("tempfile.star", "tempfile")

path = tempfile.mkdtemp(prefix="dyson-", suffix="-dir")
assert.true(path.startswith("dyson-"))
assert.true(path.endswith("-dir"))
assert.true(os.path.isdir(path))

---

load("assert.star", "assert")
load("os.star", "os")
load("tempfile.star", "tempfile")

fd, name = tempfile.mkstemp(prefix="dyson-", suffix=".txt")
assert.true(name.startswith("dyson-"))
assert.true(name.endswith(".txt"))
assert.true(os.path.isfile(name))
assert.eq(os.write(fd, b"hello"), 5)
os.close(fd)

read_fd = os.open(name, os.O_RDONLY)
assert.eq(os.read(read_fd, 5), b"hello")
os.close(read_fd)

---

load("assert.star", "assert")
load("os.star", "os")
load("tempfile.star", "tempfile")

candidate = tempfile.mktemp(prefix="dyson-", suffix=".tmp")
assert.true(candidate.startswith("dyson-"))
assert.true(candidate.endswith(".tmp"))
assert.eq(os.path.exists(candidate), False)

---

load("assert.star", "assert")
load("os.star", "os")
load("tempfile.star", "tempfile")

tfd, named = tempfile.NamedTemporaryFile(prefix="named-")
assert.true(os.path.isfile(named))
os.close(tfd)

tmpfd = tempfile.TemporaryFile(prefix="anon-")
os.close(tmpfd)

tmpdir = tempfile.TemporaryDirectory(prefix="td-")
assert.true(os.path.isdir(tmpdir))

---

load("tempfile.star", "tempfile")

tempfile.SpooledTemporaryFile()  ### "tempfile.SpooledTemporaryFile: not supported"
