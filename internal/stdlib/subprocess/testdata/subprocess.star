# Tests for Dyson's Python-like subprocess compatibility module.

---
# The module exposes the core subprocess namespace and compatibility constants.
load("assert.star", "assert")
load("subprocess.star", "subprocess")

assert.eq(type(subprocess.run), "builtin_function_or_method")
assert.eq(type(subprocess.getoutput), "builtin_function_or_method")
assert.eq(type(subprocess.getstatusoutput), "builtin_function_or_method")
assert.eq(type(subprocess.CompletedProcess), "builtin_function_or_method")
assert.eq(subprocess.PIPE, -1)
assert.eq(subprocess.STDOUT, -2)
assert.eq(subprocess.DEVNULL, -3)

---
# CompletedProcess stores args, returncode, and optional captured streams.
load("assert.star", "assert")
load("subprocess.star", "subprocess")

completed = subprocess.CompletedProcess(["cmd"], 0, stdout="out", stderr=None)
assert.eq(type(completed), "subprocess.CompletedProcess")
assert.eq(completed.args, ["cmd"])
assert.eq(completed.returncode, 0)
assert.eq(completed.stdout, "out")
assert.eq(completed.stderr, None)
assert.eq(completed.check_returncode(), None)
assert.eq(str(completed), 'CompletedProcess(args=["cmd"], returncode=0, stdout="out")')

---
# Non-zero CompletedProcess.check_returncode aborts with a stable message.
load("subprocess.star", "subprocess")

subprocess.CompletedProcess(["cmd"], 9).check_returncode()  ### "subprocess.CompletedProcess.check_returncode: command exited with status 9"

---
# run captures binary stdout/stderr when capture_output is requested.
load("assert.star", "assert")
load("subprocess.star", "subprocess")

completed = subprocess.run(["echo", "hello"], capture_output=True)
assert.eq(type(completed), "subprocess.CompletedProcess")
assert.eq(completed.args, ["echo", "hello"])
assert.eq(completed.returncode, 0)
assert.eq(completed.stdout, b"argv:echo|hello\n")
assert.eq(completed.stderr, b"")

---
# Default stdout/stderr inherit parent streams, so CompletedProcess does not store them.
load("assert.star", "assert")
load("subprocess.star", "subprocess")

completed = subprocess.run(["echo", "default"])
assert.eq(completed.returncode, 0)
assert.eq(completed.stdout, None)
assert.eq(completed.stderr, None)

---
# stderr=STDOUT only stores merged output when stdout itself is captured.
load("assert.star", "assert")
load("subprocess.star", "subprocess")

inherited = subprocess.run(["fail"], stderr=subprocess.STDOUT)
assert.eq(inherited.returncode, 7)
assert.eq(inherited.stdout, None)
assert.eq(inherited.stderr, None)

discarded = subprocess.run(["fail"], stdout=subprocess.DEVNULL, stderr=subprocess.STDOUT)
assert.eq(discarded.returncode, 7)
assert.eq(discarded.stdout, None)
assert.eq(discarded.stderr, None)

merged = subprocess.run(["fail"], stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
assert.eq(merged.returncode, 7)
assert.eq(merged.stdout, b"bad\nerr\n")
assert.eq(merged.stderr, None)

---
# run supports text output, explicit env/cwd, and stdin input.
load("assert.star", "assert")
load("subprocess.star", "subprocess")

context = subprocess.run(["context"], capture_output=True, text=True, cwd="work", env={"NAME": "dyson"})
assert.eq(context.stdout, "cwd=work env=NAME=dyson\n")

binary_input = subprocess.run(["input"], input=b"abc", stdout=subprocess.PIPE)
assert.eq(binary_input.stdout, b"abc")

text_input = subprocess.run(["input"], input="abc", stdout=subprocess.PIPE, text=True)
assert.eq(text_input.stdout, "abc")

---
# Shell helpers merge stderr into stdout and trim trailing newlines like Python.
load("assert.star", "assert")
load("subprocess.star", "subprocess")

assert.eq(subprocess.getstatusoutput("status output"), (5, "status output"))
assert.eq(subprocess.getoutput("plain output"), "plain output")
assert.eq(subprocess.getoutput("double newline"), "a\n")

shell_completed = subprocess.run("echo shell", shell=True, capture_output=True, text=True)
assert.eq(shell_completed.stdout, "shell:echo shell\n")

shell_sequence = subprocess.run(["echo \"$0:$1\"", "NAME", "ARG"], shell=True, capture_output=True, text=True)
assert.eq(shell_sequence.stdout, "NAME:ARG\n")

---
# check=True reports non-zero return codes as Starlark-visible errors.
load("subprocess.star", "subprocess")

subprocess.run(["fail"], capture_output=True, check=True)  ### "subprocess.run: command exited with status 7"

---
# input and explicit stdin are mutually exclusive, matching Python's run contract.
load("subprocess.star", "subprocess")

subprocess.run(["input"], input=b"abc", stdin=subprocess.PIPE)  ### "subprocess.run: stdin and input may not both be used"

---
# Unsupported stdin values fail instead of being ignored.
load("subprocess.star", "subprocess")

subprocess.run(["echo"], stdin=123)  ### "subprocess.run: stdin must be PIPE, DEVNULL, or None"

---
# Timeout handling is intentionally unsupported until command cancellation exists.
load("subprocess.star", "subprocess")

subprocess.run(["echo"], timeout=1)  ### "subprocess.run: timeout is not supported"
