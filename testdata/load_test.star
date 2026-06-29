load("assert.star", "assert")
load("testdata/load.star", "adder")

assert.eq(adder(1, 2), 3)