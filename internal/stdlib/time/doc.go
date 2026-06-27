// Package time implements Dyson's Starlark compatibility subset of Python's
// time module.
//
// # Loading
//
// In user code, import the module namespace explicitly:
//
//	load("time.star", "time")
//	now = time.time()
//
// Go Starlark does not support bare load("time.star"); the namespace symbol
// must be named in the load statement. Direct symbol imports such as
// load("time.star", "sleep") are intentionally unsupported.
//
// # Supported API
//
// The module supports common Python time workflows with a deterministic UTC
// timezone policy:
//
//   - time() -> float Unix timestamp seconds.
//   - time_ns() -> int Unix timestamp nanoseconds.
//   - monotonic() -> float elapsed seconds from the current Starlark thread's
//     first monotonic clock read.
//   - monotonic_ns() -> int elapsed nanoseconds from the current Starlark
//     thread's first monotonic clock read.
//   - perf_counter() -> float elapsed seconds using the same source as
//     monotonic().
//   - perf_counter_ns() -> int elapsed nanoseconds using the same source as
//     monotonic_ns().
//   - sleep(seconds) -> None after blocking for the requested int or float
//     seconds.
//   - gmtime(seconds=None) -> struct_time in UTC.
//   - localtime(seconds=None) -> struct_time in UTC.
//   - mktime(t) -> float Unix timestamp seconds, interpreting t as UTC local
//     time under Dyson's deterministic timezone policy.
//   - asctime(t=None) -> str formatted like CPython's fixed-width asctime
//     output.
//   - ctime(seconds=None) -> str equivalent to asctime(localtime(seconds)).
//   - strftime(format, t=None) -> str for common directives.
//   - strptime(string, format) -> struct_time for common directives.
//   - get_clock_info(name) -> namespace-like value with adjustable,
//     implementation, monotonic, and resolution attributes.
//   - struct_time(sequence) -> struct_time value from a sequence with at least
//     nine integer fields.
//   - tzset() is present but intentionally unsupported.
//
// Module globals timezone, altzone, daylight, and tzname are available with UTC
// values. The supported struct_time value behaves like Python's tuple subclass
// for Starlark purposes: it has length 9, can be indexed and iterated, converts
// with tuple(value), and exposes tm_year, tm_mon, tm_mday, tm_hour, tm_min,
// tm_sec, tm_wday, tm_yday, and tm_isdst attributes.
//
// # Supported strftime/strptime directives
//
// strftime supports the common directives %Y, %m, %d, %H, %M, %S, %a, %b, %j,
// %w, and %%. Unknown strftime directives are preserved literally as % plus the
// directive character. strptime currently supports %Y, %m, %d, %H, %M, %S, %a,
// %b, and %% through Go's time parser.
//
// # Python API divergences
//
// Dyson deliberately uses UTC for localtime, mktime, timezone, altzone,
// daylight, and tzname so generated programs behave deterministically and do
// not depend on the host process timezone or environment. tzset therefore
// aborts with a stable unsupported error instead of mutating timezone state.
//
// Python CPU-time clocks process_time, process_time_ns, thread_time, and
// thread_time_ns are intentionally unsupported for now because Go has no
// portable standard-library process CPU clock, and goroutines do not map
// cleanly to Python's OS-thread CPU-time semantics.
//
// Starlark has no Python class objects, namedtuple subclassing, exceptions, or
// None/float/int identity with CPython. Return values use the closest Starlark
// equivalents: float timestamps are starlark.Float, integer nanoseconds and
// constants are starlark.Int, strings are starlark.String, sleep returns
// starlark.None, and struct_time/get_clock_info are custom or namespace-like
// Starlark values with Python-compatible fields.
package time
