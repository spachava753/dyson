package xctx

import (
	"context"
	"testing"

	"github.com/nalgeon/be"
	"go.starlark.net/starlark"
)

type contextKey struct{}

func TestWithContextUpdatesLocalBetweenExecutions(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	first := context.WithValue(t.Context(), contextKey{}, "first")
	WithContext(thread, first)

	_, err := starlark.ExecFile(thread, "test.star", `value = 1`, nil)
	be.Err(t, err, nil)

	second := context.WithValue(t.Context(), contextKey{}, "second")
	WithContext(thread, second)
	if FromLocal(thread) != second {
		t.Fatal("thread did not expose the updated context")
	}
}
