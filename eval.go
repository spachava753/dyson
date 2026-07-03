package dyson

import (
	"context"
	"errors"

	"go.starlark.net/repl"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// Sphere owns one durable REPL session: the Starlark thread, globals, parser
// options, codec registry, and the append-only chunk/event log produced by
// evaluation.
type Sphere struct {
	// log is the durable transcript of submitted chunks and recorded host calls.
	log []ReplChunk

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
	codecs CodecRegistry
}

// NewSphere creates a REPL session with the provided print hook, loadable
// modules, and codec registry. Builtins in the supplied module dictionaries are
// wrapped per session so calls can be recorded against this session.
func NewSphere(
	print func(thread *starlark.Thread, msg string),
	modules map[string]starlark.StringDict,
	codecs CodecRegistry,
) *Sphere {
	fopts := &syntax.FileOptions{
		Set:               true,
		While:             true,
		TopLevelControl:   true,
		GlobalReassign:    true,
		LoadBindsGlobally: true,
		Recursion:         false,
	}
	s := &Sphere{fopts: fopts, g: make(starlark.StringDict), codecs: codecs}

	sessionModules := make(map[string]starlark.StringDict, len(modules))
	for module, sd := range modules {
		sessionSD := make(starlark.StringDict, len(sd))
		for member, val := range sd {
			if b, ok := val.(*starlark.Builtin); ok {
				sessionSD[member] = &DurableBuiltin{Builtin: b, s: s}
				continue
			}
			sessionSD[member] = val
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

// Eval executes one submitted Starlark REPL chunk and appends it to the log
// before execution so host calls can attach their events to the active chunk.
//
// Note: [starlark.ExecREPLChunk] provides the REPL semantics Dyson wants and
// automatically brings in [starlark.Universe], but it does not accept a context;
// ctx is reserved for the future executor/replay layer.
func (s *Sphere) Eval(ctx context.Context, code string) error {
	f, err := s.fopts.Parse("<dyson_sphere_repl>", code, 0)
	if err != nil {
		return err
	}
	s.log = append(s.log, ReplChunk{
		Code: code,
	})
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

	return nil
}
