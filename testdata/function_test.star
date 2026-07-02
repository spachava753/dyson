# restore function results
def add(x, y):
    return x + y

a = add(1, 2)
---
b = add(a, 3)
---
load("assert.star", "assert")

assert.eq(a, 3)
assert.eq(b, 6)
