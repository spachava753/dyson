package re

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson/internal/chunkedfile"
	"github.com/spachava753/dyson/internal/xctx"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarktest"
	"go.starlark.net/syntax"
)

func TestReTestdata(t *testing.T) {
	filename := filepath.Join("testdata", "re.star")
	for i, chunk := range chunkedfile.Read(filename, t) {
		t.Run(fmt.Sprintf("chunk_%02d", i+1), func(t *testing.T) {
			thread := newTestThread(t)
			_, err := starlark.ExecFileOptions(&syntax.FileOptions{}, thread, filename, chunk.Source, nil)
			if err != nil {
				chunk.GotErrorAnyLine(err.Error())
			}
			chunk.Done()
		})
	}
}

func TestComputeIntensiveBuiltinsReturnContextError(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{name: "compile", src: `re.compile("[a-z]+")`},
		{name: "search", src: `re.search("[a-z]+", "abc")`},
		{name: "split", src: `re.split("a", "banana")`},
		{name: "findall", src: `re.findall("a", "banana")`},
		{name: "finditer", src: `re.finditer("a", "banana")`},
		{name: "sub", src: `re.sub("a", "x", "banana")`},
		{name: "subn", src: `re.subn("a", "x", "banana")`},
		{name: "escape", src: `re.escape("a.b")`},
		{name: "pattern split", src: `re.compile("a").split("banana")`},
		{name: "pattern findall", src: `re.compile("a").findall("banana")`},
		{name: "match expand", src: `re.search("(a)", "banana").expand("\\1")`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			thread := newTestThread(t)
			xctx.WithContext(thread, ctx)
			globals := starlark.StringDict{
				ModuleName: Module,
			}

			_, err := starlark.EvalOptions(&syntax.FileOptions{}, thread, "test.star", tt.src, globals)
			be.Err(t, err, context.Canceled)
		})
	}
}

func newTestThread(t *testing.T) *starlark.Thread {
	thread := &starlark.Thread{
		Name: "test",
		Load: func(thread *starlark.Thread, name string) (starlark.StringDict, error) {
			switch name {
			case ModuleName + ".star":
				return starlark.StringDict{
					ModuleName: Module,
				}, nil
			case "assert.star":
				return starlarktest.LoadAssertModule()
			default:
				return nil, fmt.Errorf("unknown module %q", name)
			}
		},
	}
	starlarktest.SetReporter(thread, t)
	return thread
}
