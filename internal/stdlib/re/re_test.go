package re

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/nalgeon/be"
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
			case ModuleName:
				module, err := LoadModule()
				if err != nil {
					return nil, err
				}
				value := module[ModuleName]
				moduleValue, ok := value.(*starlarkstruct.Module)
				if !ok || moduleValue == nil {
					return nil, fmt.Errorf("module %q did not load as a Starlark module", name)
				}
				return moduleValue.Members, nil
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
