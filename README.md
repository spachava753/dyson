# Dyson

`dyson` runs Starlark programs from Go with Python-inspired standard-library compatibility. Host access is capability-based: Starlark code has no ambient filesystem, environment, process, command, or clock access unless its caller supplies that capability.

## Sessions and standard-library capabilities

`NewSphere` installs only the sources passed by its caller. With no sources, a session contains Starlark's built-in universe but no Dyson globals or loadable modules; unknown `load` names never fall back to the ambient filesystem.

```go
sphere := dyson.NewSphere(print)
defer sphere.Close()
```

Plain custom modules and globals are direct sources:

```go
sphere := dyson.NewSphere(
	print,
	dyson.ModuleSet(customModules),
	dyson.GlobalSet(initialGlobals),
)
defer sphere.Close()
```

Construct a `Stdlib` only when Dyson's compatibility library is wanted. Passing the owner itself installs every standard module and global, and transfers its cleanup lifetime to the Sphere:

```go
stdlib := dyson.NewStdlib(dyson.StdlibConfig{
	FS:            fakeFS,
	Env:           fakeEnv,
	CommandRunner: fakeRunner,
	Clock:         fakeClock,
})
sphere := dyson.NewSphere(print, stdlib, dyson.ModuleSet(customModules))
defer sphere.Close()
```

Use `Stdlib.Select` to expose an exact subset. Module names are load names such as `"os.star"`; globals such as `open` are selected separately. Unknown names are omitted rather than broadening access.

```go
stdlib := dyson.NewStdlib(config)
selected := stdlib.Select(dyson.StdlibSelection{
	Modules: []string{"os.star", "re.star"},
	Globals: []string{"open"},
})
sphere := dyson.NewSphere(print, selected, dyson.ModuleSet(customModules))
defer sphere.Close()
```

Sources are merged in order, and later sources replace modules or globals with matching names. `Sphere.Close` closes every source and is therefore the normal owner for a `Stdlib` passed to its constructor.

The public interfaces use standard Go types where possible. `CommandRunner` receives a `context.Context` plus Dyson's public `Command` and returns `CommandResult`, allowing fakes and policy adapters to inspect or reject normalized command requests. Implementations must stop promptly when the context ends.

Custom builtins can call `dyson.EvaluationContext(thread)` with their `*starlark.Thread` argument to obtain the context passed to the current `Sphere.Eval` call. The accessor returns `context.Background()` when the thread is not managed by a Sphere. Builtins should honor cancellation and must not retain the context beyond the current call.

Callers that provide their own Starlark thread and loader use the same owned standard library directly:

```go
stdlib := dyson.NewStdlib(config)
defer stdlib.Close()
modules := stdlib.Modules()
globals := stdlib.Globals()
```

`Modules` and `Globals` return dictionary snapshots whose Starlark values share the owner's module state. `Stdlib.Close` deterministically closes global files and `os`/`tempfile` descriptors abandoned by the custom execution.

### Trusted host access

Host access must be selected explicitly. `HostStdlibConfig(root)` grants broad host filesystem, environment, process, working-directory, terminal, platform, and clock access, but deliberately does not enable commands or HTTP requests:

```go
config := dyson.HostStdlibConfig(cwd)
sphere := dyson.NewSphere(print, dyson.NewStdlib(config))
```

`root` is a base for relative paths, not a sandbox boundary. Absolute paths and `..` retain normal host semantics. When command execution is enabled, commands that omit `cwd` also start in `root`. The host configuration can mutate environment variables, change the process working directory and umask, and signal processes, so use it only for trusted code or behind an appropriate policy layer.

Command execution and HTTP access are separate, visible opt-ins. A downstream caller such as CPE can enable full host filesystem, subprocess, and network access as follows:

```go
config := dyson.HostStdlibConfig(cwd)
config.CommandRunner = dyson.HostCommandRunner() // Grants arbitrary host command execution.
config.HTTPClient = dyson.HostHTTPClient()       // Grants arbitrary host HTTP access.
sphere := dyson.NewSphere(print, dyson.NewStdlib(config))
```

`HostCommandRunner` uses context-aware host commands, and `HostHTTPClient` attaches the evaluation context to each request. Canceling `Sphere.Eval` can therefore terminate either active operation.

### Subprocess execution

`subprocess.run` accepts `timeout` as finite integer or floating-point seconds. Positive values must fit within Go's `time.Duration` range, which is approximately 292 years; `None` leaves the evaluation context unchanged, while zero and negative values create an immediate timeout. The timeout is applied only around command execution, after argument validation:

```python
load("subprocess.star", "subprocess")

completed = subprocess.run(
    ["tool", "argument"],
    capture_output=True,
    timeout=5,
)
```

Dyson derives a child context for the timeout, so an earlier `Sphere.Eval` cancellation remains distinguishable from a command-local deadline. Expiration aborts evaluation with a `*dyson.SubprocessTimeoutExpiredError` whose message is such as `subprocess.run: command timed out after 5 seconds`. Go callers can recover that typed cause through the Starlark evaluation wrapper with `errors.AsType`; its `Cmd` and `Timeout` fields retain the original Starlark arguments, while `Stdout` and `Stderr` contain partial captured bytes and remain bytes even when `text=True`. `HostCommandRunner` kills and waits for the host process through `exec.CommandContext`, then closes its controlled stdin and capture-pipe endpoints when the context ends so descendants cannot block timeout cleanup; successful commands with an active context still finish input delivery and drain captured output to EOF. Custom `CommandRunner` implementations must likewise stop promptly when the supplied context ends and should return any already captured output together with the context error.

