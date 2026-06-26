package time

import (
	"fmt"
	"sync"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

const ModuleName = "time"

var module = sync.OnceValue(func() starlark.StringDict {
	members := starlark.StringDict{
		"time":            starlark.NewBuiltin(ModuleName+".time", notImplemented),
		"time_ns":         starlark.NewBuiltin(ModuleName+".time_ns", notImplemented),
		"monotonic":       starlark.NewBuiltin(ModuleName+".monotonic", notImplemented),
		"monotonic_ns":    starlark.NewBuiltin(ModuleName+".monotonic_ns", notImplemented),
		"perf_counter":    starlark.NewBuiltin(ModuleName+".perf_counter", notImplemented),
		"perf_counter_ns": starlark.NewBuiltin(ModuleName+".perf_counter_ns", notImplemented),
		"process_time":    starlark.NewBuiltin(ModuleName+".process_time", unsupportedCPUClock),
		"process_time_ns": starlark.NewBuiltin(ModuleName+".process_time_ns", unsupportedCPUClock),
		"thread_time":     starlark.NewBuiltin(ModuleName+".thread_time", unsupportedCPUClock),
		"thread_time_ns":  starlark.NewBuiltin(ModuleName+".thread_time_ns", unsupportedCPUClock),
		"sleep":           starlark.NewBuiltin(ModuleName+".sleep", notImplemented),
		"gmtime":          starlark.NewBuiltin(ModuleName+".gmtime", notImplemented),
		"localtime":       starlark.NewBuiltin(ModuleName+".localtime", notImplemented),
		"mktime":          starlark.NewBuiltin(ModuleName+".mktime", notImplemented),
		"asctime":         starlark.NewBuiltin(ModuleName+".asctime", notImplemented),
		"ctime":           starlark.NewBuiltin(ModuleName+".ctime", notImplemented),
		"strftime":        starlark.NewBuiltin(ModuleName+".strftime", notImplemented),
		"strptime":        starlark.NewBuiltin(ModuleName+".strptime", notImplemented),
		"tzset":           starlark.NewBuiltin(ModuleName+".tzset", notImplemented),
		"get_clock_info":  starlark.NewBuiltin(ModuleName+".get_clock_info", notImplemented),

		"timezone": starlark.MakeInt(0),
		"altzone":  starlark.MakeInt(0),
		"daylight": starlark.MakeInt(0),
		"tzname":   starlark.Tuple{starlark.String("UTC"), starlark.String("UTC")},
	}

	module := &starlarkstruct.Module{Name: ModuleName, Members: members}
	module.Freeze()
	return starlark.StringDict{ModuleName: module}
})

// LoadModule returns Dyson's Python-compatible time module.
func LoadModule() (starlark.StringDict, error) {
	return module(), nil
}

func notImplemented(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return nil, fmt.Errorf("%s: not implemented", fn.Name())
}

func unsupportedCPUClock(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return nil, fmt.Errorf("%s: CPU time clocks are not supported", fn.Name())
}
