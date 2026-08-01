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

`HostStdlibConfig(root)` enables broad host filesystem, environment, process, working-directory, terminal, platform, and clock access. It deliberately does not enable commands or HTTP requests:

```go
config := dyson.HostStdlibConfig(cwd)
modules := dyson.StdlibModules(config)
sphere := dyson.NewSphere(print, modules, initialGlobals, nil, false)
```

`root` is a base for relative paths, not a sandbox boundary. Absolute paths and `..` retain normal host semantics. When command execution is enabled, commands that omit `cwd` also start in `root`. The host configuration can mutate environment variables, change the process working directory and umask, and signal processes, so use it only for trusted code or behind an appropriate policy layer.

Command execution and HTTP access are separate, visible opt-ins. A downstream caller such as CPE can enable full host filesystem, subprocess, and network access as follows:

```go
config := dyson.HostStdlibConfig(cwd)
config.CommandRunner = dyson.HostCommandRunner() // Grants arbitrary host command execution.
config.HTTPClient = dyson.HostHTTPClient()       // Grants arbitrary host HTTP access.
modules := dyson.StdlibModules(config)
sphere := dyson.NewSphere(print, modules, initialGlobals, nil, false)
```

`HostCommandRunner` uses context-aware host commands, and `HostHTTPClient` attaches the evaluation context to each request. Canceling `Sphere.Eval` can therefore terminate either active operation.

### Capability mapping

| Configuration field | Consuming modules | Zero-value behavior |
| --- | --- | --- |
| `FS` | `os`, `glob`, `shutil`, `tempfile` | Filesystem operations fail closed. Optional interfaces add mutation, file I/O, path resolution, identity, and disk usage. |
| `Env` | `os`, `shutil`, `tempfile` | No host environment is read or mutated; required `os` operations fail closed. |
| `Process` | `os` | Process identity, signaling, groups, and umask operations fail closed. It never enables commands. |
| `WorkingDirectory` | `os` | `getcwd` and `chdir` fail closed. |
| `Terminal` | `shutil` | `get_terminal_size` uses its fallback when terminal access is unavailable. |
| `Platform` | `os`, `shutil` | Portable POSIX-like constants are used; no host access is granted. |
| `CommandRunner` | `subprocess`, `os.system` | Command execution fails with `subprocess execution is not configured`. |
| `HTTPClient` | `requests` | HTTP operations fail with `requests.request: HTTP requests are not configured`. |
| `Clock` | `time`, implicit `os.utime` timestamps | Current-time and sleep operations fail closed; pure conversions with explicit timestamps remain available. |

### HTTP requests

The `requests` module exposes `request`, `get`, `options`, `head`, `post`, `put`, `patch`, and `delete`:

```python
load("requests.star", "requests")

response = requests.get(
    "https://api.example.test/items",
    params={"limit": 10},
    timeout=5,
)
if response.ok:
    items = response.json()
```

The initial `requests.request` compatibility surface supports query parameters, raw or form data, JSON bodies, headers, cookies, Basic auth tuples, scalar or `(connect, read)` timeouts, and redirect control. Responses expose buffered `content`, decoded `text`, `status_code`, `url`, `reason`, `ok`, writable `encoding`, redirect `history`, and `is_redirect`, plus `json()`, `raise_for_status()`, and `close()`.

Response headers are an ordinary Starlark dictionary whose keys are normalized to lowercase. For example, use `response.headers["content-type"]`.

Multipart `files`, proxies, hooks, streaming, custom TLS verification, and client certificates are accepted by the call surface but fail explicitly rather than being ignored. Response bodies are fully buffered, with a 64 MiB limit in `HostHTTPClient`, and redirect history is ordered oldest to newest.

Successful HTTP calls and transport failures cross Dyson's normal durable builtin boundary, so replay returns the recorded response or error without repeating the network effect. Starlark has no exception types, so durable errors preserve their visible message rather than reconstructing a Go error type.

One `StdlibModules` call shares the configured filesystem and environment across related modules. It also creates one internal file-descriptor table shared by `os` and `tempfile`, so a descriptor returned by `tempfile.mkstemp` can be consumed by `os.read`, `os.write`, and `os.close`.

`os`, `glob`, `shutil`, and `requests` export Go-backed Starlark builtins. A `Sphere` wraps those builtins uniformly for record/replay; the modules do not instantiate a second layer of Starlark functions around host operations.

`pwd` and `grp` are currently catalog-only modules and do not perform user or group lookup, so there is no lookup capability yet.
