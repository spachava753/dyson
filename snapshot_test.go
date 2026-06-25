package dyson

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/nalgeon/be"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

func TestREPLSnapshotRestoresGlobalsOnNewThread(t *testing.T) {
	opts := &syntax.FileOptions{GlobalReassign: true}
	globals := starlark.StringDict{}
	thread := newTestREPLThread("first")

	runREPLCell(t, opts, thread, globals, `counter = 1`)
	runREPLCell(t, opts, thread, globals, `items = ["first"]`)
	runREPLCell(t, opts, thread, globals, `alias = items`)
	runREPLCell(t, opts, thread, globals, `state = {"counter": counter, "items": items}`)

	var buf bytes.Buffer
	be.Err(t, NewEncoder(&buf).Encode(globals), nil)
	snapshot := buf.Bytes()
	be.True(t, json.Valid(snapshot))

	var restoredGlobals starlark.StringDict
	be.Err(t, NewDecoder(bytes.NewReader(snapshot)).Decode(&restoredGlobals), nil)
	restoredThread := newTestREPLThread("restored")

	runREPLCell(t, opts, restoredThread, restoredGlobals, `counter += 41`)
	runREPLCell(t, opts, restoredThread, restoredGlobals, `alias.append("second")`)
	runREPLCell(t, opts, restoredThread, restoredGlobals, `state["counter"] = counter`)
	runREPLCell(t, opts, restoredThread, restoredGlobals, `state["items"].append("third")`)

	assertEvalTrue(t, opts, restoredThread, restoredGlobals, `counter == 42`)
	assertEvalTrue(t, opts, restoredThread, restoredGlobals, `items == ["first", "second", "third"]`)
	assertEvalTrue(t, opts, restoredThread, restoredGlobals, `alias == items`)
	assertEvalTrue(t, opts, restoredThread, restoredGlobals, `state == {"counter": 42, "items": ["first", "second", "third"]}`)
}

func TestEncoderRejectsInvalidGlobals(t *testing.T) {
	tests := []struct {
		name     string
		globals  starlark.StringDict
		wantErrs []string
	}{
		{
			name: "unsupported value",
			globals: starlark.StringDict{
				"fn": starlark.NewBuiltin("fn", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
					return starlark.None, nil
				}),
			},
			wantErrs: []string{"encode global fn", "unsupported value type"},
		},
		{
			name:     "non finite float",
			globals:  starlark.StringDict{"nan": starlark.Float(math.NaN())},
			wantErrs: []string{"unsupported non-finite float"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := NewEncoder(&buf).Encode(tt.globals)
			be.Err(t, err)
			assertErrorContains(t, err, tt.wantErrs...)
		})
	}
}

func TestDecoderRejectsInvalidSnapshots(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		nilDst   bool
		wantErrs []string
	}{
		{
			name:     "nil target",
			input:    `{"globals":{}}`,
			nilDst:   true,
			wantErrs: []string{"nil target"},
		},
		{
			name:     "unknown object ref",
			input:    `{"globals":{"x":{"kind":7,"ref":99}}}`,
			wantErrs: []string{"decode global x", "unknown object ref 99"},
		},
		{
			name:     "unknown value kind",
			input:    `{"globals":{"x":{"kind":99}}}`,
			wantErrs: []string{"decode global x", "unknown value kind 99"},
		},
		{
			name:     "unknown object kind",
			input:    `{"globals":{"x":{"kind":7,"ref":1}},"objects":{"1":{"kind":99}}}`,
			wantErrs: []string{"decode global x", "unknown object kind 99"},
		},
		{
			name:     "invalid int",
			input:    `{"globals":{"x":{"kind":3,"text":"not-an-int"}}}`,
			wantErrs: []string{"decode global x", `invalid int "not-an-int"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder := NewDecoder(strings.NewReader(tt.input))
			var globals starlark.StringDict
			var err error
			if tt.nilDst {
				err = decoder.Decode(nil)
			} else {
				err = decoder.Decode(&globals)
			}
			be.Err(t, err)
			assertErrorContains(t, err, tt.wantErrs...)
		})
	}
}

func assertErrorContains(t *testing.T, err error, wants ...string) {
	t.Helper()

	for _, want := range wants {
		be.Err(t, err, want)
	}
}

func newTestREPLThread(name string) *starlark.Thread {
	return &starlark.Thread{
		Name: name,
		Print: func(thread *starlark.Thread, msg string) {
			// Tests intentionally discard REPL print output.
		},
	}
}

func runREPLCell(t *testing.T, opts *syntax.FileOptions, thread *starlark.Thread, globals starlark.StringDict, src string) {
	t.Helper()

	f, err := opts.Parse("<repl>", strings.TrimSpace(src)+"\n", 0)
	be.Err(t, err, nil)
	be.Err(t, starlark.ExecREPLChunk(f, thread, globals), nil)
}

func assertEvalTrue(t *testing.T, opts *syntax.FileOptions, thread *starlark.Thread, globals starlark.StringDict, expr string) {
	t.Helper()

	v, err := starlark.EvalOptions(opts, thread, "<expr>", expr, globals)
	be.Err(t, err, nil)
	be.Equal(t, v, starlark.Value(starlark.True))
}
