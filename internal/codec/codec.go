package codec

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/big"

	"go.starlark.net/starlark"
)

// ValueCodec converts one Starlark value type to and from a versioned
// SerializedVal. Built-in values and custom values use the same registry path.
type ValueCodec struct {
	// Type is the exact Starlark type string returned by starlark.Value.Type.
	Type string

	// Version identifies the SerializedVal payload format for this type.
	Version int

	// Serialize converts a live Starlark value into durable form. Implementations
	// may leave Type and Version empty; Registry.Serialize fills them from the
	// registered codec.
	Serialize func(starlark.Value) (SerializedVal, error)

	// Restore reconstructs a live Starlark value from durable form after the
	// registry has already verified Type and Version.
	Restore func(SerializedVal) (starlark.Value, error)
}

// Registry is the per-session table of supported durable value types. A value
// cannot cross a recorded host boundary unless its Starlark type has a
// registered codec in the session registry.
type Registry map[string]ValueCodec

// SerializedVal is Dyson's durable representation of a Starlark value crossing
// a recorded host-event boundary.
//
// Data is a codec-owned byte payload for scalar/custom values. List, DictKeys,
// and DictValues keep containers structural for now so recursive validation is
// explicit. They represent a tree, not a mutable-container graph: aliases,
// cycles, and self-references are not preserved by the current format.
type SerializedVal struct {
	Type       string
	Version    int
	Hash       uint32
	List       []SerializedVal
	DictKeys   []SerializedVal
	DictValues []SerializedVal
	Data       []byte
}

// Register installs or replaces a codec in the registry. The registry map must
// already be initialized, as with any direct Go map assignment.
func (r Registry) Register(codec ValueCodec) {
	r[codec.Type] = codec
}

// Serialize converts a Starlark value into a durable record payload using the
// registered codec for that value type. It fails closed for unsupported values
// because a hash-only entry is not enough to replay a host event later.
//
// Current container codecs are tree-shaped only. They recursively serialize
// list, tuple, and dict contents but do not preserve mutable-container graph
// semantics: aliases are duplicated, cycles are unsupported, and self-referential
// containers may recurse until the Go stack overflows. Container graph support
// will require object IDs/references in SerializedVal.
func (r Registry) Serialize(val starlark.Value) (SerializedVal, error) {
	if val == nil {
		return SerializedVal{}, nil
	}
	codec, ok := r.codecForValue(val)
	if !ok {
		return SerializedVal{}, fmt.Errorf("dyson: cannot serialize starlark value of type %s", val.Type())
	}

	sv, err := codec.Serialize(val)
	if err != nil {
		return SerializedVal{}, err
	}
	if sv.Type == "" {
		sv.Type = codec.Type
	}
	if sv.Version == 0 {
		sv.Version = codec.Version
	}
	return sv, nil
}

// Restore reconstructs a supported value from SerializedVal. It only consults
// this registry, so persisted values cannot be restored unless their type has an
// explicit codec with a matching version.
func (r Registry) Restore(val SerializedVal) (starlark.Value, error) {
	codec, ok := r[val.Type]
	if !ok {
		return nil, fmt.Errorf("dyson: cannot restore starlark value of type %s", val.Type)
	}
	if val.Version != codec.Version {
		return nil, fmt.Errorf("dyson: cannot restore %s version %d with codec version %d", val.Type, val.Version, codec.Version)
	}
	return codec.Restore(val)
}

