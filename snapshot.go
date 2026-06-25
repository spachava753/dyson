package dyson

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"

	"go.starlark.net/starlark"
)

// Encoder writes snapshots of supported Starlark global values.
type Encoder struct {
	json *json.Encoder

	nextID int

	// Mutable Starlark containers form a graph, not a tree: multiple globals may
	// point at the same list/dict, and containers may even contain themselves.
	// Object ids let refs preserve that aliasing and avoid recursive cycles.
	lists   map[*starlark.List]int
	dicts   map[*starlark.Dict]int
	objects map[int]object
}

// NewEncoder returns a new snapshot encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{json: json.NewEncoder(w)}
}

// Encode writes globals as a JSON snapshot.
func (e *Encoder) Encode(globals starlark.StringDict) error {
	e.reset()
	defer e.clear()

	snap := snapshot{
		Globals: map[string]snapshotValue{},
		Objects: e.objects,
	}

	for name, value := range globals {
		encoded, err := e.encode(value)
		if err != nil {
			return fmt.Errorf("encode global %s: %w", name, err)
		}
		snap.Globals[name] = encoded
	}

	return e.json.Encode(snap)
}

func (e *Encoder) reset() {
	e.nextID = 0
	e.lists = map[*starlark.List]int{}
	e.dicts = map[*starlark.Dict]int{}
	e.objects = map[int]object{}
}

func (e *Encoder) clear() {
	e.nextID = 0
	e.lists = nil
	e.dicts = nil
	e.objects = nil
}

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

type snapshot struct {
	Globals map[string]snapshotValue `json:"globals"`
	Objects map[int]object           `json:"objects,omitempty"`
}

type object struct {
	Kind    objectType      `json:"kind"`
	Items   []snapshotValue `json:"items,omitempty"`
	Entries []objectEntry   `json:"entries,omitempty"`
}

type objectType uint8

const (
	objectKindInvalid objectType = iota
	objectKindList
	objectKindDict
)

type objectEntry struct {
	Key   snapshotValue `json:"key"`
	Value snapshotValue `json:"value"`
}

type snapshotValue struct {
	Kind  valueType       `json:"kind"`
	Ref   int             `json:"ref,omitempty"`
	Bool  bool            `json:"bool,omitempty"`
	Float float64         `json:"float,omitempty"`
	Text  string          `json:"text,omitempty"`
	Items []snapshotValue `json:"items,omitempty"`
}

type valueType uint8

const (
	valueTypeInvalid valueType = iota
	valueTypeNone
	valueTypeBool
	valueTypeInt
	valueTypeFloat
	valueTypeString
	valueTypeTuple
	valueTypeRef
)

func (e *Encoder) encode(value starlark.Value) (snapshotValue, error) {
	switch value := value.(type) {
	case starlark.NoneType:
		return snapshotValue{Kind: valueTypeNone}, nil
	case starlark.Bool:
		return snapshotValue{Kind: valueTypeBool, Bool: bool(value)}, nil
	case starlark.Int:
		return snapshotValue{Kind: valueTypeInt, Text: value.String()}, nil
	case starlark.Float:
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return snapshotValue{}, fmt.Errorf("unsupported non-finite float %s", value)
		}
		return snapshotValue{Kind: valueTypeFloat, Float: float64(value)}, nil
	case starlark.String:
		return snapshotValue{Kind: valueTypeString, Text: string(value)}, nil
	case starlark.Tuple:
		items, err := e.encodeTuple(value)
		if err != nil {
			return snapshotValue{}, err
		}
		return snapshotValue{Kind: valueTypeTuple, Items: items}, nil
	case *starlark.List:
		return e.encodeList(value)
	case *starlark.Dict:
		return e.encodeDict(value)
	default:
		return snapshotValue{}, fmt.Errorf("unsupported value type %s (%T)", value.Type(), value)
	}
}

func (e *Encoder) encodeTuple(tuple starlark.Tuple) ([]snapshotValue, error) {
	items := make([]snapshotValue, 0, tuple.Len())
	for item := range tuple.Elements() {
		encoded, err := e.encode(item)
		if err != nil {
			return nil, err
		}
		items = append(items, encoded)
	}
	return items, nil
}

func (e *Encoder) encodeList(list *starlark.List) (snapshotValue, error) {
	if id, ok := e.lists[list]; ok {
		return snapshotValue{Kind: valueTypeRef, Ref: id}, nil
	}

	id := e.nextObjectID()
	e.lists[list] = id
	e.objects[id] = object{Kind: objectKindList}

	items := make([]snapshotValue, 0, list.Len())
	for item := range list.Elements() {
		encoded, err := e.encode(item)
		if err != nil {
			return snapshotValue{}, err
		}
		items = append(items, encoded)
	}
	e.objects[id] = object{Kind: objectKindList, Items: items}
	return snapshotValue{Kind: valueTypeRef, Ref: id}, nil
}

func (e *Encoder) encodeDict(dict *starlark.Dict) (snapshotValue, error) {
	if id, ok := e.dicts[dict]; ok {
		return snapshotValue{Kind: valueTypeRef, Ref: id}, nil
	}

	id := e.nextObjectID()
	e.dicts[dict] = id
	e.objects[id] = object{Kind: objectKindDict}

	entries := make([]objectEntry, 0, dict.Len())
	for key, value := range dict.Entries() {
		encodedKey, err := e.encode(key)
		if err != nil {
			return snapshotValue{}, err
		}
		encodedEntryValue, err := e.encode(value)
		if err != nil {
			return snapshotValue{}, err
		}
		entries = append(entries, objectEntry{Key: encodedKey, Value: encodedEntryValue})
	}
	e.objects[id] = object{Kind: objectKindDict, Entries: entries}
	return snapshotValue{Kind: valueTypeRef, Ref: id}, nil
}

func (e *Encoder) nextObjectID() int {
	e.nextID++
	return e.nextID
}

func (d *Decoder) decode(value snapshotValue) (starlark.Value, error) {
	switch value.Kind {
	case valueTypeNone:
		return starlark.None, nil
	case valueTypeBool:
		return starlark.Bool(value.Bool), nil
	case valueTypeInt:
		bigInt, ok := new(big.Int).SetString(value.Text, 10)
		if !ok {
			return nil, fmt.Errorf("invalid int %q", value.Text)
		}
		return starlark.MakeBigInt(bigInt), nil
	case valueTypeFloat:
		return starlark.Float(value.Float), nil
	case valueTypeString:
		return starlark.String(value.Text), nil
	case valueTypeTuple:
		items := make([]starlark.Value, 0, len(value.Items))
		for _, item := range value.Items {
			decoded, err := d.decode(item)
			if err != nil {
				return nil, err
			}
			items = append(items, decoded)
		}
		return starlark.Tuple(items), nil
	case valueTypeRef:
		return d.decodeObject(value.Ref)
	default:
		return nil, fmt.Errorf("unknown value kind %d", value.Kind)
	}
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
	default:
		return nil, fmt.Errorf("unknown object kind %d", object.Kind)
	}
}
