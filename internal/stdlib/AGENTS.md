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

Go Starlark `load` imports named symbols; bare `load("re.star")` is invalid. For Python-like namespacing, expose the module namespace as a symbol and import it explicitly:

```python
load("re.star", "re")
pattern = re.compile("[a-z]+", re.I | re.M)
```

The root `dyson.Load` function returns only the namespace symbol for stdlib modules. Direct member imports such as `load("re.star", "compile")` are intentionally unsupported.

## Snapshot Compatibility

Durable values returned into user globals should be plain snapshot-supported Starlark values whenever possible: `None`, bool, int, float, string, bytes, tuple, list, dict, and set.

Custom `starlark.Value` implementations are allowed for real module-defined types, but every production custom value returned to Starlark must implement `snapshot.Converter` and the module must register a matching `snapshot.RegisterRestorer` hook. The converter payload should use only snapshot-supported values, and tests should prove the custom value round-trips through `snapshot.NewEncoder` / `snapshot.NewDecoder`.

Avoid defining custom `starlark.Value` implementations for placeholder or scaffold values when a plain supported value would be enough.

Host functions are necessarily `*starlark.Builtin`; these should generally live in module globals, not inside durable user data structures.

## Error Handling

Starlark has no Python exceptions. For scaffolded or unsupported operations, return a Go error with a clear module-qualified message, which aborts evaluation in the same way as Starlark `fail`.

For future APIs where failures are expected and recoverable, prefer explicit data-returning variants such as `try_*` convention: `(value, None)` on success and `(None, "error message")` on failure.

## Tests

Keep module API-surface tests in the module package. Tests should verify:

- `LoadModule` returns the `{ModuleName: *starlarkstruct.Module}` shape.
- Scripts load symbols with `load("module", "symbol")`.
- Exported constants and aliases have compatibility-visible values.
- Scaffolded or unsupported operations abort with stable module-qualified messages.
- Any durable values returned by module APIs can round-trip through the root snapshot encoder when relevant.

Prefer Starlark testdata for module behavior. Put user-visible compatibility scenarios in `internal/stdlib/<name>/testdata/*.star` and execute them from a small Go harness in the module package. Keep Go assertions for host integration details that are awkward to express in Starlark, such as `LoadModule` shape, snapshot encoder/decoder integration, or low-level Go type checks.

Use focused chunks separated by `---` for Starlark testdata. Each chunk should cover one behavior area and include short comments explaining what is being verified. Use `go.starlark.net/starlarktest`'s `assert.star` helpers for in-script assertions, and prefer inline expected-error annotations such as `### "module.fn: message"` for failure cases when the local harness supports them.

When adding broad compatibility for a module, favor many small Starlark examples over one large script that sets globals for Go to inspect. The test should read like executable documentation of the supported compatibility surface and its intentional divergences.
