# Design

This document sketches the design for Dyson: a record-replay, durable Starlark engine. The name comes from the idea of a Dyson Sphere: Starlark is the small deterministic core, and Dyson wraps it with controlled host capabilities, Python-inspired standard-library compatibility, and durable execution.

Starlark is a hermetic, deterministic language with Python-like syntax and a much smaller runtime surface. Dyson should preserve that property by making every non-deterministic or effectful boundary explicit, recordable, and replayable.

## Use cases

- Properly sandboxed LLM agent execution with fine-grained capabilities.
- Avoid a full container with bash, a filesystem namespace, process spawning, and broad ambient authority when the agent only needs a small capability surface.
- Serve higher volume than container-per-run architectures because the host can run Starlark in-process with explicit resource and capability controls.
- Offer a platform-independent Go module, conceptually similar to hosted code-execution products, but embeddable anywhere Go runs.
- Embedded scripting for Go applications that need Python-like ergonomics without a Python runtime.
- Untrusted code execution with a deliberately small, auditable host API.
- Durable scripting and workflow execution where a REPL/session can be restored after process restart or machine failure.
- Reproducible debugging of generated scripts by replaying the exact prompts and host interactions that produced a result.

## Record-replay

The durable unit is a REPL session, represented by:

- The ordered prompt/chunk log.
- The Starlark file options and Dyson runtime configuration used for the session.
- The session capability manifest: the registered host builtins, modules, custom value restorers, their schema versions, and the policy assigned to each builtin.
- The ordered host-event log produced while executing each chunk.

A running Dyson session owns one `*starlark.Thread` and one globals dictionary. At a normal REPL boundary, Starlark call frames have unwound. Dyson should not serialize the thread, interpreter frames, or thread locals. Instead, restore creates a fresh thread, a fresh globals dictionary, installs the same host API, and re-executes chunks in order. If recovery reaches a chunk that was interrupted after some host events were saved, the chunk is executed again from its beginning; already saved events are consumed, and execution starts appending new events at the first missing event.

### Chunks

Each user prompt is recorded as a chunk. A chunk should record enough information to make replay unambiguous:

- The source text as provided to the REPL.
- Whether it was executed as an expression or as statement/file input.
- The chunk index and synthetic filename used for parsing/backtraces.
- The host events produced while running the chunk.
- The final result, evaluation error, printed output transcript, and optional post-chunk checksum for debugging.

Failed chunks are still part of the session history. Starlark has no `try`/`except`, so a Go error from a builtin aborts the current chunk, but earlier mutations in that chunk may already have happened. Replay must reproduce the same abort at the same point rather than silently continuing.

### Host events

Host builtins are the main boundary between deterministic Starlark execution and the outside world. Dyson wraps registered host builtins and maintains an ordered event cursor while executing each chunk. For every `RecordReplay` builtin call, the wrapper should construct the expected event shape containing:

- A monotonically increasing event index, either per session or per chunk.
- The stable builtin name, such as `time.time` or `os.read`.
- The builtin implementation/schema version.
- The serialized positional arguments and keyword arguments when they are supported by the value layer.
- The serialized result value, or the serialized Go error message/classification.
- Optional observability data such as wall time, duration, and backtrace. These are useful for debugging but should not be required for deterministic replay.

If the next event already exists at the cursor, the wrapper consumes exactly that event. It verifies that the builtin name, version, and serialized inputs match the call being executed. If they match, it returns the recorded value or recorded error without consulting the host effect. If they do not match, execution has diverged and should fail loudly.

If no event exists at the cursor, the wrapper serializes and validates the inputs before calling the real Go function, calls the function, serializes the result or error, and appends a new event. Unsupported inputs or outputs must fail closed. Unsupported values must not be allowed to enter durable user state through a recorded host event. The wrapper should normally convert this to a Go error so the current Starlark chunk aborts with a clear durability message. A panic is appropriate only for development-time strict mode or impossible internal invariant violations.

This is an ordered log, not an input-keyed cache. Repeated calls with the same inputs are separate events:

