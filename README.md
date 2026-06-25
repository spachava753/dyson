# Dyson

`dyson` is a Go module to be make it easier to run `starlark` programs. In particular, we offer partial python stdlib compatability, which is useful for when you want want to securely run AI generated code, without a python installation or tricky sandboxing techniques.

# Limitations

TBD

# Python Stdlib compatibility

Dyson exposes loadable compatibility modules through `dyson.Load`, a `starlark.Thread.Load` implementation:

```go
thread := &starlark.Thread{Load: dyson.Load}
```
