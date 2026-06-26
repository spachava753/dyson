# Tests for Dyson's Python-like re compatibility module.
#
# This file is chunked by lines containing "---". Each chunk executes as an
# independent Starlark file so related assertions stay small and failures point
# at a focused behavior area.

---
# Pattern objects expose Python-like core attributes.
load("assert.star", "assert")
load("re.star", "re")

pattern = re.compile("(?P<word>[a-z]+)", re.I | re.M)
assert.eq(type(pattern), "re.Pattern")
assert.eq(pattern.pattern, "(?P<word>[a-z]+)")
assert.eq(pattern.flags, re.I | re.M)
assert.eq(pattern.groups, 1)
assert.eq(pattern.groupindex, {"word": 1})

---
# Compiling an already compiled pattern returns the same value when no extra
# flags are supplied, matching Python's re.compile behavior.
load("assert.star", "assert")
load("re.star", "re")

pattern = re.compile("[a-z]+")
assert.true(re.compile(pattern) == pattern)

---
# Public flag aliases keep Python-compatible numeric values so generated code
# can combine short and long flag names interchangeably.
load("assert.star", "assert")
load("re.star", "re")

assert.true(re.A == re.ASCII)
assert.true(re.I == re.IGNORECASE)
assert.true(re.L == re.LOCALE)
assert.true(re.M == re.MULTILINE)
assert.true(re.S == re.DOTALL)
assert.true(re.U == re.UNICODE)
assert.true(re.X == re.VERBOSE)
assert.eq((re.NOFLAG, re.I, re.L, re.M, re.S, re.U, re.X, re.DEBUG, re.A), (0, 2, 4, 8, 16, 32, 64, 128, 256))

---
# purge exists for API compatibility. Dyson does not currently maintain a regex
# cache, so it is a no-op returning None.
load("assert.star", "assert")
load("re.star", "re")

assert.eq(re.purge(), None)

---
# Pattern values advertise the methods and attributes that user code can access
# with dot expressions.
load("assert.star", "assert")
load("re.star", "re")

pattern = re.compile("[a-z]+")
assert.true("search" in dir(pattern), "search")
assert.true("match" in dir(pattern), "match")
assert.true("fullmatch" in dir(pattern), "fullmatch")
assert.true("split" in dir(pattern), "split")
assert.true("findall" in dir(pattern), "findall")
assert.true("finditer" in dir(pattern), "finditer")
assert.true("sub" in dir(pattern), "sub")
assert.true("subn" in dir(pattern), "subn")
assert.true("pattern" in dir(pattern), "pattern")
assert.true("flags" in dir(pattern), "flags")
assert.true("groups" in dir(pattern), "groups")
assert.true("groupindex" in dir(pattern), "groupindex")

---
# re.Module-level search returns re.Match objects with Python-like group accessors,
# named groups, positional spans, and source metadata.
load("assert.star", "assert")
load("re.star", "re")

m = re.search("(?P<word>[a-z]+)-(\\d+)", "xx abc-123 yy")
assert.eq(m.group(0, 1, 2, "word"), ("abc-123", "abc", "123", "abc"))
assert.eq(m.groups("missing"), ("abc", "123"))
assert.eq(m.groupdict("missing"), {"word": "abc"})
assert.eq(m.span(2), (7, 10))
assert.eq((m.start(), m.end(), m.pos, m.endpos, m.lastindex, m.lastgroup, m.string), (3, 10, 0, 13, 2, None, "xx abc-123 yy"))

---
# Failed searches and non-prefix matches return None rather than raising.
load("assert.star", "assert")
load("re.star", "re")

assert.eq(re.search("z+", "abc"), None)
assert.eq(re.match("b", "abc"), None)
assert.eq(re.fullmatch("a.*e", "abcdef"), None)

---
# match anchors at the start, while fullmatch requires the whole input to match.
load("assert.star", "assert")
load("re.star", "re")

assert.eq(re.match("abc", "abcdef").group(), "abc")
assert.eq(re.fullmatch("a.*f", "abcdef").group(), "abcdef")

