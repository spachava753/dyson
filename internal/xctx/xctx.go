package xctx

import (
	"context"

	"go.starlark.net/starlark"
)

const ctxKey = "dysonCtxKey"

func FromLocal(thread *starlark.Thread) context.Context {
	if thread == nil {
		return nil
	}
	ctx, _ := thread.Local(ctxKey).(context.Context)
	return ctx
}

func WithContext(thread *starlark.Thread, ctx context.Context) {
	thread.SetLocal(ctxKey, ctx)
}

func Check(thread *starlark.Thread) error {
	ctx := FromLocal(thread)
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
