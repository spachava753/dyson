# restore up until first fail
x = 1
y = 2
z = x + y

fail() ### "fail:"
z = 4
fail()
---
load("assert.star", "assert")

assert.eq(x, 1)
assert.eq(y, 2)
assert.eq(z, 3)