package glob

import (
	_ "embed"
	"fmt"

	stdlibos "github.com/spachava753/dyson/internal/stdlib/os"
	stdlibre "github.com/spachava753/dyson/internal/stdlib/re"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"
)

// ModuleName is the Starlark stdlib module name for Dyson's glob compatibility module.
const ModuleName = "glob"

//go:embed glob.star
var source string

// Module is the default Starlark module namespace exposed by load("glob.star", "glob").
var Module = MakeModule(stdlibos.Module)

// MakeModule returns a Starlark glob module using osModule for filesystem access.
func MakeModule(osModule *starlarkstruct.Module) *starlarkstruct.Module {
	return loadModule(osModule)
}

func loadModule(osModule *starlarkstruct.Module) *starlarkstruct.Module {
	globals, err := starlark.ExecFileOptions(
		&syntax.FileOptions{While: true, Recursion: true},
		&starlark.Thread{Name: ModuleName + ".star"},
		ModuleName+".star",
		source,
		starlark.StringDict{
			"module": starlark.NewBuiltin("module", starlarkstruct.MakeModule),
			"os":     osModule,
			"re":     stdlibre.Module,
		},
	)
	if err != nil {
		panic(fmt.Sprintf("load %s.star: %v", ModuleName, err))
	}

	module, ok := globals[ModuleName].(*starlarkstruct.Module)
	if !ok {
		panic(fmt.Sprintf("load %s.star: global %q is %T", ModuleName, ModuleName, globals[ModuleName]))
	}
	return module
}
