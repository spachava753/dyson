package time

import (
	"fmt"
	"math"
	"strings"
	"sync"
	gotime "time"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

const ModuleName = "time"

var moduleStart = gotime.Now()

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

func timeBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	return starlark.Float(float64(gotime.Now().UnixNano()) / 1e9), nil
}

func timeNSBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	return starlark.MakeInt64(gotime.Now().UnixNano()), nil
}

func monotonicBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	return starlark.Float(float64(gotime.Since(monotonicStart(thread)).Nanoseconds()) / 1e9), nil
}

func monotonicNSBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	return starlark.MakeInt64(gotime.Since(monotonicStart(thread)).Nanoseconds()), nil
}

const monotonicStartKey = "github.com/spachava753/dyson/internal/stdlib/time.monotonicStart"

func monotonicStart(thread *starlark.Thread) gotime.Time {
	if thread != nil {
		if start, ok := thread.Local(monotonicStartKey).(gotime.Time); ok {
			return start
		}
		start := gotime.Now()
		thread.SetLocal(monotonicStartKey, start)
		return start
	}
	return moduleStart
}

func sleepBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var seconds starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "seconds", &seconds); err != nil {
		return nil, err
	}
	d, err := secondsDuration(fn.Name(), seconds)
	if err != nil {
		return nil, err
	}
	gotime.Sleep(d)
	return starlark.None, nil
}

func gmtimeBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	sec, err := optionalSeconds(fn.Name(), args, kwargs)
	if err != nil {
		return nil, err
	}
	return newStructTime(unixFloat(sec).UTC()), nil
}

func localtimeBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	sec, err := optionalSeconds(fn.Name(), args, kwargs)
	if err != nil {
		return nil, err
	}
	return newStructTime(unixFloat(sec).UTC()), nil
}

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

func asctimeBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	st, err := optionalStructTime(fn.Name(), args, kwargs)
	if err != nil {
		return nil, err
	}
	return starlark.String(asctime(st)), nil
}

func ctimeBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	sec, err := optionalSeconds(fn.Name(), args, kwargs)
	if err != nil {
		return nil, err
	}
	return starlark.String(asctime(structFromTime(unixFloat(sec).UTC()))), nil
}

func strftimeBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var format string
	var value starlark.Value = starlark.None
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "format", &format, "t?", &value); err != nil {
		return nil, err
	}
	var st structTime
	var err error
	if value == starlark.None {
		st = structFromTime(gotime.Now().UTC())
	} else {
		st, err = structTimeFromValue(fn.Name(), value)
		if err != nil {
			return nil, err
		}
	}
	return starlark.String(formatStructTime(format, st)), nil
}

func strptimeBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var value, format string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "string", &value, "format", &format); err != nil {
		return nil, err
	}
	layout := goLayout(format)
	if layout == "" {
		return nil, fmt.Errorf("%s: unsupported format directive", fn.Name())
	}
	tm, err := gotime.ParseInLocation(layout, value, gotime.UTC)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", fn.Name(), err)
	}
	return newStructTime(tm.UTC()), nil
}

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

func structTimeBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var value starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "sequence", &value); err != nil {
		return nil, err
	}
	st, err := structTimeFromValue(fn.Name(), value)
	if err != nil {
		return nil, err
	}
	return &structTimeValue{structTime: st}, nil
}

func unsupportedCPUClock(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return nil, fmt.Errorf("%s: CPU time clocks are not supported", fn.Name())
}

func unsupportedTZSet(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return nil, fmt.Errorf("%s: timezone environment changes are not supported", fn.Name())
}

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

func unixFloat(seconds float64) gotime.Time {
	whole, frac := math.Modf(seconds)
	return gotime.Unix(int64(whole), int64(frac*1e9)).UTC()
}

type structTime struct {
	values [9]int
}

type structTimeValue struct {
	structTime
}

func newStructTime(t gotime.Time) starlark.Value {
	return &structTimeValue{structTime: structFromTime(t)}
}

func structFromTime(t gotime.Time) structTime {
	_, offset := t.Zone()
	isdst := 0
	if offset != 0 {
		isdst = 1
	}
	return structTime{values: [9]int{
		t.Year(), int(t.Month()), t.Day(), t.Hour(), t.Minute(), t.Second(),
		(int(t.Weekday()) + 6) % 7,
		t.YearDay(), isdst,
	}}
}

