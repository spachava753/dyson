package dyson

import (
	"bytes"
	"testing"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson/snapshot"
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

func TestReMatchCanBeSnapshotted(t *testing.T) {
	globals, err := starlark.ExecFileOptions(&syntax.FileOptions{}, &starlark.Thread{Name: "test", Load: Load}, "test.star", `
load("re", "search")
match = search("(?P<word>[a-z]+)-(\\d+)", "xx abc-123 yy")
`, nil)
	be.Err(t, err, nil)

	var buf bytes.Buffer
	be.Err(t, snapshot.NewEncoder(&buf).Encode(starlark.StringDict{"match": globals["match"]}), nil)

	var restored starlark.StringDict
	be.Err(t, snapshot.NewDecoder(bytes.NewReader(buf.Bytes())).Decode(&restored), nil)

	match, ok := restored["match"].(starlark.HasAttrs)
	be.True(t, ok)
	group, err := match.Attr("group")
	be.Err(t, err, nil)
	groupValue, err := starlark.Call(&starlark.Thread{Name: "test"}, group, starlark.Tuple{starlark.MakeInt(0), starlark.String("word"), starlark.MakeInt(2)}, nil)
	be.Err(t, err, nil)
	be.Equal(t, groupValue, starlark.Value(starlark.Tuple{starlark.String("abc-123"), starlark.String("abc"), starlark.String("123")}))

	span, err := match.Attr("span")
	be.Err(t, err, nil)
	spanValue, err := starlark.Call(&starlark.Thread{Name: "test"}, span, nil, nil)
	be.Err(t, err, nil)
	be.Equal(t, spanValue, starlark.Value(starlark.Tuple{starlark.MakeInt(3), starlark.MakeInt(10)}))

	input, err := match.Attr("string")
	be.Err(t, err, nil)
	be.Equal(t, input, starlark.Value(starlark.String("xx abc-123 yy")))

	patternValue, err := match.Attr("re")
	be.Err(t, err, nil)
	pattern, ok := patternValue.(starlark.HasAttrs)
	be.True(t, ok)
	patternText, err := pattern.Attr("pattern")
	be.Err(t, err, nil)
	be.Equal(t, patternText, starlark.Value(starlark.String("(?P<word>[a-z]+)-(\\d+)")))
}

func TestLoadRejectsUnknownStdlibModule(t *testing.T) {
	_, err := Load(&starlark.Thread{Name: "test"}, "math")
	be.Err(t, err, `unknown stdlib module "math"`)
}
