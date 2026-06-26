package re

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson/internal/xctx"
	"github.com/spachava753/dyson/snapshot"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/starlarktest"
	"go.starlark.net/syntax"
)

func TestLoadModuleShape(t *testing.T) {
	globals, err := LoadModule()
	be.Err(t, err, nil)
	be.Equal(t, len(globals), 1)

	module, ok := globals[ModuleName].(*starlarkstruct.Module)
	be.True(t, ok)
	be.True(t, module != nil)

	compile, err := module.Attr("compile")
	be.Err(t, err, nil)
	be.True(t, compile != nil)
}

func TestReTestdata(t *testing.T) {
	filename := filepath.Join("testdata", "re.star")
	src, err := os.ReadFile(filename)
	be.Err(t, err, nil)

	for _, chunk := range splitChunks(string(src)) {
		t.Run(chunk.name, func(t *testing.T) {
			thread := newTestThread(t)
			_, err := starlark.ExecFileOptions(&syntax.FileOptions{}, thread, filename, chunk.source, nil)
			if chunk.wantErr == "" {
				be.Err(t, err, nil)
				return
			}
			be.Err(t, err, chunk.wantErr)
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
			globals, err := LoadModule()
			be.Err(t, err, nil)

			_, err = starlark.EvalOptions(&syntax.FileOptions{}, thread, "test.star", tt.src, globals)
			be.Err(t, err, context.Canceled)
		})
	}
}

func TestPatternCanBeSnapshotted(t *testing.T) {
	globals, err := starlark.ExecFileOptions(&syntax.FileOptions{}, newTestThread(t), "test.star", `
load("re.star", "re")
pattern = re.compile("x", re.I)
`, nil)
	be.Err(t, err, nil)

	var buf bytes.Buffer
	be.Err(t, snapshot.NewEncoder(&buf).Encode(starlark.StringDict{"pattern": globals["pattern"]}), nil)

	var restored starlark.StringDict
	be.Err(t, snapshot.NewDecoder(bytes.NewReader(buf.Bytes())).Decode(&restored), nil)

	pattern, ok := restored["pattern"].(starlark.HasAttrs)
	be.True(t, ok)
	patternText, err := pattern.Attr("pattern")
	be.Err(t, err, nil)
	be.Equal(t, patternText, starlark.Value(starlark.String("x")))
	flags, err := pattern.Attr("flags")
	be.Err(t, err, nil)
	be.Equal(t, flags, starlark.Value(starlark.MakeInt(2)))
}

func TestMatchCanBeSnapshotted(t *testing.T) {
	globals, err := starlark.ExecFileOptions(&syntax.FileOptions{}, newTestThread(t), "test.star", `
load("re.star", "re")
match = re.search("(?P<word>[a-z]+)-(\\d+)", "xx abc-123 yy")
`, nil)
	be.Err(t, err, nil)

	var buf bytes.Buffer
	be.Err(t, snapshot.NewEncoder(&buf).Encode(starlark.StringDict{"match": globals["match"]}), nil)

	var restored starlark.StringDict
	be.Err(t, snapshot.NewDecoder(bytes.NewReader(buf.Bytes())).Decode(&restored), nil)

	match, ok := restored["match"].(starlark.HasAttrs)
	be.True(t, ok)
	span, err := match.Attr("span")
	be.Err(t, err, nil)
	spanValue, err := starlark.Call(&starlark.Thread{Name: "test"}, span, nil, nil)
	be.Err(t, err, nil)
	be.Equal(t, spanValue, starlark.Value(starlark.Tuple{starlark.MakeInt(3), starlark.MakeInt(10)}))
}

func splitChunks(src string) []testChunk {
	parts := strings.Split(src, "\n---\n")
	chunks := make([]testChunk, 0, len(parts))
	for i, part := range parts {
		part = strings.TrimSpace(part) + "\n"
		chunks = append(chunks, testChunk{
			name:    fmt.Sprintf("chunk_%02d", i+1),
			source:  stripErrorExpectations(part),
			wantErr: errorExpectation(part),
		})
	}
	return chunks
}

type testChunk struct {
	name    string
	source  string
	wantErr string
}

func stripErrorExpectations(src string) string {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		if before, _, ok := strings.Cut(line, "###"); ok {
			lines[i] = strings.TrimRight(before, " \t")
		}
	}
	return strings.Join(lines, "\n")
}

func errorExpectation(src string) string {
	for _, line := range strings.Split(src, "\n") {
		_, after, ok := strings.Cut(line, "###")
		if !ok {
			continue
		}
		want, err := strconv.Unquote(strings.TrimSpace(after))
		if err != nil {
			return strings.TrimSpace(after)
		}
		return want
	}
	return ""
}

func newTestThread(t *testing.T) *starlark.Thread {
	thread := &starlark.Thread{
		Name: "test",
		Load: func(thread *starlark.Thread, name string) (starlark.StringDict, error) {
			switch name {
			case ModuleName + ".star":
				return LoadModule()
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
