// Package os implements Dyson's Starlark compatibility subset of Python's os
// module.
//
// # Loading
//
// In user code, import the module namespace explicitly:
//
//	load("os.star", "os")
//	path = os.path.join("a", "b")
//
// Go Starlark does not support bare load("os.star"); the namespace symbol must
// be named in the load statement. Direct symbol imports such as
// load("os.star", "getcwd") are intentionally unsupported.
//
// Dyson will eventually expose capability-selected stdlib modules. When the os
// module is available, it should provide Python-like host filesystem, process,
// and environment access rather than silently sandboxing individual functions.
// Capability policy should decide whether this module is exposed to a Starlark
// program in the first place.
package os
