package dyson

import (
	"context"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/nalgeon/be"
	"go.starlark.net/starlark"
)

func TestEvalStopsWhenContextEnds(t *testing.T) {
	s := NewSphere(nil, nil, nil)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := s.Eval(ctx, `while True: pass`)
	if err == nil || !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("Eval() error = %v, want cancellation error", err)
	}

	be.Err(t, s.Eval(t.Context(), `completed = True`), nil)
}

// delayedCancellation pauses the cancellation callback immediately before
// Thread.Cancel, after context propagation has already completed.
type delayedCancellation struct {
	started chan struct{}
	release chan struct{}
}

func (e *delayedCancellation) Error() string {
	close(e.started)
	<-e.release
	return context.Canceled.Error()
}

func TestEvalCancellationCannotOutliveCall(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		printStarted := make(chan struct{})
		releasePrint := make(chan struct{})
		s := NewSphere(func(_ *starlark.Thread, _ string) {
			close(printStarted)
			<-releasePrint
		}, nil, nil)
		cause := &delayedCancellation{
			started: make(chan struct{}),
			release: make(chan struct{}),
		}
		ctx, cancel := context.WithCancelCause(t.Context())
		result := make(chan error, 1)
		go func() {
			result <- s.Eval(ctx, `print("running")`)
		}()

		<-printStarted
		cancel(cause)
		<-cause.started
		close(releasePrint)
		synctest.Wait()

		select {
		case err := <-result:
			close(cause.release)
			t.Fatalf("Eval returned before its cancellation callback: %v", err)
		default:
		}

		close(cause.release)
		<-result
		be.Err(t, s.Eval(t.Context(), `completed = True`), nil)
	})
}

func TestNewSphereAcceptsInitialGlobals(t *testing.T) {
	initialGlobals := starlark.StringDict{
		"answer": starlark.MakeInt(21),
		"double": starlark.NewBuiltin("double", func(
			thread *starlark.Thread,
			fn *starlark.Builtin,
			args starlark.Tuple,
			kwargs []starlark.Tuple,
		) (starlark.Value, error) {
			var value int
			if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "value", &value); err != nil {
				return nil, err
			}
			return starlark.MakeInt(value * 2), nil
		}),
	}
	s := NewSphere(nil, nil, initialGlobals)

	be.Err(t, s.Eval(t.Context(), `
if double(answer) != 42:
    fail("initial globals unavailable")
`), nil)
}

func TestEvalPreservesGlobalsAndLoadedModules(t *testing.T) {
	modules := map[string]starlark.StringDict{
		"answer.star": {"answer": starlark.MakeInt(21)},
	}
	s := NewSphere(nil, modules, nil)

	be.Err(t, s.Eval(t.Context(), `
load("answer.star", "answer")
def double(value):
    return value * 2
`), nil)
	be.Err(t, s.Eval(t.Context(), `
if double(answer) != 42:
    fail("session globals were not preserved")
`), nil)
}
