package snapshot

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"

	"go.starlark.net/starlark"
)

// Decoder reads snapshots of supported Starlark global values.
type Decoder struct {
	json *json.Decoder

	snapshot snapshot
	values   map[int]starlark.Value
}

// NewDecoder returns a new snapshot decoder that reads from r.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{json: json.NewDecoder(r)}
}

// Decode reads one JSON snapshot into globals.
func (d *Decoder) Decode(globals *starlark.StringDict) error {
	if globals == nil {
		return fmt.Errorf("decode globals: nil target")
	}

	var snap snapshot
	if err := d.json.Decode(&snap); err != nil {
		return err
	}

	d.snapshot = snap
	d.values = map[int]starlark.Value{}
	defer d.reset()

	restored := starlark.StringDict{}
	for name, encoded := range snap.Globals {
		value, err := d.decode(encoded)
		if err != nil {
			return fmt.Errorf("decode global %s: %w", name, err)
		}
		restored[name] = value
	}

	*globals = restored
	return nil
}

func (d *Decoder) reset() {
	d.snapshot = snapshot{}
	d.values = nil
}

func (d *Decoder) decode(value snapshotValue) (starlark.Value, error) {
	var decoded starlark.Value
	switch value.Kind {
	case valueTypeNone:
		decoded = starlark.None
	case valueTypeBool:
		decoded = starlark.Bool(value.Bool)
	case valueTypeInt:
		bigInt, ok := new(big.Int).SetString(value.Text, 10)
		if !ok {
			return nil, fmt.Errorf("invalid int %q", value.Text)
		}
		decoded = starlark.MakeBigInt(bigInt)
	case valueTypeFloat:
		decoded = starlark.Float(value.Float)
	case valueTypeString:
		decoded = starlark.String(value.Text)
	case valueTypeBytes:
		decoded = starlark.Bytes(value.Text)
	case valueTypeTuple:
		items := make([]starlark.Value, 0, len(value.Items))
		for _, item := range value.Items {
			decodedItem, err := d.decode(item)
			if err != nil {
				return nil, err
			}
			items = append(items, decodedItem)
		}
		decoded = starlark.Tuple(items)
	case valueTypeRef:
		var err error
		decoded, err = d.decodeObject(value.Ref)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown value kind %d", value.Kind)
	}

	if value.TypeName == "" {
		return decoded, nil
	}
	restore, ok := registry[value.TypeName]
	if !ok {
		return nil, fmt.Errorf("unregistered custom type %q", value.TypeName)
	}
	restored, err := restore(decoded)
	if err != nil {
		return nil, fmt.Errorf("restore custom type %s: %w", value.TypeName, err)
	}
	return restored, nil
}

func (d *Decoder) decodeObject(id int) (starlark.Value, error) {
	if value, ok := d.values[id]; ok {
		return value, nil
	}

	object, ok := d.snapshot.Objects[id]
	if !ok {
		return nil, fmt.Errorf("unknown object ref %d", id)
	}

	switch object.Kind {
	case objectKindList:
		list := starlark.NewList(nil)
		d.values[id] = list
		for _, item := range object.Items {
			decoded, err := d.decode(item)
			if err != nil {
				return nil, err
			}
			if err := list.Append(decoded); err != nil {
				return nil, err
			}
		}
		return list, nil
	case objectKindDict:
		dict := starlark.NewDict(len(object.Entries))
		d.values[id] = dict
		for _, entry := range object.Entries {
			key, err := d.decode(entry.Key)
			if err != nil {
				return nil, err
			}
			value, err := d.decode(entry.Value)
			if err != nil {
				return nil, err
			}
			if err := dict.SetKey(key, value); err != nil {
				return nil, err
			}
		}
		return dict, nil
	case objectKindSet:
		set := starlark.NewSet(len(object.Items))
		d.values[id] = set
		for _, item := range object.Items {
			decoded, err := d.decode(item)
			if err != nil {
				return nil, err
			}
			if err := set.Insert(decoded); err != nil {
				return nil, err
			}
		}
		return set, nil
	default:
		return nil, fmt.Errorf("unknown object kind %d", object.Kind)
	}
}
