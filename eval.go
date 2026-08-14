package dyson

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/spachava753/dyson/internal/xctx"
	"github.com/spachava753/starlarkx/starlark"
	"github.com/spachava753/starlarkx/syntax"
)

var errSphereClosed = errors.New("dyson: sphere is closed")

// SphereSource contributes loadable modules and globals to a Sphere and owns any
// resources retained by those values. NewSphere snapshots its dictionaries and
// calls Close during Sphere.Close. Close must be idempotent.
type SphereSource interface {
	Modules() map[string]starlark.StringDict
	Globals() starlark.StringDict
	Close() error
}

// ModuleSet adapts custom loadable modules into a SphereSource.
type ModuleSet map[string]starlark.StringDict

// Modules returns a snapshot of the custom modules.
func (m ModuleSet) Modules() map[string]starlark.StringDict { return cloneModules(m) }

// Globals returns no globals.
func (ModuleSet) Globals() starlark.StringDict { return nil }

// Close performs no cleanup for a plain module set.
func (ModuleSet) Close() error { return nil }

// GlobalSet adapts custom initial globals into a SphereSource.
type GlobalSet starlark.StringDict

// Modules returns no modules.
func (GlobalSet) Modules() map[string]starlark.StringDict { return nil }

// Globals returns a snapshot of the custom globals.
func (g GlobalSet) Globals() starlark.StringDict {
	return maps.Clone(starlark.StringDict(g))
}

// Close performs no cleanup for a plain global set.
func (GlobalSet) Close() error { return nil }

// Sphere owns one incremental Starlark session. Globals persist across Eval
// calls. Close releases session-owned files. A Sphere is not safe for concurrent
// use.
type Sphere struct {
	t       *starlark.Thread
	g       starlark.StringDict
	fopts   *syntax.FileOptions
	sources []SphereSource
	closed  bool
}

func cloneModules(modules map[string]starlark.StringDict) map[string]starlark.StringDict {
	cloned := make(map[string]starlark.StringDict, len(modules))
	for name, globals := range modules {
		cloned[name] = maps.Clone(globals)
	}
	return cloned
}

// NewSphere creates a Starlark session from explicit sources. With no sources,
// the session contains only Starlark's built-in universe. Later sources replace
// modules and globals with matching names.
func NewSphere(
	print func(thread *starlark.Thread, msg string),
	sources ...SphereSource,
) *Sphere {
	sessionModules := map[string]starlark.StringDict{}
	globals := starlark.StringDict{}
	for _, source := range sources {
		maps.Copy(sessionModules, cloneModules(source.Modules()))
		maps.Copy(globals, source.Globals())
	}

	fopts := &syntax.FileOptions{
		Set:               true,
		While:             true,
		TopLevelControl:   true,
		GlobalReassign:    true,
		LoadBindsGlobally: true,
		Recursion:         false,
	}
	s := &Sphere{fopts: fopts, g: globals, sources: slices.Clone(sources)}
	s.t = &starlark.Thread{
		Name:  "dyson.sphere",
		Print: print,
		Load: func(_ *starlark.Thread, module string) (starlark.StringDict, error) {
			if globals, ok := sessionModules[module]; ok {
				return globals, nil
			}
			return nil, fmt.Errorf("load: module %q is not registered", module)
		},
	}
	return s
}

// Close releases all files still owned by the session and prevents further
// evaluation. It is safe to call Close more than once.
func (s *Sphere) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	sources := s.sources
	s.sources = nil

	var closeErrors []error
	for _, source := range sources {
		if err := source.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	return errors.Join(closeErrors...)
}

// EvaluationContext returns the context supplied to the Sphere.Eval call
// currently executing on thread. It returns context.Background when thread is
// nil or is not managed by a Sphere. Builtins must not retain the returned
// context beyond the current call.
func EvaluationContext(thread *starlark.Thread) context.Context {
	if ctx := xctx.FromLocal(thread); ctx != nil {
		return ctx
	}
	return context.Background()
}

// Eval executes one Starlark REPL chunk. If ctx ends during execution, Eval
// cancels the Starlark thread; each call clears cancellation left by a previous
// evaluation. Eval waits for its cancellation callback to stop before returning
// so that callback cannot interrupt the next sequential call.
func (s *Sphere) Eval(ctx context.Context, code string) error {
	if s.closed {
		return errSphereClosed
	}
	s.t.Uncancel()
	// Thread.SetLocal is documented as setup-only. Dyson deliberately updates
	// this local between sequential REPL chunks, while no Starlark code is
	// running, so blocking builtins can receive the context for this Eval.
	xctx.WithContext(s.t, ctx)
	cancelDone := make(chan struct{})
	stopCancel := context.AfterFunc(ctx, func() {
		defer close(cancelDone)
		// Cancel interrupts interpreted Starlark; it is not resource cleanup
		// and cannot stop a blocking host builtin. Context-aware builtins can
		// read ctx above and must honor it themselves; file I/O is deliberately
		// synchronous and remains blocking.
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
