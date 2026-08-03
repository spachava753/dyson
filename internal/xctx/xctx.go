package xctx

import (
	"context"

	"go.starlark.net/starlark"
)

const ctxKey = "dysonCtxKey"

// FromLocal returns the context attached to thread, or nil when none is attached.
func FromLocal(thread *starlark.Thread) context.Context {
	if thread == nil {
		return nil
	}
	ctx, _ := thread.Local(ctxKey).(context.Context)
	return ctx
}

// WithContext attaches ctx to thread for use by context-aware builtins.
func WithContext(thread *starlark.Thread, ctx context.Context) {
	thread.SetLocal(ctxKey, ctx)
}

// Check returns the attached context's error, or nil when no context is attached.
func Check(thread *starlark.Thread) error {
	ctx := FromLocal(thread)
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
