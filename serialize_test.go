package dyson

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nalgeon/be"
	"go.starlark.net/starlark"
)

func mustSerialize(t *testing.T, registry CodecRegistry, val starlark.Value) SerializedVal {
	t.Helper()
	sv, err := registry.Serialize(val)
	be.Err(t, err, nil)
	return sv
}

func TestSerializePrimitiveData(t *testing.T) {
	registry := DefaultCodecRegistry()
	bigInt := starlark.MakeInt64(1)
	bigInt = bigInt.Lsh(80)

	cases := []struct {
		name string
		val  starlark.Value
		data []byte
	}{
		{name: "none", val: starlark.None, data: []byte{}},
		{name: "bool", val: starlark.Bool(true), data: []byte{1}},
		{name: "int", val: starlark.MakeInt(42), data: []byte("42")},
		{name: "big_int", val: bigInt, data: []byte("1208925819614629174706176")},
		{name: "float", val: starlark.Float(1.25), data: []byte{0x3f, 0xf4, 0, 0, 0, 0, 0, 0}},
		{name: "string", val: starlark.String("hello"), data: []byte("hello")},
		{name: "bytes", val: starlark.Bytes("hello"), data: []byte("hello")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mustSerialize(t, registry, tc.val)
			be.Equal(t, got.Type, tc.val.Type())
			be.Equal(t, got.Version, 1)
			be.Equal(t, got.Data, tc.data)

			restored, err := registry.Restore(got)
			be.Err(t, err, nil)
			be.Equal(t, restored.Type(), tc.val.Type())
			be.Equal(t, restored.String(), tc.val.String())
		})
	}
}

type serializedCustomValue struct {
	payload string
}

func (v serializedCustomValue) String() string        { return v.payload }
func (v serializedCustomValue) Type() string          { return "custom.serialized" }
func (v serializedCustomValue) Freeze()               {}
func (v serializedCustomValue) Truth() starlark.Bool  { return starlark.Bool(v.payload != "") }
func (v serializedCustomValue) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable") }

type fallbackCustomValue struct {
	payload string
}

func (v fallbackCustomValue) String() string        { return v.payload }
func (v fallbackCustomValue) Type() string          { return "custom.fallback" }
func (v fallbackCustomValue) Freeze()               {}
func (v fallbackCustomValue) Truth() starlark.Bool  { return starlark.Bool(v.payload != "") }
func (v fallbackCustomValue) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable") }

func TestSerializeCustomValueData(t *testing.T) {
	registry := DefaultCodecRegistry()
	customType := serializedCustomValue{payload: ""}.Type()
	registry.Register(ValueCodec{
		Type:    customType,
		Version: 1,
		Serialize: func(val starlark.Value) (SerializedVal, error) {
			v, ok := val.(serializedCustomValue)
			if !ok {
				return SerializedVal{}, fmt.Errorf("got %T for custom codec", val)
			}
			return SerializedVal{Data: []byte(v.payload)}, nil
		},
		Restore: func(val SerializedVal) (starlark.Value, error) {
			return serializedCustomValue{payload: string(val.Data)}, nil
		},
	})

	custom := serializedCustomValue{payload: "kept"}
	be.Equal(t, mustSerialize(t, registry, custom), SerializedVal{
		Type:    "custom.serialized",
		Version: 1,
		Data:    []byte("kept"),
	})

	fallback := fallbackCustomValue{payload: "opaque"}
	_, err := registry.Serialize(fallback)
	if err == nil {
		t.Fatal("expected fallback custom value serialization to fail")
	}
	if !strings.Contains(err.Error(), "custom.fallback") {
		t.Fatalf("serialization error %q does not mention custom.fallback", err.Error())
	}
}
