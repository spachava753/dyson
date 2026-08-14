package time

import (
	"fmt"
	"strings"
	gotime "time"

	"github.com/spachava753/starlarkx/starlark"
)

// strftimeBuiltin implements time.strftime for the common format directives
// supported by Dyson's deterministic UTC time module.
//
// It mirrors the supported subset of Python's time.strftime:
// https://docs.python.org/3/library/time.html#time.strftime
func (m moduleTime) strftimeBuiltin(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var format string
	var value starlark.Value = starlark.None
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "format", &format, "t?", &value); err != nil {
		return nil, err
	}
	var st structTime
	var err error
	if value == starlark.None {
		now, nowErr := m.now(fn.Name())
		if nowErr != nil {
			return nil, nowErr
		}
		st = structFromTime(now.UTC())
	} else {
		st, err = structTimeFromValue(fn.Name(), value)
		if err != nil {
			return nil, err
		}
	}
	return starlark.String(formatStructTime(format, st)), nil
}

// strptimeBuiltin implements time.strptime for the common format directives
// supported by goLayout, returning a struct_time value.
//
// It mirrors the supported subset of Python's time.strptime:
// https://docs.python.org/3/library/time.html#time.strptime
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

// weekdays contains Python's abbreviated weekday names in tm_wday order.
var weekdays = []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

// months contains Python's abbreviated month names in calendar order.
var months = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// asctime formats struct_time fields using Python's asctime layout.
func asctime(st structTime) string {
	v := st.values
	return fmt.Sprintf("%s %s %2d %02d:%02d:%02d %04d", weekdays[v[6]], months[v[1]-1], v[2], v[3], v[4], v[5], v[0])
}

// formatStructTime applies Dyson's supported strftime directives to a
// struct_time value.
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

// goLayout maps Dyson's supported Python strptime directives to a Go time
// layout string, returning an empty layout when an unsupported directive remains.
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
