package time

import (
	"sync"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ModuleName is the Starlark stdlib module name for Dyson's time compatibility module.
const ModuleName = "time"

// module lazily builds the frozen time module namespace exposed by load("time.star", "time").
var module = sync.OnceValue(func() starlark.StringDict {
	members := starlark.StringDict{
		"time":            starlark.NewBuiltin(ModuleName+".time", timeBuiltin),
		"time_ns":         starlark.NewBuiltin(ModuleName+".time_ns", timeNSBuiltin),
		"monotonic":       starlark.NewBuiltin(ModuleName+".monotonic", monotonicBuiltin),
		"monotonic_ns":    starlark.NewBuiltin(ModuleName+".monotonic_ns", monotonicNSBuiltin),
		"perf_counter":    starlark.NewBuiltin(ModuleName+".perf_counter", monotonicBuiltin),
		"perf_counter_ns": starlark.NewBuiltin(ModuleName+".perf_counter_ns", monotonicNSBuiltin),
		"process_time":    starlark.NewBuiltin(ModuleName+".process_time", unsupportedCPUClock),
		"process_time_ns": starlark.NewBuiltin(ModuleName+".process_time_ns", unsupportedCPUClock),
		"thread_time":     starlark.NewBuiltin(ModuleName+".thread_time", unsupportedCPUClock),
		"thread_time_ns":  starlark.NewBuiltin(ModuleName+".thread_time_ns", unsupportedCPUClock),
		"sleep":           starlark.NewBuiltin(ModuleName+".sleep", sleepBuiltin),
		"gmtime":          starlark.NewBuiltin(ModuleName+".gmtime", gmtimeBuiltin),
		"localtime":       starlark.NewBuiltin(ModuleName+".localtime", localtimeBuiltin),
		"mktime":          starlark.NewBuiltin(ModuleName+".mktime", mktimeBuiltin),
		"asctime":         starlark.NewBuiltin(ModuleName+".asctime", asctimeBuiltin),
		"ctime":           starlark.NewBuiltin(ModuleName+".ctime", ctimeBuiltin),
		"strftime":        starlark.NewBuiltin(ModuleName+".strftime", strftimeBuiltin),
		"strptime":        starlark.NewBuiltin(ModuleName+".strptime", strptimeBuiltin),
		"tzset":           starlark.NewBuiltin(ModuleName+".tzset", unsupportedTZSet),
		"get_clock_info":  starlark.NewBuiltin(ModuleName+".get_clock_info", getClockInfoBuiltin),
		"struct_time":     starlark.NewBuiltin(ModuleName+".struct_time", structTimeBuiltin),

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
