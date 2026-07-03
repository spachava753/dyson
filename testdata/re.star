# replay durable values returned by Dyson's re stdlib module
load("re.star", "re")

pattern = re.compile("(?P<word>[A-Za-z]+)-(\\d+)", re.I)
---
match = re.search("(?P<word>[A-Za-z]+)-(\\d+)", "xx Abc-123 yy")
missing = re.search("z+", "abc")
---
iter_matches = re.finditer("([a-z]+)", "one two")
subbed = re.sub("([a-z]+)", "<\\1>", "one two")
---
load("assert.star", "assert")

assert.eq(type(pattern), "re.Pattern")
assert.eq(pattern.pattern, "(?P<word>[A-Za-z]+)-(\\d+)")
assert.eq(pattern.flags, re.I)
assert.eq(pattern.groups, 2)
assert.eq(pattern.groupindex["word"], 1)

pattern_match = pattern.search("xx Abc-123 yy")
assert.eq(type(pattern_match), "re.Match")
assert.eq(pattern_match.group(0), "Abc-123")
assert.eq(pattern_match.group("word"), "Abc")
assert.eq(pattern_match.group(2), "123")

assert.eq(type(match), "re.Match")
assert.eq(match.group(0), "Abc-123")
assert.eq(match.group("word"), "Abc")
assert.eq(match.groups(), ("Abc", "123"))
assert.eq(match.groupdict(), {"word": "Abc"})
assert.eq(match.span(), (3, 10))
assert.eq(match.start(), 3)
assert.eq(match.end(), 10)
assert.eq(match.pos, 0)
assert.eq(match.endpos, 13)
assert.eq(match.re.pattern, pattern.pattern)
assert.eq(match.string, "xx Abc-123 yy")
assert.eq(missing, None)

assert.eq(len(iter_matches), 2)
assert.eq(iter_matches[0].group(0), "one")
assert.eq(iter_matches[0].group(1), "one")
assert.eq(iter_matches[1].group(0), "two")
assert.eq(iter_matches[1].span(), (4, 7))
assert.eq(subbed, "<one> <two>")
