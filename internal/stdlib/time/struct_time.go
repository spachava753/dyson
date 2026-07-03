package time

import (
	"encoding/binary"
	"fmt"
	gotime "time"

	"github.com/spachava753/dyson/internal/codec"
	"go.starlark.net/starlark"
)

// structTime stores Python's nine struct_time fields in tuple order.
type structTime struct {
	values [9]int
}

// structTimeValue is the Starlark-visible tuple-like time.struct_time value.
type structTimeValue struct {
	structTime
}

const (
	structTimeTypeName   = "struct_time"
	structTimeCodecBytes = 9 * 8
)

// RegisterStructTimeCodec installs durable serialization support for
// time.struct_time values into a codec registry.
func RegisterStructTimeCodec(registry codec.Registry) {
	registry[structTimeTypeName] = codec.ValueCodec{
		Type:    structTimeTypeName,
		Version: 1,
		Serialize: func(val starlark.Value) (codec.SerializedVal, error) {
			v, ok := val.(*structTimeValue)
			if !ok {
				return codec.SerializedVal{}, fmt.Errorf("dyson: got %T for struct_time codec", val)
			}
			data := make([]byte, structTimeCodecBytes)
			for i, field := range v.values {
				binary.BigEndian.PutUint64(data[i*8:], uint64(int64(field)))
			}
			hash, _ := v.Hash()
			return codec.SerializedVal{Type: structTimeTypeName, Hash: hash, Data: data}, nil
		},
		Restore: func(val codec.SerializedVal) (starlark.Value, error) {
			if len(val.Data) != structTimeCodecBytes {
				return nil, fmt.Errorf("dyson: invalid struct_time payload length %d", len(val.Data))
			}
			var values [9]int
			for i := range values {
				field := int64(binary.BigEndian.Uint64(val.Data[i*8:]))
				if int64(int(field)) != field {
					return nil, fmt.Errorf("dyson: struct_time field %d is out of range", i)
				}
				values[i] = int(field)
			}
			return &structTimeValue{structTime: structTime{values: values}}, nil
		},
	}
}

// newStructTime converts a Go time to the Starlark-visible struct_time value.
func newStructTime(t gotime.Time) starlark.Value {
	return &structTimeValue{structTime: structFromTime(t)}
}

// structFromTime converts a UTC Go time to Python's struct_time field order.
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

// structTimeBuiltin implements time.struct_time, constructing a tuple-like
// struct_time value from a sequence with at least nine integer fields.
//
// It mirrors the supported subset of Python's time.struct_time type:
// https://docs.python.org/3/library/time.html#time.struct_time
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

// String returns a Python-like representation of the struct_time value.
func (v *structTimeValue) String() string { return fmt.Sprintf("time.struct_time%v", v.Tuple()) }

// Type reports the Starlark-visible type name for struct_time values.
func (v *structTimeValue) Type() string { return structTimeTypeName }

// Freeze marks struct_time immutable for Starlark's shared-value semantics.
func (v *structTimeValue) Freeze() {}

// Truth reports that struct_time values are always truthy.
func (v *structTimeValue) Truth() starlark.Bool {
	return starlark.True
}

// Hash returns the tuple hash for the struct_time fields.
func (v *structTimeValue) Hash() (uint32, error) { return v.Tuple().Hash() }

// Len returns the fixed Python struct_time tuple length.
func (v *structTimeValue) Len() int { return 9 }

// Index returns the struct_time field at Python's tuple position i.
func (v *structTimeValue) Index(i int) starlark.Value {
	return starlark.MakeInt(v.values[i])
}

// Attr exposes Python-compatible struct_time field names such as tm_year and
// tm_wday.
func (v *structTimeValue) Attr(name string) (starlark.Value, error) {
	for i, attr := range structTimeAttrs {
		if name == attr {
			return starlark.MakeInt(v.values[i]), nil
		}
	}
	return nil, nil
}

// AttrNames returns the names discoverable on struct_time values.
func (v *structTimeValue) AttrNames() []string { return structTimeAttrs }

// Iterate returns an iterator over the nine struct_time tuple fields.
func (v *structTimeValue) Iterate() starlark.Iterator {
	return v.Tuple().Iterate()
}

// Tuple returns the struct_time fields as a Starlark tuple in Python order.
func (v *structTimeValue) Tuple() starlark.Tuple {
	out := make(starlark.Tuple, 9)
	for i, value := range v.values {
		out[i] = starlark.MakeInt(value)
	}
	return out
}

// structTimeAttrs lists Python-compatible field names exposed by struct_time.
var structTimeAttrs = []string{"tm_year", "tm_mon", "tm_mday", "tm_hour", "tm_min", "tm_sec", "tm_wday", "tm_yday", "tm_isdst"}

// optionalStructTime decodes optional struct_time-like parameters used by
// asctime and strftime, defaulting to current UTC localtime.
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

// structTimeFromValue validates and converts a Starlark struct_time or
// indexable nine-item integer sequence to internal structTime fields.
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

// timeFromStruct converts internal struct_time fields to a UTC Go time.
func timeFromStruct(st structTime) gotime.Time {
	v := st.values
	return gotime.Date(v[0], gotime.Month(v[1]), v[2], v[3], v[4], v[5], 0, gotime.UTC)
}
