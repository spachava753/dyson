# restore nested list values
root1 = [[1], [1]]
---
# restore dict values
root2 = {"a": ["start"], "b": ["start"]}
---
# restore dict and nested list values
root3 = {"direct": [1], "wrapped": [[1]]}
---
# restore tuple values
root4 = (["x"], ["x"])
---
load("assert.star", "assert")

root1[0].append(2)
assert.eq(root1[0], [1, 2])
assert.eq(root1[1], [1])

root2["a"].append("mutated")
assert.eq(root2["a"], ["start", "mutated"])
assert.eq(root2["b"], ["start"])

root3["direct"].append(2)
assert.eq(root3["direct"], [1, 2])
assert.eq(root3["wrapped"][0], [1])

root4[0].append("y")
assert.eq(root4[0], ["x", "y"])
assert.eq(root4[1], ["x"])
