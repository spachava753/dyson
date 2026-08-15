# Tests for Dyson's basic Python-like JSON compatibility module.

---
# The module intentionally exposes only the two initially supported functions.
load("assert.star", "assert")
load("json.star", "json")

assert.eq(dir(json), ["dumps", "load"])
assert.eq(type(json.dumps), "builtin_function_or_method")
assert.eq(type(getattr(json, "load")), "builtin_function_or_method")

---
# dumps encodes JSON scalar values.
load("assert.star", "assert")
load("json.star", "json")

assert.eq(json.dumps(None), "null")
assert.eq(json.dumps(True), "true")
assert.eq(json.dumps(False), "false")
assert.eq(json.dumps(12345678901234567890), "12345678901234567890")
assert.eq(json.dumps(1.5), "1.5")
assert.eq(json.dumps("line\nvalue"), '"line\\nvalue"')

---
# dumps recursively encodes sequences and string-keyed dictionaries.
load("assert.star", "assert")
load("json.star", "json")

value = {
    "b": 2,
    "a": [True, None, ("nested", 3)],
}
assert.eq(json.dumps(value), '{"a":[true,null,["nested",3]],"b":2}')

---
# load invokes a text reader and reconstructs ordinary Starlark values.
load("assert.star", "assert")
load("json.star", "json")

def read_text():
    return '{"name":"dyson","numbers":[1,2.5],"enabled":true,"missing":null}'

value = getattr(json, "load")(struct(read=read_text))
assert.eq(value, {
    "name": "dyson",
    "numbers": [1, 2.5],
    "enabled": True,
    "missing": None,
})
assert.eq(type(value), "dict")
assert.eq(type(value["numbers"]), "list")
assert.eq(type(value["numbers"][0]), "int")
assert.eq(type(value["numbers"][1]), "float")

---
# load also accepts bytes returned by a binary reader.
load("assert.star", "assert")
load("json.star", "json")

def read_bytes():
    return b'["bytes",3]'

assert.eq(getattr(json, "load")(struct(read=read_bytes)), ["bytes", 3])

---
# dumps rejects values that the basic codec cannot represent as JSON objects.
load("json.star", "json")

json.dumps({1: "number key"})  ### "json.dumps: dict has int key, want string"

---
# load reports malformed input through its public function name.
load("json.star", "json")

def read_invalid():
    return "{"

getattr(json, "load")(struct(read=read_invalid))  ### "json.load: at offset 1, unexpected end of file"

---
# load requires a file-like value with a callable read method.
load("json.star", "json")

getattr(json, "load")(1)  ### "json.load: int has no read method"

---
# load rejects non-callable read attributes.
load("json.star", "json")

getattr(json, "load")(struct(read="not callable"))  ### "json.load: fp.read is string, want callable"

---
# load requires read to produce text or bytes.
load("json.star", "json")

def read_list():
    return []

getattr(json, "load")(struct(read=read_list))  ### "json.load: fp.read\\(\\) returned list, want string or bytes"

---
# Optional CPython encoder configuration is outside the initial surface.
load("json.star", "json")

json.dumps({}, indent=2)  ### "json.dumps: unexpected keyword argument"