// DefaultRegistry returns a fresh registry containing Dyson's built-in codecs.
// Callers can add custom codecs to the returned map before passing it to a
// Sphere.
//
// The scalar encodings favor portability over compactness: ints are decimal
// bytes because Starlark integers are arbitrary precision, and floats are
// fixed-width IEEE 754 binary64 bytes in big-endian order. Container codecs are
// intentionally only a starting point: they serialize nested list/tuple/dict
// values as trees and do not yet detect or preserve aliases, cycles, or
// self-references.
func DefaultRegistry() Registry {
	var registry Registry
	registry = Registry{
		starlark.None.Type(): {
			Type:    starlark.None.Type(),
			Version: 1,
			Serialize: func(val starlark.Value) (SerializedVal, error) {
				return scalarSerializedVal(val, []byte{}), nil
			},
			Restore: func(val SerializedVal) (starlark.Value, error) {
				if len(val.Data) != 0 {
					return nil, fmt.Errorf("dyson: invalid None payload length %d", len(val.Data))
				}
				return starlark.None, nil
			},
		},
		starlark.Bool(false).Type(): {
			Type:    starlark.Bool(false).Type(),
			Version: 1,
			Serialize: func(val starlark.Value) (SerializedVal, error) {
				v, ok := val.(starlark.Bool)
				if !ok {
					return SerializedVal{}, fmt.Errorf("dyson: got %T for bool codec", val)
				}
				if v {
					return scalarSerializedVal(val, []byte{1}), nil
				}
				return scalarSerializedVal(val, []byte{0}), nil
			},
			Restore: func(val SerializedVal) (starlark.Value, error) {
				if len(val.Data) != 1 || val.Data[0] > 1 {
					return nil, fmt.Errorf("dyson: invalid bool payload %v", val.Data)
				}
				return starlark.Bool(val.Data[0] == 1), nil
			},
		},
		starlark.MakeInt(0).Type(): {
			Type:    starlark.MakeInt(0).Type(),
			Version: 1,
			Serialize: func(val starlark.Value) (SerializedVal, error) {
				v, ok := val.(starlark.Int)
				if !ok {
					return SerializedVal{}, fmt.Errorf("dyson: got %T for int codec", val)
				}
				return scalarSerializedVal(val, []byte(v.String())), nil
			},
			Restore: func(val SerializedVal) (starlark.Value, error) {
				v := new(big.Int)
				if _, ok := v.SetString(string(val.Data), 10); !ok {
					return nil, fmt.Errorf("dyson: invalid int payload %q", string(val.Data))
				}
				return starlark.MakeBigInt(v), nil
			},
		},
		starlark.Float(0).Type(): {
			Type:    starlark.Float(0).Type(),
			Version: 1,
			Serialize: func(val starlark.Value) (SerializedVal, error) {
				v, ok := val.(starlark.Float)
				if !ok {
					return SerializedVal{}, fmt.Errorf("dyson: got %T for float codec", val)
				}
				data := make([]byte, 8)
				binary.BigEndian.PutUint64(data, math.Float64bits(float64(v)))
				return scalarSerializedVal(val, data), nil
			},
			Restore: func(val SerializedVal) (starlark.Value, error) {
				if len(val.Data) != 8 {
					return nil, fmt.Errorf("dyson: invalid float payload length %d", len(val.Data))
				}
				return starlark.Float(math.Float64frombits(binary.BigEndian.Uint64(val.Data))), nil
			},
		},
		starlark.String("").Type(): {
			Type:    starlark.String("").Type(),
			Version: 1,
			Serialize: func(val starlark.Value) (SerializedVal, error) {
				v, ok := val.(starlark.String)
				if !ok {
					return SerializedVal{}, fmt.Errorf("dyson: got %T for string codec", val)
				}
				return scalarSerializedVal(val, []byte(string(v))), nil
			},
			Restore: func(val SerializedVal) (starlark.Value, error) {
				return starlark.String(string(val.Data)), nil
			},
		},
		starlark.Bytes("").Type(): {
			Type:    starlark.Bytes("").Type(),
			Version: 1,
			Serialize: func(val starlark.Value) (SerializedVal, error) {
				v, ok := val.(starlark.Bytes)
				if !ok {
					return SerializedVal{}, fmt.Errorf("dyson: got %T for bytes codec", val)
				}
				return scalarSerializedVal(val, []byte(string(v))), nil
			},
			Restore: func(val SerializedVal) (starlark.Value, error) {
				return starlark.Bytes(string(val.Data)), nil
			},
		},
		(starlark.Tuple{}).Type(): {
			Type:    (starlark.Tuple{}).Type(),
			Version: 1,
			Serialize: func(val starlark.Value) (SerializedVal, error) {
				return registry.serializeIndexable(val)
			},
			Restore: func(val SerializedVal) (starlark.Value, error) {
				items, err := registry.restoreList(val.List)
				if err != nil {
					return nil, err
				}
				return starlark.Tuple(items), nil
			},
		},
		starlark.NewList(nil).Type(): {
			Type:    starlark.NewList(nil).Type(),
			Version: 1,
			Serialize: func(val starlark.Value) (SerializedVal, error) {
				return registry.serializeIndexable(val)
			},
			Restore: func(val SerializedVal) (starlark.Value, error) {
				items, err := registry.restoreList(val.List)
				if err != nil {
					return nil, err
				}
				return starlark.NewList(items), nil
			},
		},
		starlark.NewSet(0).Type(): {
			Type:    starlark.NewSet(0).Type(),
			Version: 1,
			Serialize: func(val starlark.Value) (SerializedVal, error) {
				return registry.serializeIterable(val)
			},
			Restore: func(val SerializedVal) (starlark.Value, error) {
				items, err := registry.restoreList(val.List)
				if err != nil {
					return nil, err
				}
				set := starlark.NewSet(len(items))
				for _, item := range items {
					if err := set.Insert(item); err != nil {
						return nil, err
					}
				}
				return set, nil
			},
		},
		starlark.NewDict(0).Type(): {
			Type:    starlark.NewDict(0).Type(),
			Version: 1,
			Serialize: func(val starlark.Value) (SerializedVal, error) {
				return registry.serializeDict(val)
			},
			Restore: func(val SerializedVal) (starlark.Value, error) {
				return registry.restoreDict(val)
			},
		},
	}
	return registry
}

