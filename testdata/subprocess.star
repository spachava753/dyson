# replay durable values returned by Dyson's subprocess stdlib module
load("subprocess.star", "subprocess")

constructed = subprocess.CompletedProcess(["manual"], 0, stdout="ok", stderr=None)
---
failed = subprocess.CompletedProcess(["manual"], 3, stdout=None, stderr="bad")
---
load("assert.star", "assert")

assert.eq(type(constructed), "subprocess.CompletedProcess")
assert.eq(constructed.args, ["manual"])
assert.eq(constructed.returncode, 0)
assert.eq(constructed.stdout, "ok")
assert.eq(constructed.stderr, None)
assert.eq(constructed.check_returncode(), None)

assert.eq(type(failed), "subprocess.CompletedProcess")
assert.eq(failed.args, ["manual"])
assert.eq(failed.returncode, 3)
assert.eq(failed.stdout, None)
assert.eq(failed.stderr, "bad")
