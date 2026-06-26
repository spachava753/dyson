package dyson

import (
	"testing"

	"github.com/nalgeon/be"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

func TestLoadStdlibModule(t *testing.T) {
	globals, err := Load(&starlark.Thread{Name: "test"}, "re.star")
	be.Err(t, err, nil)
	be.True(t, globals["re"] != nil)
	be.True(t, globals["compile"] == nil)
	be.True(t, globals["I"] == nil)
}

func TestLoadTimeStdlibModule(t *testing.T) {
	globals, err := Load(&starlark.Thread{Name: "test"}, "time.star")
	be.Err(t, err, nil)
	be.True(t, globals["time"] != nil)
	be.True(t, globals["sleep"] == nil)
}

func TestLoadOSStdlibModule(t *testing.T) {
	globals, err := Load(&starlark.Thread{Name: "test"}, "os.star")
	be.Err(t, err, nil)
	be.True(t, globals["os"] != nil)
	be.True(t, globals["getcwd"] == nil)
}

func TestLoadStdlibModuleRejectsDirectModuleName(t *testing.T) {
	_, err := Load(&starlark.Thread{Name: "test"}, "re")
	be.Err(t, err, `unknown stdlib module "re"`)
}

func TestLoadStdlibModuleNamespace(t *testing.T) {
	globals, err := starlark.ExecFileOptions(&syntax.FileOptions{}, &starlark.Thread{Name: "test", Load: Load}, "test.star", `
load("re.star", "re")
text = "The order number is 98765, please process it."
match = re.search("\\d+", text)
matched = match.group() if match else None
`, nil)
	be.Err(t, err, nil)
	be.Equal(t, globals["matched"], starlark.Value(starlark.String("98765")))
}

func TestLoadRejectsUnknownStdlibModule(t *testing.T) {
	_, err := Load(&starlark.Thread{Name: "test"}, "math")
	be.Err(t, err, `unknown stdlib module "math"`)
}
