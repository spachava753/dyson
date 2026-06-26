package snapshot

import (
	"fmt"

	"go.starlark.net/starlark"
)

// Converter is implemented by a custom [starlark.Value].
// This is useful when a module defines a custom type, but we
// need to snapshot it as concrete supported Starlark values.
type Converter interface {
	starlark.Value
	ToValue() (starlark.Value, error)
}

// RestorerFunc rebuilds a custom value from its snapshot-supported value.
type RestorerFunc func(starlark.Value) (starlark.Value, error)

var registry = map[string]RestorerFunc{}

// RegisterRestorer registers the decode hook for a custom Starlark value type.
func RegisterRestorer(typeName string, r RestorerFunc) {
	if typeName == "" {
		panic("snapshot: empty restorer type")
	}
	if r == nil {
		panic(fmt.Sprintf("snapshot: nil restorer for type %s", typeName))
	}
	if _, ok := registry[typeName]; ok {
		panic(fmt.Sprintf("snapshot: registered duplicate type %s", typeName))
	}
	registry[typeName] = r
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
	objectKindSet
)

type objectEntry struct {
	Key   snapshotValue `json:"key"`
	Value snapshotValue `json:"value"`
}

type snapshotValue struct {
	Kind     valueType       `json:"kind"`
	TypeName string          `json:"type_name,omitempty"`
	Ref      int             `json:"ref,omitempty"`
	Bool     bool            `json:"bool,omitempty"`
	Float    float64         `json:"float,omitempty"`
	Text     string          `json:"text,omitempty"`
	Items    []snapshotValue `json:"items,omitempty"`
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
	valueTypeBytes
)
