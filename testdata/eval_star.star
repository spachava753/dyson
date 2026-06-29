load("assert.star", "assert")

x = 1
y = 0
assert.eq(x, 1)
assert.eq(y, 0)

x += 1
y = y + 1
assert.eq(x, 2)
assert.eq(y, 1)

print(x)
print(y)

# asserts that we have 
assert.eq(min(x, y), 1)
print(min(x, y))
