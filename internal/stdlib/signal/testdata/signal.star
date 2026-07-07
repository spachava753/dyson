# Tests for Dyson's Python-like signal compatibility module.

---
# The module exposes pure signal helpers, enum-like constructors, and common constants.
load("assert.star", "assert")
load("signal.star", "signal")

assert.eq(type(signal.strsignal), "builtin_function_or_method")
assert.eq(type(signal.valid_signals), "builtin_function_or_method")
assert.eq(type(signal.Signals), "builtin_function_or_method")
assert.eq(type(signal.Handlers), "builtin_function_or_method")
assert.eq(type(signal.SIGINT), "signal.Signals")
assert.eq(type(signal.SIG_DFL), "signal.Handlers")
assert.eq(signal.SIGHUP.value, 1)
assert.eq(signal.SIGINT.value, 2)
assert.eq(signal.SIGQUIT.value, 3)
assert.eq(signal.SIGKILL.value, 9)
assert.eq(signal.SIGTERM.value, 15)
assert.eq(signal.SIGIOT, signal.SIGABRT)
assert.eq(signal.SIG_DFL.value, 0)
assert.eq(signal.SIG_IGN.value, 1)

---
# Signals constructs IntEnum-like values with Python-visible name and value attributes.
load("assert.star", "assert")
load("signal.star", "signal")

sigint = signal.Signals(signal.SIGINT)
assert.eq(type(sigint), "signal.Signals")
assert.eq(sigint.name, "SIGINT")
assert.eq(sigint.value, 2)
assert.eq(str(sigint), "Signals.SIGINT")
assert.eq(sigint, signal.SIGINT)
assert.eq(sigint.value, 2)

sigterm = signal.Signals(15)
assert.eq(sigterm.name, "SIGTERM")
assert.eq(sigterm.value, signal.SIGTERM.value)

---
# Handlers constructs the SIG_DFL and SIG_IGN enum values CPython exposes.
load("assert.star", "assert")
load("signal.star", "signal")

assert.eq(signal.Handlers(signal.SIG_DFL).name, "SIG_DFL")
assert.eq(signal.Handlers(1), signal.SIG_IGN)
assert.eq(str(signal.SIG_IGN), "Handlers.SIG_IGN")

---
# strsignal accepts ints and Signals values and rejects out-of-range numbers.
load("assert.star", "assert")
load("signal.star", "signal")

assert.eq(signal.strsignal(signal.SIGINT), "Interrupt")
assert.eq(signal.strsignal(signal.SIGTERM), "Terminated")

---
# valid_signals returns the supported Signals values as a set.
load("assert.star", "assert")
load("signal.star", "signal")

signals = signal.valid_signals()
assert.eq(type(signals), "set")
assert.true(signal.SIGINT in signals, "SIGINT should be valid")
assert.true(signal.SIGTERM in signals, "SIGTERM should be valid")
assert.true(signal.SIGKILL in signals, "SIGKILL should be valid")
assert.eq(0 in signals, False)

---
# Signals rejects unknown signal numbers with a CPython-like message.
load("signal.star", "signal")

signal.Signals(999)  ### "signal.Signals: 999 is not a valid Signals"

---
# strsignal rejects unknown signal numbers like CPython does on POSIX.
load("signal.star", "signal")

signal.strsignal(999)  ### "signal.strsignal: signal number out of range"

---
# Signal numbers must fit in Go int for host portability.
load("signal.star", "signal")

signal.strsignal(999999999999999999999999999999999999999)  ### "signal.strsignal: signalnum is out of range"
