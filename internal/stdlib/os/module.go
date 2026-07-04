package os

import (
	_ "embed"
	"fmt"
	goos "os"
	"runtime"

	"github.com/spachava753/dyson/internal/xfs"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"
)

// ModuleName is the Starlark stdlib module name for Dyson's os compatibility module.
const ModuleName = "os"

//go:embed os.star
var source string

// Module is the default Starlark module namespace exposed by load("os.star", "os").
var Module = MakeModule(xfs.HostFS{Root: "."})

// MakeModule returns a Starlark os module backed by fsys. Path behavior is
// defined by fsys, so callers can choose host-style parent traversal or
// contained io/fs semantics.
func MakeModule(fsys xfs.FS) *starlarkstruct.Module {
	return loadModule(makePrimitiveModule(fsys))
}

func loadModule(primitives *starlarkstruct.Module) *starlarkstruct.Module {
	globals, err := starlark.ExecFileOptions(
		&syntax.FileOptions{While: true, Recursion: true},
		&starlark.Thread{Name: ModuleName + ".star"},
		ModuleName+".star",
		source,
		starlark.StringDict{
			"module": starlark.NewBuiltin("module", starlarkstruct.MakeModule),
			"_os":    primitives,

			"os_name": starlark.String(pythonOSName()),
			"pathsep": starlark.String(string(goos.PathListSeparator)),
			"devnull": starlark.String(goos.DevNull),

			"o_rdonly": starlark.MakeInt(goos.O_RDONLY),
			"o_wronly": starlark.MakeInt(goos.O_WRONLY),
			"o_rdwr":   starlark.MakeInt(goos.O_RDWR),
			"o_append": starlark.MakeInt(goos.O_APPEND),
			"o_creat":  starlark.MakeInt(goos.O_CREATE),
			"o_excl":   starlark.MakeInt(goos.O_EXCL),
			"o_sync":   starlark.MakeInt(goos.O_SYNC),
			"o_trunc":  starlark.MakeInt(goos.O_TRUNC),
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

func pythonOSName() string {
	if runtime.GOOS == "windows" {
		return "nt"
	}
	return "posix"
}
