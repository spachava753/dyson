package xctx

import (
	"context"

	"go.starlark.net/starlark"
)

const ctxKey = "dysonCtxKey"

func FromLocal(t *starlark.Thread) context.Context {
	if t == nil {
		return nil
	}
	c, _ := t.Local(ctxKey).(context.Context)
	return c
}

func WithContext(t *starlark.Thread, c context.Context) {
	t.SetLocal(ctxKey, c)
}

func Check(t *starlark.Thread) error {
	ctx := FromLocal(t)
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
