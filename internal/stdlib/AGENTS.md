# AGENTS.md

This directory contains loadable Starlark standard-library compatibility modules.

## Module Shape

- Put each module in its own package under `internal/stdlib/<name>`.
- Export `const ModuleName = "<name>"`.
- Export `func LoadModule() (starlark.StringDict, error)`.
- `LoadModule` should return a single-entry dict shaped as `{ModuleName: *starlarkstruct.Module}`.
- The `starlarkstruct.Module` should use `Name: ModuleName` and `Members: starlark.StringDict{...}`.
- Builtins should be named with fully qualified names such as `ModuleName + ".compile"` so errors read like `re.compile: ...`.
- Cache immutable module globals with `sync.OnceValue` or an equivalent concurrency-safe one-time initializer.
- Freeze module values before returning them when they are shared across threads or executions.

## Load Semantics

Go Starlark `load` imports named symbols; bare `load("re")` is invalid.

The root `dyson.Load` function unwraps a module dict shaped as `{name: *starlarkstruct.Module}` and returns the module members to the interpreter. User-facing examples should therefore prefer direct imports:

```python
load("re", "compile", "I", "M")
pattern = compile("[a-z]+", I | M)
```

Do not design examples around `load("re", "re")` unless there is a deliberate reason to expose a module namespace as a symbol.

## Snapshot Compatibility

Durable values returned into user globals should be plain snapshot-supported Starlark values whenever possible: `None`, bool, int, float, string, tuple, list, and dict.

Avoid defining custom `starlark.Value` implementations for placeholder or scaffold values. Custom values are not currently supported by `snapshot.go` and will break REPL/session snapshots unless snapshot support is explicitly added.

Host functions are necessarily `*starlark.Builtin`; these should generally live in module globals, not inside durable user data structures.

## Error Handling

Starlark has no Python exceptions. For scaffolded or unsupported operations, return a Go error with a clear module-qualified message, which aborts evaluation in the same way as Starlark `fail`.

For future APIs where failures are expected and recoverable, prefer explicit data-returning variants such as `try_*` convention: `(value, None)` on success and `(None, "error message")` on failure.

## Tests

Keep module API-surface tests in the module package. Tests should verify:

- `LoadModule` returns the `{ModuleName: *starlarkstruct.Module}` shape.
- Scripts load symbols with `load("module", "symbol")`.
- Exported constants and aliases have compatibility-visible values.
- Scaffolded operations abort with stable module-qualified messages.
- Any durable values returned by stubs can round-trip through the root snapshot encoder when relevant.