### Capability mapping

| Configuration field | Consumers | Zero-value behavior |
| --- | --- | --- |
| `FS` | global `open`, `os`, `glob`, `shutil`, `tempfile` | File operations fail closed. Assign any `afero.Fs`; its methods and optional Afero symlink interfaces define the available behavior. |
| `Env` | `os`, `shutil`, `tempfile` | No host environment is read or mutated; required `os` operations fail closed. |
| `Process` | `os` | Process identity, signaling, groups, and umask operations fail closed. It never enables commands. |
| `WorkingDirectory` | `os` | `getcwd` and `chdir` fail closed. |
| `Terminal` | `shutil` | `get_terminal_size` uses its fallback when terminal access is unavailable. |
| `Platform` | `os`, `shutil` | Current Go platform constants are used; no host access is granted. |
| `CommandRunner` | `subprocess`, `os.system` | Command execution fails with `subprocess execution is not configured`. |
| `HTTPClient` | `requests` | HTTP operations fail with `requests.request: HTTP requests are not configured`. |
| `Clock` | `time`, implicit `os.utime` timestamps | Current-time and sleep operations fail closed; pure conversions with explicit timestamps remain available. |

`os.lstat` and symlink-sensitive operations require the configured filesystem's `afero.Lstater` to report `usedLstat=true`; merely implementing the interface is not sufficient, and Dyson never substitutes link-following `Stat` behavior. `shutil.rmtree` rejects a symbolic link at its root, then delegates recursive deletion to `afero.Fs.RemoveAll`. Symlink-attack resistance within the tree therefore depends on the selected Afero backend. `HostStdlibConfig` uses `afero.OsFs`, whose `RemoveAll` delegates to Go's `os.RemoveAll`.

### File reading

Python-style `open` is available only when a selected source contributes that global. A full `Stdlib` includes it; a subset must list `"open"` in `StdlibSelection.Globals`. It uses the configured `afero.Fs` shared with `os.star`; it never falls back to the ambient host filesystem or invokes subprocesses. Afero backends such as `afero.NewMemMapFs()` can be assigned directly. Adapt a standard read-only `io/fs.FS` with `dyson.FromIOFS`, which accepts leading `./` path components, preserves source-level `fs.ReadDirFS` traversal, rejects non-read-only descriptor flags, and exposes strict link inspection when the source implements `fs.ReadLinkFS`. Do not use bare `afero.FromIOFS` as a Python-compatible descriptor backend: its `OpenFile` ignores flags and permissions.

```go
writableConfig := dyson.StdlibConfig{FS: afero.NewMemMapFs()}
readOnlyConfig := dyson.StdlibConfig{FS: dyson.FromIOFS(os.DirFS("."))}
```

```python
file = open("README.md", mode="r", encoding="utf-8")
text = file.read()
file.close()
print(text)
```

The supported call shape is `open(file, mode="r", buffering=-1, encoding=None, errors=None, newline=None, closefd=True, opener=None)`, with Python's positional parameter order. Modes `"r"` and `"rt"` return text; `"rb"` returns native bytes. Only default buffering (`-1`), path-owned descriptors (`closefd=True`), and no custom opener (`opener=None`) are supported.

Text defaults to UTF-8 with strict decoding. Supported encodings and error handlers match `bytes.decode` below. With the default `newline=None`, `\r`, `\r\n`, and `\n` are translated to `\n`, including when a `\r\n` pair crosses host read boundaries. The other accepted newline values are `""`, `"\n"`, `"\r"`, and `"\r\n"`; `read` returns line endings unchanged for those values.

Open file values provide `read()`, positional-only `read(size)`, and idempotent `close()`. `None` reads to EOF for both text and binary streams. Text streams also treat any negative size as read-to-EOF; binary streams accept only `-1` and reject values below it. `read(size=...)` is rejected. Non-negative text sizes count decoded characters, binary sizes count bytes, and `read(0)` does not access the underlying file. File opening and reads use ordinary synchronous I/O and are not interrupted by `Sphere.Eval` cancellation.

Handles remain usable across successful `Sphere.Eval` calls until explicitly closed. `Sphere.Close` is terminal and idempotent: it closes every remaining global `open` handle and every descriptor-style file retained by `os` or `tempfile`, including handles abandoned by failed evaluations, then rejects later `Eval` calls. Explicit `file.close()` and `os.close()` remove their handles from session ownership. Callers should normally `defer sphere.Close()` immediately after construction.


Native bytes returned by `open`, `os.read`, HTTP responses, subprocess output, regular-expression operations, and byte-oriented tempfile functions provide Python-compatible decoding:

```python
text = data.decode("utf-8", errors="strict")
```

`bytes.decode` supports UTF-8, ASCII, and Latin-1, including common aliases. Its `errors` argument supports `"strict"` (the default), `"ignore"`, and `"replace"`. Unsupported modes, encodings, error handlers, empty or missing paths, directory paths, and reads from closed files fail with operation and path context where applicable. Empty filenames fail before filesystem-specific root normalization.

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

One `Sphere` or `Stdlib` owner shares the configured filesystem and environment across related modules. It also creates one internal file-descriptor table shared by `os` and `tempfile`, so a descriptor returned by `tempfile.mkstemp` can be consumed by `os.read`, `os.write`, and `os.close`.

`pwd` and `grp` are currently catalog-only modules and do not perform user or group lookup, so there is no lookup capability yet.
