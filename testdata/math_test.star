# basic test with math function builtins

# should restore values derived from module constants
load("math", "pi")
a = pi
---
load("math", "e")
b = e
---
# call a module builtin
load("math", "ceil")
c = ceil(1.5)
---
load("assert.star", "assert")
load("math", "pi", "e")

assert.eq(a, pi)
assert.eq(b, e)
assert.eq(c, 2)