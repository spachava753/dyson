# AGENTS.md

This directory contains loadable Starlark standard-library compatibility modules.

## Module Shape

- Put each module in its own package under `internal/stdlib/<name>`.
- Export `const ModuleName = "<name>"`.
- Export `var Module = &starlarkstruct.Module{Name: ModuleName, Members: starlark.StringDict{...}}` for static immutable module namespaces, load an embedded Starlark source file into `var Module` when most behavior is clearer in Starlark, or `func MakeModule(...) *starlarkstruct.Module` when a module needs per-session state.
- Add the module to the root `dyson.StdlibModules(config)` factory as `{ModuleName + ".star": {ModuleName: Module}}` or by calling its module factory. Host-backed behavior must come from `StdlibConfig`; do not construct ambient host capabilities in the root factory.
- Builtins should be named with fully qualified names such as `ModuleName + ".compile"` so errors read like `re.compile: ...`.
- For methods on custom Starlark values, follow Go Starlark's native bound-method pattern: keep a package-level static method table of `*starlark.Builtin` values, return `method.BindReceiver(value)` from `Attr`, and read the receiver inside the package-level builtin with `fn.Receiver()`. Avoid allocating per-attribute closure builtins such as `starlark.NewBuiltin("type.method", value.method(name))`.
- Put the method's implementation directly in the receiver-aware builtin when it is specific to that method. Use plain package-level helpers only for genuinely shared algorithms; avoid creating trivial receiver methods that bound builtins immediately call through.
- Use a package-level `Module` value for static immutable module globals.
- Freeze module values before returning them when they are shared across threads or executions.
- Catalog-only modules may temporarily use `nil` member values with Python doc-link comments to reserve planned interface surface. Do not freeze modules containing nil members; replace nil entries with real values and freeze the module when behavior is added.

## Load Semantics

Callers should pass the result of `dyson.StdlibModules(config)` into `NewSphere` or their own Starlark load implementation. Go Starlark `load` imports named symbols; bare `load("re.star")` is invalid. For Python-like namespacing, expose the module namespace as a symbol and import it explicitly:

```python
load("re.star", "re")
pattern = re.compile("[a-z]+", re.I | re.M)
```

The root `dyson.StdlibModules(config)` factory returns only namespace symbols for stdlib modules. Direct member imports such as `load("re.star", "compile")` are intentionally unsupported.

## Runtime Values

Prefer plain Starlark values (`None`, bool, int, float, string, bytes, tuple, list, dict, and set where supported) when a custom value is not required. Custom `starlark.Value` implementations are appropriate for real module-defined types such as compiled patterns, matches, response objects, and tuple-like values.

Host functions are necessarily `*starlark.Builtin`; these should generally live in module globals rather than inside user data structures.

## Error Handling

Starlark has no Python exceptions. For scaffolded or unsupported operations, return a Go error with a clear module-qualified message, which aborts evaluation in the same way as Starlark `fail`.

For future interfaces where failures are expected and recoverable, prefer explicit data-returning variants such as `try_*` convention: `(value, None)` on success and `(None, "error message")` on failure.

## Tests

Keep module interface-surface tests in the module package. Tests should verify:

- `Module` exposes the expected namespace members and attributes.
- Scripts load symbols with namespace-only imports such as `load("re.star", "re")`.
- Exported constants and aliases have compatibility-visible values.
- Intentionally unsupported operations abort with stable module-qualified messages.

Prefer Starlark testdata for module behavior. Put user-visible compatibility scenarios in `internal/stdlib/<name>/testdata/*.star` and execute them from a small Go harness in the module package. Keep Go assertions for host integration details that are awkward to express in Starlark, such as module shape, deterministic fake-time setup, or low-level Go type checks.

Write behavior tests for the desired interface, not for temporary scaffold behavior. When adding a new module, it is correct and expected for broad compatibility testdata to be red until the implementation catches up. Do not make tests pass by asserting generic placeholder errors such as `"module.fn: not implemented"` for interfaces that are intended to be implemented. Only assert error behavior in testdata when the error is the intended public contract, such as an explicitly unsupported Python feature, invalid argument validation, platform/policy limitation, or resource/cancellation failure.

Use focused chunks separated by `---` for Starlark testdata. Each chunk should cover one behavior area and include short comments explaining what is being verified. Use `go.starlark.net/starlarktest`'s `assert.star` helpers for in-script assertions, and prefer inline expected-error annotations such as `### "module.fn: message"` for intended failure cases when the local harness supports them.

When adding broad compatibility for a module, favor many small Starlark examples over one large script that sets globals for Go to inspect. The test should read like executable documentation of the supported compatibility surface and its intentional divergences.

For interfaces involving clocks, timers, sleeps, timeouts, or concurrent blocking, consider using Go's `testing/synctest` in the Go harness. Inside a synctest bubble, the standard `time` package uses deterministic fake time starting at `2000-01-01 00:00:00 UTC`; fake time advances only when all goroutines are blocked. This is ideal for asserting exact behavior for wall-clock, monotonic, perf-counter, and sleep interfaces without weakening tests around real time.
