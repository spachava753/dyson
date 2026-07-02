# restore primitives
a = "hello"
b = 1
c = 1.0
d = True
e = None
---
# restore primitive values derived from expressions
x = 1
y = 2
z = x + y
---
load("assert.star", "assert")

assert.eq(a, "hello")
assert.eq(b, 1)
assert.eq(c, 1.0)
assert.eq(d, True)
assert.eq(e, None)

assert.eq(x, 1)
assert.eq(y, 2)
assert.eq(z, 3)