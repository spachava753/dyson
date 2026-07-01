package dyson

import (
	"fmt"

	"go.starlark.net/starlark"
)

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

// Serialize converts a Starlark value into a durable record payload using the
// registered codec for that value type. It fails closed for unsupported values
// because a hash-only entry is not enough to replay a host event later.
//
// Current container codecs are tree-shaped only. They recursively serialize
// list, tuple, and dict contents but do not preserve mutable-container graph
// semantics: aliases are duplicated, cycles are unsupported, and self-referential
// containers may recurse until the Go stack overflows. Container graph support
// will require object IDs/references in SerializedVal.
func (r CodecRegistry) Serialize(val starlark.Value) (SerializedVal, error) {
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

// serializeHashable preserves a Starlark hash as optional metadata. The hash is
// useful for equality checks, but it is never treated as the durable value.
func serializeHashable(val starlark.Value) SerializedVal {
	sv := SerializedVal{Type: val.Type()}
	if hash, err := val.Hash(); err == nil {
		sv.Hash = hash
	}
	return sv
}
