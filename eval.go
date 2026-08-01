package dyson

import (
	"context"
	"maps"

	"github.com/spachava753/dyson/internal/xctx"
	"go.starlark.net/repl"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// Sphere owns one incremental Starlark session. Globals persist across Eval
// calls. A Sphere is not safe for concurrent use.
type Sphere struct {
	t     *starlark.Thread
	g     starlark.StringDict
	fopts *syntax.FileOptions
}

// NewSphere creates a Starlark session with the provided print hook, loadable
// modules, and initial globals.
func NewSphere(
	print func(thread *starlark.Thread, msg string),
	modules map[string]starlark.StringDict,
	initialGlobals starlark.StringDict,
) *Sphere {
	fopts := &syntax.FileOptions{
		Set:               true,
		While:             true,
		TopLevelControl:   true,
		GlobalReassign:    true,
		LoadBindsGlobally: true,
		Recursion:         false,
	}
	globals := maps.Clone(initialGlobals)
	if globals == nil {
		globals = starlark.StringDict{}
	}

	// Snapshot the loader dictionaries so later caller changes do not alter an
	// existing session. The Starlark values themselves are intentionally shared.
	sessionModules := make(map[string]starlark.StringDict, len(modules))
	for module, globals := range modules {
		sessionModules[module] = maps.Clone(globals)
	}

	loadFallback := repl.MakeLoadOptions(fopts)
	s := &Sphere{fopts: fopts, g: globals}
	s.t = &starlark.Thread{
		Name:  "dyson.sphere",
		Print: print,
		Load: func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
			if globals, ok := sessionModules[module]; ok {
				return globals, nil
			}
			return loadFallback(thread, module)
		},
	}
	return s
}

// Eval executes one Starlark REPL chunk. If ctx ends during execution, Eval
// cancels the Starlark thread; each call clears cancellation left by a previous
// evaluation. Eval waits for its cancellation callback to stop before returning
// so that callback cannot interrupt the next sequential call.
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

	file, err := s.fopts.Parse("<dyson_sphere_repl>", code, 0)
	if err != nil {
		return err
	}
	return starlark.ExecREPLChunk(file, s.t, s.g)
}
