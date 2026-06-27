package time

import (
	"fmt"
	"math"
	gotime "time"

	"go.starlark.net/starlark"
)

// gmtimeBuiltin implements time.gmtime, converting Unix seconds to a UTC
// struct_time value and defaulting to the current wall clock when omitted.
//
// It mirrors the supported subset of Python's time.gmtime:
// https://docs.python.org/3/library/time.html#time.gmtime
func gmtimeBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	sec, err := optionalSeconds(fn.Name(), args, kwargs)
	if err != nil {
		return nil, err
	}
	return newStructTime(unixFloat(sec).UTC()), nil
}

// localtimeBuiltin implements time.localtime using Dyson's deterministic UTC
// local-time policy.
//
// It mirrors the supported subset of Python's time.localtime:
// https://docs.python.org/3/library/time.html#time.localtime
func localtimeBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	sec, err := optionalSeconds(fn.Name(), args, kwargs)
	if err != nil {
		return nil, err
	}
	return newStructTime(unixFloat(sec).UTC()), nil
}

// mktimeBuiltin implements time.mktime, converting a local struct_time-like
// sequence to Unix timestamp seconds under Dyson's UTC policy.
//
// It mirrors the supported subset of Python's time.mktime:
// https://docs.python.org/3/library/time.html#time.mktime
func mktimeBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var value starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "t", &value); err != nil {
		return nil, err
	}
	st, err := structTimeFromValue(fn.Name(), value)
	if err != nil {
		return nil, err
	}
	return starlark.Float(float64(timeFromStruct(st).Unix())), nil
}

// asctimeBuiltin implements time.asctime, formatting a struct_time-like value
// with CPython's fixed-width weekday/month representation.
//
// It mirrors the supported subset of Python's time.asctime:
// https://docs.python.org/3/library/time.html#time.asctime
func asctimeBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	st, err := optionalStructTime(fn.Name(), args, kwargs)
	if err != nil {
		return nil, err
	}
	return starlark.String(asctime(st)), nil
}

// ctimeBuiltin implements time.ctime, formatting seconds as
// asctime(localtime(seconds)).
//
// It mirrors the supported subset of Python's time.ctime:
// https://docs.python.org/3/library/time.html#time.ctime
func ctimeBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	sec, err := optionalSeconds(fn.Name(), args, kwargs)
	if err != nil {
		return nil, err
	}
	return starlark.String(asctime(structFromTime(unixFloat(sec).UTC()))), nil
}

// unsupportedTZSet implements time.tzset as an explicit unsupported operation
// because Dyson's time module uses a deterministic UTC timezone policy.
func unsupportedTZSet(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return nil, fmt.Errorf("%s: timezone environment changes are not supported", fn.Name())
}

// optionalSeconds decodes optional seconds parameters used by gmtime,
// localtime, and ctime, defaulting to the current wall clock.
func optionalSeconds(name string, args starlark.Tuple, kwargs []starlark.Tuple) (float64, error) {
	var value starlark.Value = starlark.None
	if err := starlark.UnpackArgs(name, args, kwargs, "seconds?", &value); err != nil {
		return 0, err
	}
	if value == starlark.None {
		return float64(gotime.Now().UnixNano()) / 1e9, nil
	}
	return number(name, "seconds", value)
}

// secondsDuration converts a Starlark seconds value to a non-negative Go
// duration for sleep.
func secondsDuration(name string, value starlark.Value) (gotime.Duration, error) {
	sec, err := number(name, "seconds", value)
	if err != nil {
		return 0, err
	}
	if sec < 0 {
		return 0, fmt.Errorf("%s: sleep length must be non-negative", name)
	}
	return gotime.Duration(sec * float64(gotime.Second)), nil
}

// number converts a Starlark int or float parameter to a finite float64 and
// formats validation errors with the calling function and argument name.
func number(name, arg string, value starlark.Value) (float64, error) {
	switch v := value.(type) {
	case starlark.Int:
		i, ok := v.Int64()
		if !ok {
			return 0, fmt.Errorf("%s: %s is out of range", name, arg)
		}
		return float64(i), nil
	case starlark.Float:
		f := float64(v)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, fmt.Errorf("%s: %s must be finite", name, arg)
		}
		return f, nil
	default:
		return 0, fmt.Errorf("%s: %s must be int or float, got %s", name, arg, value.Type())
	}
}

// unixFloat converts Unix timestamp seconds, including fractional seconds, to a
// UTC Go time value.
func unixFloat(seconds float64) gotime.Time {
	whole, frac := math.Modf(seconds)
	return gotime.Unix(int64(whole), int64(frac*1e9)).UTC()
}
