package time

import (
	"context"
	"fmt"
	gotime "time"

	"github.com/spachava753/dyson/internal/xctx"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

func (m moduleTime) now(fn string) (gotime.Time, error) {
	if m.clock == nil {
		return gotime.Time{}, fmt.Errorf("%s: clock is not configured", fn)
	}
	return m.clock.Now(), nil
}

// timeBuiltin implements time.time, returning the current Unix timestamp as a
// floating-point number of seconds.
//
// It mirrors the supported subset of Python's time.time:
// https://docs.python.org/3/library/time.html#time.time
func (m moduleTime) timeBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	now, err := m.now(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.Float(float64(now.UnixNano()) / 1e9), nil
}

// timeNSBuiltin implements time.time_ns, returning the current Unix timestamp
// as an integer number of nanoseconds.
//
// It mirrors the supported subset of Python's time.time_ns:
// https://docs.python.org/3/library/time.html#time.time_ns
func (m moduleTime) timeNSBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	now, err := m.now(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.MakeInt64(now.UnixNano()), nil
}

// monotonicBuiltin implements time.monotonic and time.perf_counter, returning
// elapsed seconds from this module instance's monotonic origin.
//
// It mirrors the supported subset of Python's time.monotonic and
// time.perf_counter:
// https://docs.python.org/3/library/time.html#time.monotonic
// https://docs.python.org/3/library/time.html#time.perf_counter
func (m moduleTime) monotonicBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	now, err := m.now(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.Float(float64(now.Sub(m.start).Nanoseconds()) / 1e9), nil
}

// monotonicNSBuiltin implements time.monotonic_ns and time.perf_counter_ns,
// returning elapsed nanoseconds from this module instance's monotonic origin.
//
// It mirrors the supported subset of Python's nanosecond monotonic clocks:
// https://docs.python.org/3/library/time.html#time.monotonic_ns
// https://docs.python.org/3/library/time.html#time.perf_counter_ns
func (m moduleTime) monotonicNSBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	now, err := m.now(fn.Name())
	if err != nil {
		return nil, err
	}
	return starlark.MakeInt64(now.Sub(m.start).Nanoseconds()), nil
}

// sleepBuiltin implements time.sleep, blocking for the requested non-negative
// number of seconds and returning None.
//
// It mirrors the supported subset of Python's time.sleep:
// https://docs.python.org/3/library/time.html#time.sleep
func (m moduleTime) sleepBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var seconds starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "seconds", &seconds); err != nil {
		return nil, err
	}
	d, err := secondsDuration(fn.Name(), seconds)
	if err != nil {
		return nil, err
	}
	if m.clock == nil {
		return nil, fmt.Errorf("%s: clock is not configured", fn.Name())
	}
	ctx := xctx.FromLocal(thread)
	if ctx == nil {
		ctx = context.Background()
	}
	if err := m.clock.Sleep(ctx, d); err != nil {
		return nil, err
	}
	return starlark.None, nil
}

// getClockInfoBuiltin implements time.get_clock_info, returning a namespace-like
// value with implementation, monotonic, adjustable, and resolution attributes.
//
// It mirrors the supported subset of Python's time.get_clock_info:
// https://docs.python.org/3/library/time.html#time.get_clock_info
func getClockInfoBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "name", &name); err != nil {
		return nil, err
	}
	members := starlark.StringDict{
		"implementation": starlark.String("dyson time." + name),
		"resolution":     starlark.Float(1e-9),
	}
	switch name {
	case "time":
		members["monotonic"] = starlark.False
		members["adjustable"] = starlark.True
	case "monotonic", "perf_counter":
		members["monotonic"] = starlark.True
		members["adjustable"] = starlark.False
	default:
		return nil, fmt.Errorf("%s: unknown clock %q", fn.Name(), name)
	}
	return starlarkstruct.FromStringDict(starlark.String("namespace"), members), nil
}

// unsupportedCPUClock implements process_time, process_time_ns, thread_time,
// and thread_time_ns as explicit unsupported operations because Go has no
// portable standard-library CPU-time clock matching Python semantics.
func unsupportedCPUClock(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return nil, fmt.Errorf("%s: CPU time clocks are not supported", fn.Name())
}