```python
a = time.time()
b = time.time()
```

The two calls have the same input `()`, but they must consume two different events and may return two different values.

### Host event persistence

The core design does not require a single flush policy. A storage implementation may choose to persist after every host event, after each chunk, in larger batches, or at session boundaries depending on workload, volume, latency, and storage backend. Highest-fidelity recovery persists each host event before the `RecordReplay` call returns; higher-latency stores may intentionally choose weaker policies.

A saved host event must satisfy these requirements:

- It is associated with one session, one chunk, and one ordered event index.
- It is append-only relative to the committed event stream.
- It is atomic from the replay reader's point of view: replay sees either the complete event or no event.
- It contains enough information to replay the call without consulting the host effect: builtin name, builtin version, serialized positional arguments, serialized keyword arguments, and either a serialized result or serialized error.
- It can be validated against the replay-time call before its result or error is returned.

A `RecordReplay` call is replayable only after its event has been saved according to the selected storage policy. If a process crashes after a host effect occurs but before the corresponding event is saved, the storage policy determines whether that effect may be repeated during recovery. Applications that require stronger guarantees should choose a stricter persistence policy or make the underlying host effect idempotent.

The storage backend should be pluggable, but Dyson still needs a small logical store contract. Exact names are illustrative, but the interface should provide this shape:

```go
type Store interface {
    LoadSession(ctx context.Context, id SessionID) (SessionRecord, error)
    AppendChunk(ctx context.Context, chunk ChunkRecord) error
    ListChunks(ctx context.Context, id SessionID) ([]ChunkRecord, error)
    ListEvents(ctx context.Context, id SessionID, chunkIndex int) ([]HostEvent, error)
    AppendEvent(ctx context.Context, event HostEvent) error
    FinishChunk(ctx context.Context, outcome ChunkOutcome) error
}
```

`AppendChunk` records a chunk before it is executed if the application wants crash recovery for that chunk. `ListEvents` returns the committed prefix visible to recovery. `AppendEvent` must reject duplicate `(session, chunk, event_index)` writes or treat an exact duplicate as idempotent. `FinishChunk` records the final result, evaluation error, output transcript, and optional checksum. A buffered implementation may delay physical durability, but only events visible through `ListEvents` are replayable after a crash.

### Builtin replay policies

Not every Go builtin should be handled the same way. Dyson should make the policy explicit when registering a host builtin.

- `RecordReplay`: if an event exists at the current cursor, consume and validate it; otherwise call the real Go function and append its result or error. This is the default for effectful or non-deterministic host capabilities such as clocks, random values, filesystem reads, HTTP calls, database calls, LLM calls, UUIDs, and counters.
- `Reexecute`: always execute the Go function body. The outer call is not replayed from a recorded result, but nested effectful host calls still go through the event log. This is appropriate for deterministic helper builtins and for builtins that invoke Starlark callbacks whose side effects must run again during recovery.
- `NonReplayable`: the builtin cannot be used in durable sessions unless the application provides a custom replay strategy.

`RecordReplay` builtins must not mutate Starlark arguments or invoke callbacks with observable side effects. If they do, replaying only the recorded return value would skip those side effects. Such builtins should either be modeled as `Reexecute` or should record and replay explicit mutation events.

### Callbacks

A Go builtin may call back into Starlark by receiving a function argument and using `starlark.Call`. Callback support is possible, but it affects the replay policy.

For an opaque `RecordReplay` builtin, consuming an existing event returns the recorded result without running the Go body. Any Starlark callback side effects inside the original call would be skipped. Therefore, callback-capable builtins should generally be `Reexecute`, or their callbacks must be documented as observationally irrelevant.

A flat ordered event stream is enough for nested host calls during callbacks, but a structured event tree may be useful for debugging:

```text
host_call A starts
  starlark callback runs
    host_call B returns recorded result
host_call A returns
```

### Value serialization

Record-replay uses value serialization for host-event inputs and host-event outputs. The supported value layer should be independent of call-frame snapshotting.

Supported built-in Starlark values include:

