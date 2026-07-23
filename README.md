# Dyson

`dyson` runs Starlark programs from Go with Python-inspired standard-library compatibility. Host access is capability-based: Starlark code has no ambient filesystem, environment, process, command, or clock access unless its caller supplies that capability.

## Standard library capabilities

`StdlibModules` constructs one coherent set of loadable modules from `StdlibConfig`. The zero configuration is fail-closed:

```go
modules := dyson.StdlibModules(dyson.StdlibConfig{})
sphere := dyson.NewSphere(print, modules, initialGlobals, nil, false)
```

The modules remain loadable, but operations that need a missing capability return module-qualified errors. In particular, `subprocess.run` and `os.system` never fall back to host command execution.

Tests and sandboxed consumers can inject fakes directly:

```go
modules := dyson.StdlibModules(dyson.StdlibConfig{
	FS:            fakeFS,
	Env:           fakeEnv,
	CommandRunner: fakeRunner,
	Clock:         fakeClock,
})
```

The public interfaces use standard Go types where possible. `CommandRunner` receives a `context.Context` plus Dyson's public `Command` and returns `CommandResult`, allowing fakes and policy adapters to inspect or reject normalized command requests. Implementations must stop promptly when the context ends.

### Trusted host access

`HostStdlibConfig(root)` enables broad host filesystem, environment, process, working-directory, terminal, platform, and clock access. It deliberately does not enable commands:

```go
config := dyson.HostStdlibConfig(cwd)
modules := dyson.StdlibModules(config)
sphere := dyson.NewSphere(print, modules, initialGlobals, nil, false)
```

`root` is a base for relative paths, not a sandbox boundary. Absolute paths and `..` retain normal host semantics. When command execution is enabled, commands that omit `cwd` also start in `root`. The host configuration can mutate environment variables, change the process working directory and umask, and signal processes, so use it only for trusted code or behind an appropriate policy layer.

Command execution is a separate, visible opt-in. A downstream caller such as CPE can enable full host filesystem and subprocess access as follows:

```go
config := dyson.HostStdlibConfig(cwd)
config.CommandRunner = dyson.HostCommandRunner() // Grants arbitrary host command execution.
modules := dyson.StdlibModules(config)
sphere := dyson.NewSphere(print, modules, initialGlobals, nil, false)
```

`HostCommandRunner` uses context-aware host commands, so canceling `Sphere.Eval` can terminate an active command.

### Capability mapping

| Configuration field | Consuming modules | Zero-value behavior |
| --- | --- | --- |
| `FS` | `os`, `glob`, `shutil`, `tempfile` | Filesystem operations fail closed. Optional interfaces add mutation, file I/O, path resolution, identity, and disk usage. |
| `Env` | `os`, `shutil`, `tempfile` | No host environment is read or mutated; required `os` operations fail closed. |
| `Process` | `os` | Process identity, signaling, groups, and umask operations fail closed. It never enables commands. |
| `WorkingDirectory` | `os` | `getcwd` and `chdir` fail closed. |
| `Terminal` | `shutil` | Terminal queries fail unless Starlark supplies a fallback. |
| `Platform` | `os`, `shutil` | Portable POSIX-like constants are used; no host access is granted. |
| `CommandRunner` | `subprocess`, `os.system` | Command execution fails with `subprocess execution is not configured`. |
| `Clock` | `time`, implicit `os.utime` timestamps | Current-time and sleep operations fail closed; pure conversions with explicit timestamps remain available. |

One `StdlibModules` call shares the configured filesystem and environment across related modules. It also creates one internal file-descriptor table shared by `os` and `tempfile`, so a descriptor returned by `tempfile.mkstemp` can be consumed by `os.read`, `os.write`, and `os.close`.

`pwd` and `grp` are currently catalog-only modules and do not perform user or group lookup, so there is no lookup capability yet.
