# same list appears twice in list
shared = [1]
root = [shared, shared]
---
load("assert.star", "assert")

root[0].append(2)
assert.eq(root[1], [1, 2])
---
# same list appears as two dict values
shared = ["start"]
root = {"a": shared, "b": shared}
---
load("assert.star", "assert")

root["a"].append("mutated")
assert.eq(root["b"], ["start", "mutated"])
---
# same list appears through dict and nested list
shared = [1]
root = {"direct": shared, "wrapped": [shared]}
---
load("assert.star", "assert")

root["direct"].append(2)
assert.eq(root["wrapped"][0], [1, 2])
---
# tuple preserves aliases to mutable members
shared = ["x"]
root = (shared, shared)
---
load("assert.star", "assert")

root[0].append("y")
assert.eq(root[1], ["x", "y"])