func (v *structTimeValue) String() string { return fmt.Sprintf("time.struct_time%v", v.Tuple()) }
func (v *structTimeValue) Type() string   { return "struct_time" }
func (v *structTimeValue) Freeze()        {}
func (v *structTimeValue) Truth() starlark.Bool {
	return starlark.True
}
func (v *structTimeValue) Hash() (uint32, error) { return v.Tuple().Hash() }
func (v *structTimeValue) Len() int              { return 9 }
func (v *structTimeValue) Index(i int) starlark.Value {
	return starlark.MakeInt(v.values[i])
}
func (v *structTimeValue) Attr(name string) (starlark.Value, error) {
	for i, attr := range structTimeAttrs {
		if name == attr {
			return starlark.MakeInt(v.values[i]), nil
		}
	}
	return nil, nil
}
func (v *structTimeValue) AttrNames() []string { return structTimeAttrs }
func (v *structTimeValue) Iterate() starlark.Iterator {
	return v.Tuple().Iterate()
}
func (v *structTimeValue) Tuple() starlark.Tuple {
	out := make(starlark.Tuple, 9)
	for i, value := range v.values {
		out[i] = starlark.MakeInt(value)
	}
	return out
}

var structTimeAttrs = []string{"tm_year", "tm_mon", "tm_mday", "tm_hour", "tm_min", "tm_sec", "tm_wday", "tm_yday", "tm_isdst"}

func optionalStructTime(name string, args starlark.Tuple, kwargs []starlark.Tuple) (structTime, error) {
	var value starlark.Value = starlark.None
	if err := starlark.UnpackArgs(name, args, kwargs, "t?", &value); err != nil {
		return structTime{}, err
	}
	if value == starlark.None {
		return structFromTime(gotime.Now().UTC()), nil
	}
	return structTimeFromValue(name, value)
}

func structTimeFromValue(name string, value starlark.Value) (structTime, error) {
	if st, ok := value.(*structTimeValue); ok {
		return st.structTime, nil
	}
	seq, ok := value.(starlark.Indexable)
	if !ok || seq.Len() < 9 {
		return structTime{}, fmt.Errorf("%s: time tuple must have at least 9 items", name)
	}
	var values [9]int
	for i := range values {
		item := seq.Index(i)
		iv, ok := item.(starlark.Int)
		if !ok {
			return structTime{}, fmt.Errorf("%s: time tuple item %d must be int", name, i)
		}
		value, ok := iv.Int64()
		if !ok {
			return structTime{}, fmt.Errorf("%s: time tuple item %d is out of range", name, i)
		}
		values[i] = int(value)
	}
	return structTime{values: values}, nil
}

func timeFromStruct(st structTime) gotime.Time {
	v := st.values
	return gotime.Date(v[0], gotime.Month(v[1]), v[2], v[3], v[4], v[5], 0, gotime.UTC)
}

var weekdays = []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
var months = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

func asctime(st structTime) string {
	v := st.values
	return fmt.Sprintf("%s %s %2d %02d:%02d:%02d %04d", weekdays[v[6]], months[v[1]-1], v[2], v[3], v[4], v[5], v[0])
}

func formatStructTime(format string, st structTime) string {
	v := st.values
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] != '%' || i == len(format)-1 {
			b.WriteByte(format[i])
			continue
		}
		i++
		switch format[i] {
		case '%':
			b.WriteByte('%')
		case 'Y':
			fmt.Fprintf(&b, "%04d", v[0])
		case 'm':
			fmt.Fprintf(&b, "%02d", v[1])
		case 'd':
			fmt.Fprintf(&b, "%02d", v[2])
		case 'H':
			fmt.Fprintf(&b, "%02d", v[3])
		case 'M':
			fmt.Fprintf(&b, "%02d", v[4])
		case 'S':
			fmt.Fprintf(&b, "%02d", v[5])
		case 'a':
			b.WriteString(weekdays[v[6]])
		case 'b':
			b.WriteString(months[v[1]-1])
		case 'j':
			fmt.Fprintf(&b, "%03d", v[7])
		case 'w':
			fmt.Fprintf(&b, "%d", (v[6]+1)%7)
		default:
			b.WriteByte('%')
			b.WriteByte(format[i])
		}
	}
	return b.String()
}

func goLayout(format string) string {
	replacer := strings.NewReplacer(
		"%%", "%",
		"%Y", "2006",
		"%m", "01",
		"%d", "02",
		"%H", "15",
		"%M", "04",
		"%S", "05",
		"%a", "Mon",
		"%b", "Jan",
	)
	layout := replacer.Replace(format)
	if strings.Contains(layout, "%") {
		return ""
	}
	return layout
}
