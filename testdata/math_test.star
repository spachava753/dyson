# basic test with math function builtins

# should restore values derived from module constants
load("math.star", "math")
a = math.pi
---
b = math.e
---
# call a module builtin
c = math.ceil(1.5)
---
# call a module builtin
def add(x, y):
    return x + y
math.ceil = add ### "can't assign to .ceil field of module"
---
load("assert.star", "assert")

assert.eq(a, math.pi)
assert.eq(b, math.e)
assert.eq(c, 2)