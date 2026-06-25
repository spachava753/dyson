package dyson

import (
	"fmt"
	"sync"

	stdlibre "github.com/spachava753/dyson/internal/stdlib/re"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

type moduleLoader func() (starlark.StringDict, error)

var stdlibModules = sync.OnceValue(func() map[string]moduleLoader {
	return map[string]moduleLoader{
		stdlibre.ModuleName: stdlibre.LoadModule,
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
	if len(globals) == 1 {
		if value, ok := globals[module]; ok {
			if moduleValue, ok := value.(*starlarkstruct.Module); ok && moduleValue != nil {
				return moduleValue.Members, nil
			}
		}
	}
	return globals, nil
}
