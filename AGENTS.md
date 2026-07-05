# AGENTS.md

`dyson` is a Go module for running `starlark` programs from Go. The project is aimed at safely executing generated Starlark with Python-inspired standard-library compatibility, without requiring a Python runtime or broad host access.

See [README.md](./README.md) for the user-facing overview and [discovery.md](./discovery.md) for Starlark behavior notes collected during development.

## Project Structure

- `internal/codec/` contains durable host-call value serialization for record/replay. Keep codec changes local to this package unless a stdlib custom value needs a small registration hook in its own module package.
- `internal/xfs/` contains filesystem and path-capability seams used by stdlib modules. Keep environment, process, and platform concerns out of this package.
- `internal/xos/` contains host OS seams that are not filesystem-specific, such as environment, process, working-directory, and platform capabilities.
- `discovery.md` is informal design/research notes. Update it when learning important Starlark behavior that affects `dyson` design.

## Go Conventions

- This module targets Go `1.26.4`. Use modern Go idioms available up to that version.
- Keep changes small and direct. Avoid unnecessary abstractions unless they clarify a real boundary.
- Durable host-call codecs must preserve enough Starlark value semantics for replay. Container codecs currently serialize trees rather than mutable-container graphs, so aliases and cycles need explicit object refs before they can be preserved across host-call boundaries.

## Starlark Design Notes

- Treat Starlark as a small embedded language with an explicit host API, not as full Python.
- Starlark code has no ambient filesystem, network, process, or environment access unless Go exposes it.
- Prefer recoverable/domain failures as Starlark-visible data. Reserve Go errors from builtins for programmer errors, policy violations, resource limits, and unrecoverable failures.
- Do not serialize `*starlark.Thread` for REPL resume. Recreate threads and restore/replay globals instead.

## Tests

- Use `github.com/nalgeon/be` for assertions.
- Prefer Starlark execution-based tests for behavior that is easiest to understand as Starlark code. Put readable scenarios in `testdata` fixtures and use `internal/chunkedfile` with `---` separators when setup/assertion chunks make the test clearer.
- Root `testdata/*.star` fixtures are REPL scenarios: chunks execute in one durable session, globals persist across chunks, and the final chunk should generally contain assertions after replay. Load modules once in setup and reuse those globals in later chunks unless repeated `load` behavior is the specific thing being tested.
- Prefer table-driven tests and `t.Run` for related cases.
- Keep standalone test functions for distinct behavior-level scenarios, such as a REPL resume integration test.
- Run `go test ./...` after code changes.