---
# IGNORECASE, DOTALL, MULTILINE, and VERBOSE affect matching in the expected
# Python-compatible places.
load("assert.star", "assert")
load("re.star", "re")

assert.eq(re.search("^abc.def$", "ABC\nDEF", re.I | re.S).group(), "ABC\nDEF")
assert.eq(re.findall("^a", "a\nba", re.M), ["a"])
assert.eq(re.search("a  # comment\n b", "xxab", re.X).group(), "ab")

---
# split includes captured separators and respects maxsplit.
load("assert.star", "assert")
load("re.star", "re")

assert.eq(re.split("(,)", "a,b,c", 1), ["a", ",", "b,c"])

---
# findall follows Python's result shape: full matches when there are no groups,
# strings for one group, and tuples for multiple groups.
load("assert.star", "assert")
load("re.star", "re")

assert.eq(re.findall("[a-z]+", "a1bc2"), ["a", "bc"])
assert.eq(re.findall("([a-z]+)", "a1bc2"), ["a", "bc"])
assert.eq(re.findall("([a-z]+)(\\d)", "a1bc2"), [("a", "1"), ("bc", "2")])

---
# finditer returns re.Match values, so callers can inspect spans for each match.
load("assert.star", "assert")
load("re.star", "re")

assert.eq([item.span() for item in re.finditer("[a-z]+", "a1bc2")], [(0, 1), (2, 4)])

---
# sub and subn support numeric and explicit group references in replacement
# strings. subn also reports how many replacements were made.
load("assert.star", "assert")
load("re.star", "re")

assert.eq(re.sub("([a-z]+)", "<\\1>", "a1bc2", 1), "<a>1bc2")
assert.eq(re.subn("([a-z]+)", "<\\g<1>>", "a1bc2"), ("<a>1<bc>2", 2))

---
# escape quotes regex metacharacters for literal matching.
load("assert.star", "assert")
load("re.star", "re")

assert.eq(re.escape("a.b[0]"), "a\\.b\\[0\\]")

---
# Compiled Pattern methods mirror the module-level operations.
load("assert.star", "assert")
load("re.star", "re")

pattern = re.compile("([a-z]+)")
assert.eq(pattern.search("12ab").group(1), "ab")
assert.eq(pattern.match("ab12").group(), "ab")
assert.eq(pattern.fullmatch("ab").group(), "ab")
assert.eq(pattern.findall("a1bc2"), ["a", "bc"])
assert.eq(pattern.sub("X", "a1bc2", 1), "X1bc2")
assert.eq(pattern.subn("X", "a1bc2"), ("X1X2", 2))

---
# Pattern.finditer honors pos/endpos while preserving spans relative to the
# original string, not the sliced search window.
load("assert.star", "assert")
load("re.star", "re")

assert.eq([item.span() for item in re.compile("[a-z]+").finditer("12ab34cd", 2, 6)], [(2, 4)])

---
# Bytes patterns and inputs return bytes for matched text.
load("assert.star", "assert")
load("re.star", "re")

assert.eq(re.compile(b"xx([a-z]+)").search(b"xxab").group(1), b"ab")

---
# Callable replacements receive a re.Match object and use its return value as the
# replacement text.
load("assert.star", "assert")
load("re.star", "re")

def repl(m):
    return "[" + m.group(1) + "]"

assert.eq(re.sub("([a-z]+)", repl, "a1bc2"), "[a]1[bc]2")

---
# Patterns must be strings or bytes.
load("assert.star", "assert")
load("re.star", "re")

re.compile(123) ### "re.compile: pattern must be str or bytes, got int"

---
# Passing flags with a compiled pattern is rejected, as in Python.
load("assert.star", "assert")
load("re.star", "re")

pattern = re.compile("x")
re.search(pattern, "x", re.I) ### "re.search: cannot process flags argument with a compiled pattern"

---
# LOCALE has Python-specific semantics that Dyson does not implement.
load("assert.star", "assert")
load("re.star", "re")

re.compile("x", re.L) ### "re.compile: LOCALE is not supported"

---
# Invalid regex syntax is reported as a module-qualified compile error.
load("assert.star", "assert")
load("re.star", "re")

re.compile("(") ### "re.compile:"
