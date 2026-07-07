# replay durable values returned by Dyson's signal stdlib module
load("signal.star", "signal")

sigint = signal.Signals(signal.SIGINT)
sigterm = signal.Signals(signal.SIGTERM)
valid = signal.valid_signals()
text = signal.strsignal(signal.SIGINT)
---
load("assert.star", "assert")

assert.eq(type(sigint), "signal.Signals")
assert.eq(sigint.name, "SIGINT")
assert.eq(sigint.value, 2)
assert.eq(str(sigint), "Signals.SIGINT")
assert.eq(sigint, signal.SIGINT)
assert.eq(type(sigterm), "signal.Signals")
assert.eq(sigterm.name, "SIGTERM")
assert.eq(sigterm.value, 15)
assert.true(signal.SIGINT in valid, "SIGINT valid")
assert.true(signal.SIGTERM in valid, "SIGTERM valid")
assert.eq(text, "Interrupt")
