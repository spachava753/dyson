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
// The package is currently scaffolded with API-compatible stubs and
// comprehensive Starlark testdata describing the intended Python-compatible
// behavior. Python CPU-time clocks process_time, process_time_ns, thread_time,
// and thread_time_ns are intentionally unsupported for now because Go has no
// portable standard-library process CPU clock, and goroutines do not map
// cleanly to Python's OS-thread CPU-time semantics.
package time
