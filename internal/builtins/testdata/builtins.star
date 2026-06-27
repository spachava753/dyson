# Tests for Dyson's Python-like builtins.
#
# This file is chunked by lines containing "---". Each chunk executes as an
# independent Starlark file so related assertions stay small and failures point
# at a focused behavior area. Tests assert the desired Python-like builtins behavior.

---
# abs returns the absolute value of ints and floats.
load("assert.star", "assert")

assert.eq(abs(-1), 1)
assert.eq(abs(1), 1)
assert.eq(abs(-1.0), 1.0)
assert.eq(abs(1.0), 1.0)

---
# abs rejects non-numeric values.
load("assert.star", "assert")

abs("1") ### "abs: bad operand type for abs\\(\\): string"

---
# range returns a Starlark list with Python range semantics.
load("assert.star", "assert")

assert.eq(range(5), [0, 1, 2, 3, 4])
assert.eq(range(1, 5), [1, 2, 3, 4])
assert.eq(range(1, 6, 2), [1, 3, 5])
assert.eq(range(5, 1, -2), [5, 3])
assert.eq(range(3, 3), [])

---
# range validates integer arguments and non-zero step.
load("assert.star", "assert")

range(1, 5, 0) ### "range: step argument must not be zero"

---
# range rejects non-integer bounds.
load("assert.star", "assert")

range("5") ### "range: stop must be int, got string"

---
# reversed returns a list containing iterable values in reverse order.
load("assert.star", "assert")

assert.eq(reversed([1, 2, 3]), [3, 2, 1])
assert.eq(reversed(("a", "b")), ["b", "a"])
assert.eq(reversed("abc"), ["c", "b", "a"])

---
# reversed rejects non-iterables.
load("assert.star", "assert")

reversed(123) ### "reversed: int object is not iterable"

---
# round supports ints, floats, and optional ndigits.
load("assert.star", "assert")

assert.eq(round(3), 3)
assert.eq(round(3.2), 3)
assert.eq(round(3.8), 4)
assert.eq(round(3.14159, 2), 3.14)
assert.eq(round(10, 2), 10)

---
# round validates number and ndigits types.
load("assert.star", "assert")

round("3") ### "round: number must be int or float, got string"

---
# round rejects non-integer ndigits.
load("assert.star", "assert")

round(3.14, "2") ### "round: ndigits must be int, got string"

---
# sorted returns a sorted list and accepts key and reverse.
load("assert.star", "assert")

assert.eq(sorted([3, 1, 2]), [1, 2, 3])
assert.eq(sorted(("bbb", "a", "cc"), key=len), ["a", "cc", "bbb"])
assert.eq(sorted([1, 3, 2], reverse=True), [3, 2, 1])

---
# sorted rejects non-iterables.
load("assert.star", "assert")

sorted(1) ### "sorted: int object is not iterable"

---
# sorted rejects non-callable key values.
load("assert.star", "assert")

sorted([1], key=1) ### "sorted: key must be callable or None, got int"

---
# sum adds iterable values with an optional start value.
load("assert.star", "assert")

assert.eq(sum([1, 2, 3]), 6)
assert.eq(sum((1.5, 2.5)), 4.0)
assert.eq(sum([1, 2], 10), 13)

---
# sum rejects non-iterables.
load("assert.star", "assert")

sum(1) ### "sum: int object is not iterable"

---
# sum surfaces addition errors for incompatible values.
load("assert.star", "assert")

sum([1, "x"]) ### "sum: unknown binary op: int \\+ string"

---
# min and max work with iterable input, positional input, key, and default.
load("assert.star", "assert")

assert.eq(min([3, 1, 2]), 1)
assert.eq(max([3, 1, 2]), 3)
assert.eq(min(3, 1, 2), 1)
assert.eq(max(3, 1, 2), 3)
assert.eq(min(["bbb", "a", "cc"], key=len), "a")
assert.eq(max(["bbb", "a", "cc"], key=len), "bbb")
assert.eq(min([], default="empty"), "empty")
assert.eq(max([], default="empty"), "empty")

---
# min rejects empty iterables without a default.
load("assert.star", "assert")

min([]) ### "min: arg is an empty sequence"

---
# max rejects empty iterables without a default.
load("assert.star", "assert")

max([]) ### "max: arg is an empty sequence"

---
# min rejects default with multiple positional arguments.
load("assert.star", "assert")