func (r Registry) codecForValue(val starlark.Value) (ValueCodec, bool) {
	codec, ok := r[val.Type()]
	return codec, ok
}

func (r Registry) serializeIndexable(val starlark.Value) (SerializedVal, error) {
	v, ok := val.(starlark.Indexable)
	if !ok {
		return SerializedVal{}, fmt.Errorf("dyson: got %T for indexable codec", val)
	}
	sv := SerializedVal{
		Type: v.Type(),
		List: make([]SerializedVal, v.Len()),
	}
	for i := range v.Len() {
		item, err := r.Serialize(v.Index(i))
		if err != nil {
			return SerializedVal{}, err
		}
		sv.List[i] = item
	}
	return sv, nil
}

func (r Registry) serializeIterable(val starlark.Value) (SerializedVal, error) {
	v, ok := val.(starlark.Iterable)
	if !ok {
		return SerializedVal{}, fmt.Errorf("dyson: got %T for iterable codec", val)
	}
	var items []SerializedVal
	iter := v.Iterate()
	defer iter.Done()
	var item starlark.Value
	for iter.Next(&item) {
		serialized, err := r.Serialize(item)
		if err != nil {
			return SerializedVal{}, err
		}
		items = append(items, serialized)
	}
	return SerializedVal{Type: v.Type(), List: items}, nil
}

func (r Registry) restoreList(vals []SerializedVal) ([]starlark.Value, error) {
	items := make([]starlark.Value, len(vals))
	for i, val := range vals {
		item, err := r.Restore(val)
		if err != nil {
			return nil, err
		}
		items[i] = item
	}
	return items, nil
}

func (r Registry) serializeDict(val starlark.Value) (SerializedVal, error) {
	v, ok := val.(starlark.IterableMapping)
	if !ok {
		return SerializedVal{}, fmt.Errorf("dyson: got %T for dict codec", val)
	}
	items := v.Items()
	sv := SerializedVal{
		Type:       v.Type(),
		DictKeys:   make([]SerializedVal, len(items)),
		DictValues: make([]SerializedVal, len(items)),
	}
	for i := range items {
		key, err := r.Serialize(items[i][0])
		if err != nil {
			return SerializedVal{}, err
		}
		value, err := r.Serialize(items[i][1])
		if err != nil {
			return SerializedVal{}, err
		}
		sv.DictKeys[i] = key
		sv.DictValues[i] = value
	}
	return sv, nil
}

func (r Registry) restoreDict(val SerializedVal) (starlark.Value, error) {
	if len(val.DictKeys) != len(val.DictValues) {
		return nil, fmt.Errorf("dyson: dict has %d keys and %d values", len(val.DictKeys), len(val.DictValues))
	}
	dict := starlark.NewDict(len(val.DictKeys))
	for i := range val.DictKeys {
		key, err := r.Restore(val.DictKeys[i])
		if err != nil {
			return nil, err
		}
		value, err := r.Restore(val.DictValues[i])
		if err != nil {
			return nil, err
		}
		if err := dict.SetKey(key, value); err != nil {
			return nil, err
		}
	}
	return dict, nil
}

func scalarSerializedVal(val starlark.Value, data []byte) SerializedVal {
	sv := serializeHashable(val)
	sv.Data = data
	return sv
}

func serializeHashable(val starlark.Value) SerializedVal {
	sv := SerializedVal{Type: val.Type()}
	if hash, err := val.Hash(); err == nil {
		sv.Hash = hash
	}
	return sv
}
