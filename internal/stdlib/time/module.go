package time

import (
	gotime "time"

	"github.com/spachava753/dyson/internal/xos"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ModuleName is the Starlark stdlib module name for Dyson's time compatibility module.
const ModuleName = "time"

type moduleTime struct {
	clock xos.Clock
	start gotime.Time
}

// MakeModule returns a Starlark module namespace exposed by load("time.star", "time").
// A nil clock keeps clock reads and sleeping fail-closed.
func MakeModule(clock xos.Clock) *starlarkstruct.Module {
	mt := moduleTime{clock: clock}
	if clock != nil {
		mt.start = clock.Now()
	}
	m := &starlarkstruct.Module{
		Name: ModuleName,
		Members: starlark.StringDict{
			"time":            starlark.NewBuiltin(ModuleName+".time", mt.timeBuiltin),
			"time_ns":         starlark.NewBuiltin(ModuleName+".time_ns", mt.timeNSBuiltin),
			"monotonic":       starlark.NewBuiltin(ModuleName+".monotonic", mt.monotonicBuiltin),
			"monotonic_ns":    starlark.NewBuiltin(ModuleName+".monotonic_ns", mt.monotonicNSBuiltin),
			"perf_counter":    starlark.NewBuiltin(ModuleName+".perf_counter", mt.monotonicBuiltin),
			"perf_counter_ns": starlark.NewBuiltin(ModuleName+".perf_counter_ns", mt.monotonicNSBuiltin),
			"process_time":    starlark.NewBuiltin(ModuleName+".process_time", unsupportedCPUClock),
			"process_time_ns": starlark.NewBuiltin(ModuleName+".process_time_ns", unsupportedCPUClock),
			"thread_time":     starlark.NewBuiltin(ModuleName+".thread_time", unsupportedCPUClock),
			"thread_time_ns":  starlark.NewBuiltin(ModuleName+".thread_time_ns", unsupportedCPUClock),
			"sleep":           starlark.NewBuiltin(ModuleName+".sleep", mt.sleepBuiltin),
			"gmtime":          starlark.NewBuiltin(ModuleName+".gmtime", mt.gmtimeBuiltin),
			"localtime":       starlark.NewBuiltin(ModuleName+".localtime", mt.localtimeBuiltin),
			"mktime":          starlark.NewBuiltin(ModuleName+".mktime", mktimeBuiltin),
			"asctime":         starlark.NewBuiltin(ModuleName+".asctime", mt.asctimeBuiltin),
			"ctime":           starlark.NewBuiltin(ModuleName+".ctime", mt.ctimeBuiltin),
			"strftime":        starlark.NewBuiltin(ModuleName+".strftime", mt.strftimeBuiltin),
			"strptime":        starlark.NewBuiltin(ModuleName+".strptime", strptimeBuiltin),
			"tzset":           starlark.NewBuiltin(ModuleName+".tzset", unsupportedTZSet),
			"get_clock_info":  starlark.NewBuiltin(ModuleName+".get_clock_info", getClockInfoBuiltin),
			"struct_time":     starlark.NewBuiltin(ModuleName+".struct_time", structTimeBuiltin),

			"timezone": starlark.MakeInt(0),
			"altzone":  starlark.MakeInt(0),
			"daylight": starlark.MakeInt(0),
			"tzname":   starlark.Tuple{starlark.String("UTC"), starlark.String("UTC")},
		},
	}
	m.Freeze()
	return m
}
