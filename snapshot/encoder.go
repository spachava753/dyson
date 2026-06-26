package snapshot

import (
	"encoding/json"
	"fmt"
	"io"
	"math"

	"go.starlark.net/starlark"
)

// Encoder writes snapshots of supported Starlark global values.
type Encoder struct {
	json *json.Encoder

	nextID int

	// Mutable Starlark containers form a graph, not a tree: multiple globals may
	// point at the same list/dict/set, and containers may even contain themselves.
	// Object ids let refs preserve that aliasing and avoid recursive cycles.
	lists   map[*starlark.List]int
	dicts   map[*starlark.Dict]int
	sets    map[*starlark.Set]int
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
	e.sets = map[*starlark.Set]int{}
	e.objects = map[int]object{}
}

func (e *Encoder) clear() {
	e.nextID = 0
	e.lists = nil
	e.dicts = nil
	e.sets = nil
	e.objects = nil
}

func (e *Encoder) encode(value starlark.Value) (snapshotValue, error) {
	if converter, ok := value.(Converter); ok {
		converted, err := converter.ToValue()
		if err != nil {
			return snapshotValue{}, fmt.Errorf("convert custom type %s: %w", converter.Type(), err)
		}

		encoded, err := e.encode(converted)
		if err != nil {
			return snapshotValue{}, fmt.Errorf("encode custom type %s: %w", converter.Type(), err)
		}
		encoded.TypeName = converter.Type()
		return encoded, nil
	}

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
	case starlark.Bytes:
		return snapshotValue{Kind: valueTypeBytes, Text: string(value)}, nil
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
	case *starlark.Set:
		return e.encodeSet(value)
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

func (e *Encoder) encodeSet(set *starlark.Set) (snapshotValue, error) {
	if id, ok := e.sets[set]; ok {
		return snapshotValue{Kind: valueTypeRef, Ref: id}, nil
	}

	id := e.nextObjectID()
	e.sets[set] = id
	e.objects[id] = object{Kind: objectKindSet}

	items := make([]snapshotValue, 0, set.Len())
	for item := range set.Elements() {
		encoded, err := e.encode(item)
		if err != nil {
			return snapshotValue{}, err
		}
		items = append(items, encoded)
	}
	e.objects[id] = object{Kind: objectKindSet, Items: items}
	return snapshotValue{Kind: valueTypeRef, Ref: id}, nil
}

func (e *Encoder) nextObjectID() int {
	e.nextID++
	return e.nextID
}
