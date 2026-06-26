// Package re implements Dyson's Starlark compatibility subset of Python's re
// module.
//
// # Loading
//
// In user code, import the module namespace explicitly:
//
//	load("re.star", "re")
//	match = re.search("\\d+", "order 98765")
//
// Go Starlark does not support bare load("re.star"); the namespace symbol must
// be named in the load statement. Direct symbol imports such as
// load("re.star", "search", "I") are intentionally unsupported.
//
// # Supported API
//
// The module supports the common Python re workflows built around module-level
// helpers and compiled patterns:
//
//   - compile(pattern, flags=0)
//   - search(pattern, string, flags=0)
//   - match(pattern, string, flags=0)
//   - fullmatch(pattern, string, flags=0)
//   - split(pattern, string, maxsplit=0, flags=0)
//   - findall(pattern, string, flags=0)
//   - finditer(pattern, string, flags=0)
//   - sub(pattern, repl, string, count=0, flags=0)
//   - subn(pattern, repl, string, count=0, flags=0)
//   - escape(pattern)
//   - purge()
//
// Compiled patterns expose Python-like methods and attributes: search, match,
// fullmatch, split, findall, finditer, sub, subn, pattern, flags, groups, and
// groupindex. Match values expose expand, group, groups, groupdict, start, end,
// span, pos, endpos, lastindex, lastgroup, re, and string.
//
// String and bytes inputs are accepted, and results preserve the input kind when
// returning matched text or substituted text. Replacement strings support common
// Python-style group references such as \1, \g<1>, and \g<name>. Callable
// replacements are also supported.
//
// # Supported flags
//
// IGNORECASE/I, MULTILINE/M, DOTALL/S, and VERBOSE/X are implemented. NOFLAG,
// ASCII/A, and UNICODE/U are accepted as compatibility constants. LOCALE/L and
// DEBUG are intentionally rejected with explicit errors.
//
// # Regex engine limitations
//
// Matching is implemented with Go's regexp package, which uses RE2 syntax and
// guarantees linear-time matching. This is safer for generated code, but it is
// not the same engine as CPython's re module. Python regex features that RE2
// does not support are not available here, including lookaround assertions,
// pattern backreferences, conditional groups, and some Python-specific escape
// behavior.
//
// # Python API divergences
//
// Dyson does not expose Python's re.Pattern, re.Match, re.RegexFlag,
// re.PatternError, or re.error module members. Starlark has different type and
// exception semantics, and the useful runtime values are returned directly from
// compile and matching functions. Compile and match failures are reported as Go
// errors surfaced to Starlark execution rather than as Python exception objects.
//
// Compiled Pattern and Match values are durable Dyson values: both implement the
// snapshot converter/restorer hooks so they can round-trip through the root
// snapshot encoder and decoder when stored in user globals.
package re