- `None`, bool, int, float, string, and bytes.
- Tuple, list, dict, and set.
- Mutable containers as a graph, not just a tree, so aliases and cycles require object IDs/references.

Custom Go `starlark.Value` implementations must opt into durability. The intended shape is a converter/restorer contract:

- The value converts itself to a snapshot-specific payload made only of JSON-representable snapshot types.
- The event stores a stable type tag, such as `re.Pattern` or `re.Match`, a schema version, and the payload.
- A registered restorer reconstructs the Go value from that payload during event replay.

The custom value owns the semantics of its payload, including any internal cycle or alias handling needed by that custom type. The generic serializer should still validate the entire transitive Starlark value graph returned by a recorded builtin. It is not enough for the top-level result to be a list or dict; every contained Starlark value must also be supported or implement the converter/restorer contract. This gives Dyson a simple invariant: if a value can be returned from a recorded builtin, it can be written to the event log and restored later.

Go builtins themselves should be restored by registry name, not serialized as closures. Host functions should generally live in module/predeclared globals, not inside durable user data structures.

### Module loading

Dyson should not expose user-controlled dynamic `load()` as part of durable sessions. Session configuration should decide which modules are available before chunk zero, and those modules should be deterministic from the module registry and session capability manifest. Loading a module may introduce custom value types. That is acceptable if the module also registers the matching custom value restorers before replay reaches any event that needs them.

Starlark still has a `load` statement, and `starlark-go` executes it by calling `Thread.Load`. If Dyson does not support user-visible loads, it should reject `load` statements during chunk validation or install a `Thread.Load` implementation that fails with a deterministic policy error. If a future host application provides dynamic module loading, module loads should become recorded host events or the loader should expose its own durable state.

### Starlark execution API constraints

`starlark-go` provides useful execution primitives, but the public REPL API is not a perfect match for per-session capability control.

`ExecREPLChunk` has the closest REPL semantics: it compiles one syntactic file, resolves names against the existing globals dictionary, executes with unfrozen globals, and reflects changed globals back even after an error. This is the behavior Dyson wants for chunks. However, `ExecREPLChunk` hardcodes an empty predeclared environment and uses process-global `starlark.Universe` for universal builtins. It also documents itself as intended only for `go.starlark.net/repl`, with no API-stability guarantee.

The practical default should be to leave Dyson-specific capabilities out of `starlark.Universe` and install the allowed modules/builtins into the session globals before chunk zero. That gives each session its own capability surface: if a module is not installed in that session's globals and is not present in `Universe`, references to it are undefined. Frozen module values can still be shared safely across sessions as long as the values are immutable and any effectful builtin consults the current Dyson session runtime when called.

That still leaves a smaller capability-configuration tradeoff:

- Capabilities installed as initial globals are per-session and can be omitted for sessions that do not have them, but they are ordinary global names and can be rebound or shadowed by Starlark code. Rebinding removes or replaces the user's reference; it does not grant authority the session did not already have.
- Capabilities installed in `starlark.Universe` are immutable from Starlark and work naturally with `ExecREPLChunk`, but `Universe` is process-global, so it should be reserved for Starlark's standard universal names or for capabilities that every session in the process should see.
- A process-wide union of all possible capabilities in `Universe` can be made safe only if every builtin also checks the current session policy before doing work. That fails closed but makes unavailable capability names visible.
- A Dyson-owned or upstreamed REPL executor that accepts both the existing REPL globals predicate and a per-session predeclared/universe capability environment would only be necessary if Dyson needs per-session immutable or resolver-level-hidden capability names. `starlark-go` exposes `resolve.REPLChunk`, but the matching compiler package is internal, so Dyson cannot faithfully reimplement `ExecREPLChunk` outside `starlark-go` without forking, vendoring, or upstreaming an API.

`Thread.Local` should be treated as a narrow implementation hook, not durable session state. `starlark-go` documents that `SetLocal` must not be called after execution begins, and locals are not enumerable. Dyson may install a pointer to its session runtime before each chunk starts, but builtins should not lazily create hidden thread-local state during execution. Host state should instead live in deterministic module values, the Dyson session object, or the event log.

