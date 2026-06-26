package dyson

import (
	"fmt"
	"sync"

	stdlibos "github.com/spachava753/dyson/internal/stdlib/os"
	stdlibre "github.com/spachava753/dyson/internal/stdlib/re"
	stdlibtime "github.com/spachava753/dyson/internal/stdlib/time"
	"go.starlark.net/starlark"
)

type moduleLoader func() (starlark.StringDict, error)

var stdlibModules = sync.OnceValue(func() map[string]moduleLoader {
	return map[string]moduleLoader{
		stdlibos.ModuleName + ".star":   stdlibos.LoadModule,
		stdlibre.ModuleName + ".star":   stdlibre.LoadModule,
		stdlibtime.ModuleName + ".star": stdlibtime.LoadModule,
	}
})

// Load is a starlark.Thread.Load implementation for Dyson's loadable Python
// standard-library compatibility modules.
func Load(thread *starlark.Thread, module string) (starlark.StringDict, error) {
	_ = thread

	modules := stdlibModules()
	loader, ok := modules[module]
	if !ok {
		return nil, fmt.Errorf("unknown stdlib module %q", module)
	}

	globals, err := loader()
	if err != nil {
		return nil, err
	}
	return globals, nil
}
