# AGENTS.md

`dyson` is a Go module for running Starlark programs from Go. The project is aimed at safely executing generated Starlark with Python-inspired standard-library compatibility, without requiring a Python runtime or broad host access.

See [README.md](./README.md) for the user-facing overview and [discovery.md](./discovery.md) for Starlark behavior notes collected during development.

## Project Structure

- `eval.go` owns the incremental Starlark session, explicit `SphereSource` composition, globals, exact module loading without ambient fallback, evaluation cancellation, and terminal `Sphere.Close` cleanup.
- `stdlib.go` owns the closable `Stdlib` assembled for `Sphere` and for callers using their own Starlark thread and loader.
- `internal/pybytes/` owns Python-compatible methods added to native Starlark bytes. It intentionally depends on the pinned Starlark runtime's internal bytes method table because the runtime has no public extension seam.
- `internal/stdlib/builtins/` owns Python-inspired globals such as `open` and the registry of live global file handles. Explicit `file.close()` unregisters a handle; `Sphere.Close` closes whatever remains.
- `internal/stdlibfs/` owns the concrete rooted Afero host backend, the read-only `io/fs` adapter, shared Afero helpers, and the descriptor table closed by `Sphere.Close`. `StdlibConfig.FS` is directly `afero.Fs`; do not introduce a second filesystem interface. Keep environment, process, and platform concerns out of this package.
- `internal/xhttp/` owns the normalized HTTP client seam and explicit host-network adapter used by stdlib modules. Keep protocol-independent network policy out of stdlib package implementations.
- `internal/xos/` owns host OS seams that are not filesystem- or network-specific, such as environment, process, command execution, working-directory, and platform capabilities.
- The root package uses Afero directly for `StdlibConfig.FS` and aliases the `xhttp` and `xos` seams needed by the remaining configuration fields.
- `discovery.md` is informal design/research notes. Update it when learning important Starlark behavior that affects `dyson` design.

## Go Conventions

- This module targets Go `1.26.4`. Use modern Go idioms available up to that version.
- Keep changes small and direct. Avoid unnecessary abstractions unless they clarify a real seam.
- Keep Dyson byte results as native `starlark.Bytes`; route construction through `internal/pybytes` so Python-compatible methods are registered without changing bytes identity or comparison semantics.
- `NewSphere` installs only explicit sources. Use `ModuleSet` and `GlobalSet` for custom bindings, a full `Stdlib` for the complete compatibility surface, or `Stdlib.Select` for an exact subset.
- Global `open` uses ordinary synchronous `afero.File` reads. Do not add context-cancellation adapters to file operations; configured Afero backends should remain direct filesystem implementations.
- Symlink-sensitive behavior requires `afero.Lstater` to report that it actually used `lstat`; never fall back to link-following `Stat`. Standard `io/fs.FS` values must use `dyson.FromIOFS` so leading `./` paths, `fs.ReadDirFS`, descriptor flags, and `fs.ReadLinkFS` are handled correctly. `shutil.rmtree` delegates descendant deletion to `afero.Fs.RemoveAll`, so document and test the selected backend's symlink guarantees.

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
