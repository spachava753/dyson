# Dyson

`dyson` is a Go module to be make it easier to run `starlark` programs. In particular, we offer partial python stdlib compatability, which is useful for when you want want to securely run AI generated code, without a python installation or tricky sandboxing techniques.

# Limitations

TBD

# Python Stdlib compatibility

Dyson exposes loadable compatibility modules through `dyson.StdlibModules()`, keyed by Starlark load path:

```go
modules := dyson.StdlibModules()
thread := &starlark.Thread{
	Load: func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
		globals, ok := modules[module]
		if !ok {
			return nil, fmt.Errorf("unknown module %q", module)
		}
		return globals, nil
	},
}
```