A session manifest should therefore record the exact Starlark file options, Dyson version, `starlark-go` version, capability configuration, builtin policy table, module versions, and custom restorer versions used by the session. The initial compatibility policy can be strict: if these do not match at restore time, fail instead of attempting cross-version replay.

### Output

`print` output and REPL display output are non-durable UI. Dyson does not record them as host events and does not use them for replay correctness. Hosts that implement `Thread.Print` with external side effects are responsible for making those side effects durable if they matter.

### Errors

Starlark does not have Python-style exception handling. A non-nil Go error returned by a builtin is converted into an evaluation error and aborts the current expression/chunk. Starlark code cannot catch it.

Dyson should still record builtin errors as host-event results. When consuming a saved event, returning the same error from the same event ensures the chunk aborts at the same point. This matters because later statements in the chunk must not run, and because mutations before the error may remain visible in the session.

For expected domain failures, prefer Starlark-visible data instead of Go errors. For example, APIs can expose `try_*` shapes that return `(value, None)` on success and `(None, "message")` on failure. Reserve Go errors for programmer errors, policy violations, resource limits, and unrecoverable host failures.

### Determinism and divergence

Recovery should fail fast on divergence. Examples of divergence include:

- The next `RecordReplay` call has a different builtin name, positional arguments, or keyword arguments than the next recorded host event.
- A recovering chunk produces fewer or more host events than the original committed chunk.
- A final chunk result, error, or optional globals checksum differs from the log.
- A required builtin, module, or custom value restorer is missing or has an incompatible version.

Divergence should be treated as a correctness failure, not as a cache miss.

## Questions

- Which store implementations should Dyson ship first, such as in-memory, JSONL, SQLite, object storage, or adapters over an application-provided store?
- Should Dyson initially accept `ExecREPLChunk` capability tradeoffs, or should it fork/vendor/upstream a REPL executor that accepts per-session predeclared and universal capability environments?
- How strict should argument verification be for host events whose inputs contain functions or other intentionally non-serializable values?
- Which builtins should be `RecordReplay`, `Reexecute`, or `NonReplayable` by default?
- How should recovery expose observability: event traces, backtraces, per-chunk checksums, or a deterministic diff report?
- Do we need fork support, where a restored session can branch into a new event log after a prefix of an existing session?
- What resource limits need to be recorded as part of the session configuration so recovery uses the same execution budget?

## Why not snapshot

A full interpreter snapshot is attractive but does not fit `starlark-go` well without modifying or depending on private internals.

- The public API does not expose enough construction hooks to serialize and faithfully reconstruct arbitrary `*starlark.Function` values. Functions carry private compiled code, defaults, free variables, and module state.
- Starlark functions created in earlier REPL chunks can observe the module state they were created with, not merely the latest external globals map. Replaying chunks preserves this history naturally.
- Go builtins are closures and cannot be serialized generically.
- Custom Go values may contain pointers, compiled regexps, open resources, caches, or other host state that requires explicit conversion/restoration.
- `*starlark.Thread` contains execution state and thread locals that should be recreated rather than persisted.
- Call frames are gone at REPL boundaries, and attempting to snapshot mid-call would require much deeper interpreter support.
- Record-replay gives better observability because the log explains which prompts and host effects produced the current state.

Value graph snapshots are still useful as an implementation detail for host-event payloads. They are not the primary session-resume mechanism.

## Why not Python

Python is a much larger and more ambient runtime than Starlark. It has broad filesystem/process/network access through the standard library, mutable interpreter globals, imports with side effects, exceptions, classes, metaprogramming, and many implementation details that are difficult to sandbox and replay faithfully.

Dyson deliberately uses Starlark because it is smaller, Go-embeddable, deterministic by default, and capability-oriented. The host decides which modules and builtins exist. That makes it easier to run generated or untrusted code with a narrow API surface, record every non-deterministic host interaction, and replay sessions without requiring a Python runtime.
