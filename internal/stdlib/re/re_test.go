package re

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nalgeon/be"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
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

func TestModuleScaffold(t *testing.T) {
	globals := runTestFile(t, `
load("re", "compile", "purge", "A", "ASCII", "I", "IGNORECASE", "L", "LOCALE", "M", "MULTILINE", "S", "DOTALL", "U", "UNICODE", "X", "VERBOSE", "DEBUG", "NOFLAG", "RegexFlag", "Pattern", "Match", "PatternError", "error")

pattern = compile("[a-z]+", I | M)
recompiled_pattern = compile(pattern)
module_pattern = compile("x")
purge_result = purge()
type_name = type(pattern)
pattern_kind = pattern["kind"]
pattern_text = pattern["pattern"]
pattern_flags = pattern["flags"]
pattern_groups = pattern["groups"]
pattern_groupindex = pattern["groupindex"]
pattern_attrs = pattern["attrs"]
pattern_type_attrs = Pattern["attrs"]
match_type_attrs = Match["attrs"]
pattern_error_attrs = PatternError["attrs"]
module_pattern_kind = module_pattern["kind"]
module_pattern_text = module_pattern["pattern"]
recompiled_ok = recompiled_pattern == pattern
aliases_ok = A == ASCII and I == IGNORECASE and M == MULTILINE and PatternError == error and NOFLAG == 0
flag_values = (NOFLAG, I, L, M, S, U, X, DEBUG, A)
type_placeholders = (RegexFlag["name"], Pattern["name"], Match["name"], PatternError["name"])
`)

	be.Equal(t, globals["type_name"], starlark.Value(starlark.String("dict")))
	be.Equal(t, globals["pattern_kind"], starlark.Value(starlark.String("re.Pattern")))
	be.Equal(t, globals["pattern_text"], starlark.Value(starlark.String("[a-z]+")))
	be.Equal(t, globals["pattern_flags"], starlark.Value(starlark.MakeInt(flagIgnoreCase|flagMultiline)))
	be.Equal(t, globals["pattern_groups"], starlark.Value(starlark.MakeInt(0)))
	be.Equal(t, globals["aliases_ok"], starlark.Value(starlark.True))
	be.Equal(t, globals["recompiled_ok"], starlark.Value(starlark.True))
	be.Equal(t, globals["flag_values"], starlark.Value(starlark.Tuple{
		starlark.MakeInt(0),
		starlark.MakeInt(2),
		starlark.MakeInt(4),
		starlark.MakeInt(8),
		starlark.MakeInt(16),
		starlark.MakeInt(32),
		starlark.MakeInt(64),
		starlark.MakeInt(128),
		starlark.MakeInt(256),
	}))
	be.Equal(t, globals["purge_result"], starlark.Value(starlark.None))
	be.Equal(t, globals["module_pattern_kind"], starlark.Value(starlark.String("re.Pattern")))
	be.Equal(t, globals["module_pattern_text"], starlark.Value(starlark.String("x")))

	groupindex, ok := globals["pattern_groupindex"].(*starlark.Dict)
	be.True(t, ok)
	be.Equal(t, groupindex.Len(), 0)

	attrs := starlarkListStrings(t, globals["pattern_attrs"])
	for _, name := range []string{"search", "match", "fullmatch", "split", "findall", "finditer", "sub", "subn", "pattern", "flags", "groups", "groupindex"} {
		be.True(t, containsString(attrs, name))
	}

	patternTypeAttrs := starlarkListStrings(t, globals["pattern_type_attrs"])
	for _, name := range []string{"search", "match", "fullmatch", "split", "findall", "finditer", "sub", "subn", "pattern", "flags", "groups", "groupindex"} {
		be.True(t, containsString(patternTypeAttrs, name))
	}

	matchTypeAttrs := starlarkListStrings(t, globals["match_type_attrs"])
	for _, name := range []string{"expand", "group", "groups", "groupdict", "start", "end", "span", "pos", "endpos", "lastindex", "lastgroup", "re", "string"} {
		be.True(t, containsString(matchTypeAttrs, name))
	}

	patternErrorAttrs := starlarkListStrings(t, globals["pattern_error_attrs"])
	for _, name := range []string{"msg", "pattern", "pos", "lineno", "colno"} {
		be.True(t, containsString(patternErrorAttrs, name))
	}

	typePlaceholders, ok := globals["type_placeholders"].(starlark.Tuple)
	be.True(t, ok)
	be.Equal(t, typePlaceholders, starlark.Tuple{
		starlark.String("re.RegexFlag"),
		starlark.String("re.Pattern"),
		starlark.String("re.Match"),
		starlark.String("re.PatternError"),
	})
}

func TestUnimplementedOperationsAbort(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantErrs []string
	}{
		{
			name: "module search",
			src: `
load("re", "search")
search("x", "x")
`,
			wantErrs: []string{"re.search is not implemented"},
		},
		{
			name: "compiled pattern accepted by module search",
			src: `
load("re", "compile", "search")
pattern = compile("x")
search(pattern, "x")
`,
			wantErrs: []string{"re.search is not implemented"},
		},
		{
			name: "escape",
			src: `
load("re", "escape")
escape("a.b")
`,
			wantErrs: []string{"re.escape is not implemented"},
		},
		{
			name: "compile type check",
			src: `
load("re", "compile")
compile(123)
`,
			wantErrs: []string{"re.compile: pattern must be str or bytes, got int"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runTestFileExpectErr(t, tt.src)
			assertErrorContains(t, err, tt.wantErrs...)
		})
	}
}

func runTestFile(t *testing.T, src string) starlark.StringDict {
	t.Helper()

	globals, err := starlark.ExecFileOptions(&syntax.FileOptions{}, newTestThread(), "test.star", strings.TrimSpace(src)+"\n", nil)
	be.Err(t, err, nil)
	return globals
}

func runTestFileExpectErr(t *testing.T, src string) error {
	t.Helper()

	_, err := starlark.ExecFileOptions(&syntax.FileOptions{}, newTestThread(), "test.star", strings.TrimSpace(src)+"\n", nil)
	be.Err(t, err)
	return err
}

func newTestThread() *starlark.Thread {
	return &starlark.Thread{
		Name: "test",
		Load: func(thread *starlark.Thread, name string) (starlark.StringDict, error) {
			if name != ModuleName {
				return nil, fmt.Errorf("unknown module %q", name)
			}
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
		},
	}
}

func starlarkListStrings(t *testing.T, value starlark.Value) []string {
	t.Helper()

	list, ok := value.(*starlark.List)
	be.True(t, ok)

	values := make([]string, 0, list.Len())
	for item := range list.Elements() {
		text, ok := item.(starlark.String)
		be.True(t, ok)
		values = append(values, string(text))
	}
	return values
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func assertErrorContains(t *testing.T, err error, wants ...string) {
	t.Helper()

	for _, want := range wants {
		be.Err(t, err, want)
	}
}
