# AGENTS.md

`dyson` is a Go module for running `starlark` programs from Go. The project is aimed at safely executing generated Starlark with Python-inspired standard-library compatibility, without requiring a Python runtime or broad host access.

See [README.md](./README.md) for the user-facing overview and [discovery.md](./discovery.md) for Starlark behavior notes collected during development.

## Project Structure

- `snapshot/` contains the Starlark globals snapshot implementation. Keep the public surface shaped like the standard `encoding/json` package: `snapshot.NewEncoder(w).Encode(globals)` and `snapshot.NewDecoder(r).Decode(&globals)`.
- `snapshot/snapshot_test.go` contains snapshot and REPL-resume tests. Keep related cases grouped with table tests and `t.Run` rather than adding many one-off test functions.
- `discovery.md` is informal design/research notes. Update it when learning important Starlark behavior that affects `dyson` design.

## Go Conventions

- This module targets Go `1.26.4`. Use modern Go idioms available up to that version.
- Keep changes small and direct. Avoid unnecessary abstractions unless they clarify a real boundary.
- The snapshot format is currently a private implementation detail. Do not export internal snapshot structs or enum tags unless there is a concrete compatibility requirement.
- Snapshot encoding must preserve Starlark mutable-container semantics. Lists and dicts form a graph, not just a tree, so object refs/ids are required to preserve aliases and cycles.

## Starlark Design Notes

- Treat Starlark as a small embedded language with an explicit host API, not as full Python.
- Starlark code has no ambient filesystem, network, process, or environment access unless Go exposes it.
- Prefer recoverable/domain failures as Starlark-visible data. Reserve Go errors from builtins for programmer errors, policy violations, resource limits, and unrecoverable failures.
- Do not serialize `*starlark.Thread` for REPL resume. Recreate threads and restore/replay globals instead.

## Tests

- Use `github.com/nalgeon/be` for assertions.
- Prefer table-driven tests and `t.Run` for related cases.
- Keep standalone test functions for distinct behavior-level scenarios, such as a REPL resume integration test.
- Run `go test ./...` after code changes.
