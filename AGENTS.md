# AGENTS.md

`dyson` is a Go module for running Starlark programs from Go. The project is aimed at safely executing generated Starlark with Python-inspired standard-library compatibility, without requiring a Python runtime or broad host access.

See [README.md](./README.md) for the user-facing overview and [discovery.md](./discovery.md) for Starlark behavior notes collected during development.

## Project Structure

- `eval.go` owns the incremental Starlark session, including globals, module loading, and evaluation cancellation.
- `internal/xfs/` owns filesystem and path-capability seams used by stdlib modules. Keep environment, process, and platform concerns out of this package.
- `internal/xhttp/` owns the normalized HTTP client seam and explicit host-network adapter used by stdlib modules. Keep protocol-independent network policy out of stdlib package implementations.
- `internal/xos/` owns host OS seams that are not filesystem- or network-specific, such as environment, process, command execution, working-directory, and platform capabilities.
- The root package aliases the `xfs`, `xhttp`, and `xos` seams needed by `StdlibConfig`.
- `discovery.md` is informal design/research notes. Update it when learning important Starlark behavior that affects `dyson` design.

## Go Conventions

- This module targets Go `1.26.4`. Use modern Go idioms available up to that version.
- Keep changes small and direct. Avoid unnecessary abstractions unless they clarify a real seam.

## Starlark Design Notes

- Treat Starlark as a small embedded language with an explicit host interface, not as full Python.
- Starlark code has no ambient filesystem, network, process, or environment access unless Go exposes it.
- Prefer recoverable/domain failures as Starlark-visible data. Reserve Go errors from builtins for programmer errors, policy violations, resource limits, and unrecoverable failures.

## Tests

- Use `github.com/nalgeon/be` for assertions.
- Prefer Starlark execution-based tests for behavior that is easiest to understand as Starlark code. Put readable scenarios in module-local `testdata` fixtures and use `internal/chunkedfile` with `---` separators when setup/assertion chunks make the test clearer.
- Prefer table-driven tests and `t.Run` for related cases.
- Keep standalone test functions for distinct behavior-level and integration scenarios.
- Run `go test ./...` after code changes.
