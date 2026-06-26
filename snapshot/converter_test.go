package snapshot

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/nalgeon/be"
	"go.starlark.net/starlark"
)

type snapshotTestValue struct {
	typeName string
	payload  starlark.Value
}

func (v snapshotTestValue) String() string {
	return fmt.Sprintf("%s(%s)", v.typeName, v.payload)
}

func (v snapshotTestValue) Type() string {
	return v.typeName
}

func (v snapshotTestValue) Freeze() {}

func (v snapshotTestValue) Truth() starlark.Bool {
	return starlark.True
}

func (v snapshotTestValue) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable: %s", v.Type())
}

func (v snapshotTestValue) ToValue() (starlark.Value, error) {
	return v.payload, nil
}

func TestConverterRoundTripsNestedValues(t *testing.T) {
	const typeName = "snapshot.test.nested"
	RegisterRestorer(typeName, func(value starlark.Value) (starlark.Value, error) {
		return snapshotTestValue{typeName: typeName, payload: value}, nil
	})
	t.Cleanup(func() { delete(registry, typeName) })

	custom := snapshotTestValue{typeName: typeName, payload: starlark.String("payload")}
	list := starlark.NewList([]starlark.Value{custom})
	dict := starlark.NewDict(1)
	be.Err(t, dict.SetKey(starlark.String("item"), custom), nil)

	var buf bytes.Buffer
	be.Err(t, NewEncoder(&buf).Encode(starlark.StringDict{
		"custom": custom,
		"dict":   dict,
		"list":   list,
		"tuple":  starlark.Tuple{custom},
	}), nil)

	var restored starlark.StringDict
	be.Err(t, NewDecoder(bytes.NewReader(buf.Bytes())).Decode(&restored), nil)
	assertSnapshotTestValue(t, restored["custom"], typeName, starlark.String("payload"))

	restoredList, ok := restored["list"].(*starlark.List)
	be.True(t, ok)
	assertSnapshotTestValue(t, restoredList.Index(0), typeName, starlark.String("payload"))

	restoredDict, ok := restored["dict"].(*starlark.Dict)
	be.True(t, ok)
	restoredDictValue, found, err := restoredDict.Get(starlark.String("item"))
	be.Err(t, err, nil)
	be.True(t, found)
	assertSnapshotTestValue(t, restoredDictValue, typeName, starlark.String("payload"))

	restoredTuple, ok := restored["tuple"].(starlark.Tuple)
	be.True(t, ok)
	assertSnapshotTestValue(t, restoredTuple[0], typeName, starlark.String("payload"))
}

func TestRegisterRestorerRejectsInvalidInput(t *testing.T) {
	expectPanicContains(t, "empty restorer type", func() {
		RegisterRestorer("", func(value starlark.Value) (starlark.Value, error) {
			return value, nil
		})
	})
	expectPanicContains(t, "nil restorer", func() {
		RegisterRestorer("snapshot.test.nil", nil)
	})

	RegisterRestorer("snapshot.test.duplicate", func(value starlark.Value) (starlark.Value, error) {
		return value, nil
	})
	t.Cleanup(func() { delete(registry, "snapshot.test.duplicate") })
	expectPanicContains(t, "duplicate type", func() {
		RegisterRestorer("snapshot.test.duplicate", func(value starlark.Value) (starlark.Value, error) {
			return value, nil
		})
	})
}

func assertSnapshotTestValue(t *testing.T, value starlark.Value, typeName string, payload starlark.Value) {
	t.Helper()

	custom, ok := value.(snapshotTestValue)
	be.True(t, ok)
	be.Equal(t, custom.typeName, typeName)
	be.Equal(t, custom.payload, payload)
}

func expectPanicContains(t *testing.T, want string, fn func()) {
	t.Helper()

	defer func() {
		recovered := recover()
		be.True(t, recovered != nil)
		be.True(t, strings.Contains(fmt.Sprint(recovered), want))
	}()
	fn()
}
