package dyson

import (
	"context"
	"errors"

	"github.com/spachava753/dyson/internal/codec"
	"github.com/spachava753/dyson/internal/xctx"
	"go.starlark.net/repl"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"
)

// Sphere owns one durable REPL session: the Starlark thread, globals, parser
// options, codec registry, and the append-only chunk/event log produced by
// evaluation. A Sphere is not safe for concurrent use; callers must serialize
// calls to Eval and Replay and access to Log.
type Sphere struct {
	// log is the durable transcript of submitted chunks and recorded host calls.
	log []ReplChunk

	recordingEnabled bool

	// t and g are the live Starlark VM state for incremental REPL execution.
	t *starlark.Thread
	g starlark.StringDict

	// fopts defines the Starlark dialect Dyson accepts for every chunk.
	fopts *syntax.FileOptions

	// replaying switches wrapped builtins from executing host code to consuming
	// previously recorded calls from log[replayChunk].Calls[replayStep].
	replaying   bool
	replayChunk int
	replayStep  int

	// codecs controls which Starlark values can cross recorded host boundaries.
	codecs codec.Registry
}

// NewSphere creates a REPL session with the provided print hook, loadable
// modules, initial globals, and codec registry. Builtins in the supplied globals,
// module dictionaries, and nested module namespaces are wrapped per session so
// calls can be recorded against this session.
func NewSphere(
	print func(thread *starlark.Thread, msg string),
	modules map[string]starlark.StringDict,
	initialGlobals starlark.StringDict,
	codecs codec.Registry,
	record bool,
) *Sphere {
	fopts := &syntax.FileOptions{
		Set:               true,
		While:             true,
		TopLevelControl:   true,
		GlobalReassign:    true,
		LoadBindsGlobally: true,
		Recursion:         false,
	}
	s := &Sphere{
		fopts:            fopts,
		recordingEnabled: record,
		g:                make(starlark.StringDict, len(initialGlobals)),
		codecs:           codecs,
	}
	for name, val := range initialGlobals {
		s.g[name] = s.wrapSessionValue(val)
	}

	sessionModules := make(map[string]starlark.StringDict, len(modules))
	for module, sd := range modules {
		sessionSD := make(starlark.StringDict, len(sd))
		for member, val := range sd {
			sessionSD[member] = s.wrapSessionValue(val)
		}
		sessionModules[module] = sessionSD
	}

	l := repl.MakeLoadOptions(fopts)
	s.t = &starlark.Thread{
		Name:  "dyson.sphere",
		Print: print,
		Load: func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
			if m, ok := sessionModules[module]; ok {
				return m, nil
			}
			return l(thread, module)
		},
	}
	return s
}

func (s *Sphere) wrapSessionValue(val starlark.Value) starlark.Value {
	if b, ok := val.(*starlark.Builtin); ok {
		return &DurableBuiltin{Builtin: b, s: s}
	}
	if module, ok := val.(*starlarkstruct.Module); ok {
		members := make(starlark.StringDict, len(module.Members))
		freeze := true
		for name, member := range module.Members {
			// Catalog-only stdlib modules use nil values to reserve planned members.
			if member == nil {
				freeze = false
				members[name] = nil
				continue
			}
			members[name] = s.wrapSessionValue(member)
		}
		wrapped := &starlarkstruct.Module{Name: module.Name, Members: members}
		if freeze {
			wrapped.Freeze()
		}
		return wrapped
	}
	return val
}

// Eval executes one submitted Starlark REPL chunk and appends it to the log
// before execution so host calls can attach their events to the active chunk.
// If ctx ends during execution, Eval cancels the Starlark thread; each call
// clears cancellation left by a previous evaluation before it starts. Eval
// waits for its cancellation callback to stop before returning so that callback
// cannot interrupt the next sequential call.
func (s *Sphere) Eval(ctx context.Context, code string) error {
	s.t.Uncancel()
	// Thread.SetLocal is documented as setup-only. Dyson deliberately updates
	// this local between sequential REPL chunks, while no Starlark code is
	// running, so blocking builtins can receive the context for this Eval.
	xctx.WithContext(s.t, ctx)
	cancelDone := make(chan struct{})
	stopCancel := context.AfterFunc(ctx, func() {
		defer close(cancelDone)
		// Cancel interrupts interpreted Starlark; it is not resource cleanup
		// and cannot stop a blocking host builtin. Builtins receive ctx above
		// and must honor it themselves.
		s.t.Cancel(context.Cause(ctx).Error())
	})
	defer func() {
		// stopCancel does not wait when the callback has already started. Join it
		// so this call's cancellation cannot reach a later Eval.
		if !stopCancel() {
			<-cancelDone
		}
	}()

	f, err := s.fopts.Parse("<dyson_sphere_repl>", code, 0)
	if err != nil {
		return err
	}
	if s.recordingEnabled {
		s.log = append(s.log, ReplChunk{
			Code: code,
		})
	}
	return starlark.ExecREPLChunk(f, s.t, s.g)
}

// Log returns the session's recorded chunks and host-call events in evaluation
// order. The returned slice is the live log backing store; callers that need an
// immutable snapshot should copy it before retaining or modifying it.
func (s *Sphere) Log() []ReplChunk { return s.log }

// Replay rebuilds an empty session by executing each recorded chunk and
// satisfying wrapped host builtin calls from the supplied log instead of calling
// through to the host again. Replay fails if the receiver already has a log,
// because mixing existing execution history with a replay transcript would make
// the replay cursors ambiguous.
//
// Like Eval, Replay currently reserves ctx for the future executor layer because
// starlark.ExecREPLChunk does not accept a context.
func (s *Sphere) Replay(ctx context.Context, log []ReplChunk) error {
	s.replaying = true
	if len(s.log) != 0 {
		return errors.New("log is not empty")
	}
	s.log = log
	s.replayChunk = 0
	s.replayStep = 0
	defer func() {
		s.replaying = false
	}()
	for _, replChunk := range log {
		f, err := s.fopts.Parse("<dyson_sphere_repl>", replChunk.Code, 0)
		if err != nil {
			return err
		}
		if err := starlark.ExecREPLChunk(f, s.t, s.g); err != nil {
			return err
		}
		s.replayStep = 0
		s.replayChunk += 1
	}

	if !s.recordingEnabled {
		// Replay needs the supplied log while it is rebuilding state, but a
		// non-recording sphere should not retain that transcript afterward.
		s.log = nil
	}

	return nil
}
