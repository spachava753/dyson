package dyson_test

import (
	"context"
	"testing"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson"
	"go.starlark.net/starlark"
)

type evaluationContextKey struct{}

func TestEvaluationContextExposesCurrentEvalContextToExternalBuiltin(t *testing.T) {
	var contexts []context.Context
	captureContext := starlark.NewBuiltin("capture_context", func(
		thread *starlark.Thread,
		_ *starlark.Builtin,
		_ starlark.Tuple,
		_ []starlark.Tuple,
	) (starlark.Value, error) {
		contexts = append(contexts, dyson.EvaluationContext(thread))
		return starlark.None, nil
	})
	sphere := dyson.NewSphere(nil, dyson.GlobalSet{"capture_context": captureContext})
	first := context.WithValue(t.Context(), evaluationContextKey{}, "first")
	second := context.WithValue(t.Context(), evaluationContextKey{}, "second")

	be.Err(t, sphere.Eval(first, `capture_context()`), nil)
	be.Err(t, sphere.Eval(second, `capture_context()`), nil)

	be.Equal(t, len(contexts), 2)
	if contexts[0] != first || contexts[1] != second {
		t.Fatalf("builtin contexts = [%p, %p], want [%p, %p]", contexts[0], contexts[1], first, second)
	}
}

func TestEvaluationContextDefaultsToBackground(t *testing.T) {
	be.Equal(t, dyson.EvaluationContext(nil), context.Background())
	be.Equal(t, dyson.EvaluationContext(&starlark.Thread{}), context.Background())
}
