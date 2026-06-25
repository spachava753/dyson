package dyson

import (
	"bytes"
	"testing"

	"github.com/nalgeon/be"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

func TestLoadStdlibModule(t *testing.T) {
	globals, err := Load(&starlark.Thread{Name: "test"}, "re")
	be.Err(t, err, nil)
	be.True(t, globals["compile"] != nil)
	be.True(t, globals["I"] != nil)
}

func TestReCompiledPatternCanBeSnapshotted(t *testing.T) {
	globals, err := starlark.ExecFileOptions(&syntax.FileOptions{}, &starlark.Thread{Name: "test", Load: Load}, "test.star", `
load("re", "compile", "I")
pattern = compile("x", I)
`, nil)
	be.Err(t, err, nil)

	var buf bytes.Buffer
	be.Err(t, NewEncoder(&buf).Encode(starlark.StringDict{"pattern": globals["pattern"]}), nil)

	var restored starlark.StringDict
	be.Err(t, NewDecoder(bytes.NewReader(buf.Bytes())).Decode(&restored), nil)

	pattern, ok := restored["pattern"].(*starlark.Dict)
	be.True(t, ok)
	kind, found, err := pattern.Get(starlark.String("kind"))
	be.Err(t, err, nil)
	be.True(t, found)
	be.Equal(t, kind, starlark.Value(starlark.String("re.Pattern")))
}

func TestLoadRejectsUnknownStdlibModule(t *testing.T) {
	_, err := Load(&starlark.Thread{Name: "test"}, "math")
	be.Err(t, err)
	assertErrorContains(t, err, `unknown stdlib module "math"`)
}
