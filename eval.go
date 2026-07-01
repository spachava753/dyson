package dyson

import (
	"context"

	"go.starlark.net/repl"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// Sphere owns one durable REPL session: the Starlark thread, globals, parser
// options, and the append-only chunk/event log produced by evaluation.
type Sphere struct {
	Log []ReplChunk

	t      *starlark.Thread
	g      starlark.StringDict
	fopts  *syntax.FileOptions
	codecs CodecRegistry
}

// NewSphere creates a REPL session with the provided print hook, loadable
// modules, and codec registry. Builtins in the supplied module dictionaries are
// replaced in-place with DurableBuiltin wrappers so calls can be recorded
// against this session.
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
		LoadBindsGlobally: false,
		Recursion:         false,
	}
	s := &Sphere{fopts: fopts, g: make(starlark.StringDict), codecs: codecs}

	// The module dictionaries are session configuration, so wrapping mutates them
	// before any Starlark code can observe the builtin values.
	for _, sd := range modules {
		for member, val := range sd {
			b, ok := val.(*starlark.Builtin)
			if !ok {
				continue
			}
			sd[member] = &DurableBuiltin{Builtin: b, s: s}
		}
	}

	l := repl.MakeLoadOptions(fopts)
	s.t = &starlark.Thread{
		Name:  "dyson.sphere",
		Print: print,
		Load: func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
			if m, ok := modules[module]; ok {
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
	s.Log = append(s.Log, ReplChunk{
		Code: code,
	})
	return starlark.ExecREPLChunk(f, s.t, s.g)
}