min(1, 2, default=0) ### "min: default can only be used with a single iterable argument"

---
# min and max reject non-callable key values.
load("assert.star", "assert")

max([1], key=1) ### "max: key must be callable or None, got int"

---
# bin and oct format integers with Python prefixes.
load("assert.star", "assert")

assert.eq(bin(10), "0b1010")
assert.eq(bin(-10), "-0b1010")
assert.eq(oct(10), "0o12")
assert.eq(oct(-10), "-0o12")

---
# bin rejects non-integers.
load("assert.star", "assert")

bin(1.5) ### "bin: number must be int, got float"

---
# oct rejects non-integers.
load("assert.star", "assert")

oct("8") ### "oct: number must be int, got string"

---
# ord converts a one-character string to its code point.
load("assert.star", "assert")

assert.eq(ord("A"), 65)
assert.eq(ord("☃"), 9731)

---
# ord rejects strings with length other than one.
load("assert.star", "assert")

ord("ab") ### "ord: expected string of length 1"

---
# ord rejects non-string values.
load("assert.star", "assert")

ord(65) ### "ord: expected string of length 1, got int"

---
# chr converts code points to strings.
load("assert.star", "assert")

assert.eq(chr(65), "A")
assert.eq(chr(9731), "☃")

---
# chr rejects out-of-range code points.
load("assert.star", "assert")

chr(0x110000) ### "chr: arg not in range\\(0x110000\\)"

---
# chr rejects non-integers.
load("assert.star", "assert")

chr("65") ### "chr: i must be int, got string"

---
# pow supports integer, float, and modular exponentiation.
load("assert.star", "assert")

assert.eq(pow(2, 3), 8)
assert.eq(pow(2, -1), 0.5)
assert.eq(pow(2.0, 3), 8.0)
assert.eq(pow(2, 5, 5), 2)

---
# pow rejects zero modulus.
load("assert.star", "assert")

pow(2, 3, 0) ### "pow: 3rd argument cannot be 0"

---
# pow rejects modulus with non-integer base or exponent.
load("assert.star", "assert")

pow(2.0, 3, 5) ### "pow: pow\\(\\) 3rd argument not allowed unless all arguments are integers"

---
# pow rejects negative exponents with modulus.
load("assert.star", "assert")

pow(2, -1, 5) ### "pow: exponent must be non-negative when modulus is present"

---
# enumerate returns index/value tuples and accepts a start index.
load("assert.star", "assert")

assert.eq(enumerate(["a", "b"]), [(0, "a"), (1, "b")])
assert.eq(enumerate(("a", "b"), start=5), [(5, "a"), (6, "b")])
assert.eq(enumerate("ab"), [(0, "a"), (1, "b")])

---
# enumerate rejects non-iterables.
load("assert.star", "assert")

enumerate(1) ### "enumerate: int object is not iterable"

---
# enumerate validates start type.
load("assert.star", "assert")

enumerate([], start="0") ### "enumerate: start must be int, got string"

---
# open reads text from a relative path.
load("assert.star", "assert")

f = open(TEST_OPEN_RELATIVE)
assert.eq(type(f), "file")
assert.eq(f.read(), "relative file\n")
assert.eq(f.closed, False)
assert.eq(f.close(), None)
assert.eq(f.closed, True)

---
# open reads text from an absolute path.
load("assert.star", "assert")

f = open(TEST_OPEN_ABSOLUTE, "r")
assert.eq(f.read(), "absolute file\n")
f.close()

---
# open reads binary files as bytes without coercing contents to string.
load("assert.star", "assert")

f = open(TEST_OPEN_BINARY, "rb")
assert.eq(f.read(), b"\x00\x01\x02\xff")
f.close()

---
# open rejects malformed paths before touching the filesystem.
load("assert.star", "assert")

open(TEST_OPEN_MALFORMED) ### "open: invalid path"

---
# open validates path argument type.
load("assert.star", "assert")

open(123) ### "open: for parameter file: got int, want string"

---
# open validates supported modes.
load("assert.star", "assert")

open(TEST_OPEN_RELATIVE, "w") ### "open: unsupported mode"

---
# file.read rejects reads after close.
load("assert.star", "assert")

f = open(TEST_OPEN_RELATIVE)
f.close()
f.read() ### "file.read: I/O operation on closed file"
