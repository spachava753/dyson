package dyson

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/spachava753/dyson/internal/codec"
	"go.starlark.net/starlark"
)

// HostCall is one recorded crossing from Starlark into a wrapped Go builtin.
// Args, kwargs, and response are serialized before the event is appended so the
// log contains durable payloads rather than pointers into the running VM.
type HostCall struct {
	FnName   string
	Args     codec.SerializedVal
	Kwargs   []codec.SerializedVal
	Response codec.SerializedVal
	Err      *HostCallError
}

// HostCallError is the durable form of a builtin error. Starlark has no
// exception types, so replay preserves the visible message without maintaining
// a second Go error type system.
type HostCallError struct {
	Message string
}

// ReplChunk records one submitted REPL chunk and the host calls it produced.
// Failed chunks are still retained because mutations before a builtin error may
// have become visible in the session globals.
type ReplChunk struct {
	Code  string
	Calls []HostCall
	Error string
}

// DurableBuiltin wraps a Starlark builtin so Dyson can record every host call.
// The embedded builtin still performs the actual function behavior.
type DurableBuiltin struct {
	*starlark.Builtin
	s *Sphere
}

// record appends an already-serialized host event to the currently executing
// chunk. Eval appends the chunk before execution, so any wrapped builtin called
// during the chunk can safely attach its event to the last log entry.
func (s *Sphere) record(
	fnname string,
	args codec.SerializedVal,
	kwargs []codec.SerializedVal,
	resp codec.SerializedVal,
	respErr error,
) {
	if !s.recordingEnabled {
		return
	}
	currChunk := s.log[len(s.log)-1]
	currChunk.Calls = append(currChunk.Calls, HostCall{
		FnName:   fnname,
		Args:     args,
		Kwargs:   kwargs,
		Response: resp,
		Err:      hostCallError(respErr),
	})
	s.log[len(s.log)-1] = currChunk
}

// CallInternal records the durable boundary around a host builtin. When
// recording is disabled, it calls the builtin directly without codec work.
// Otherwise, inputs are serialized before the call so unsupported values abort
// without performing the host effect; outputs or Go errors are recorded after
// the call returns.
// TODO: do we need to override [starlark.Builtin.BindReceiver] too?
func (d *DurableBuiltin) CallInternal(thread *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if d.s.replaying {
		serializedArgs, serializedKwargs, err := d.s.serializeCallInputs(args, kwargs)
		if err != nil {
			return nil, err
		}
		if d.s.replayChunk >= len(d.s.log) || d.s.replayStep >= len(d.s.log[d.s.replayChunk].Calls) {
			return nil, fmt.Errorf("dyson: replay has no host call for %s", d.Builtin.Name())
		}
		capturedCall := d.s.log[d.s.replayChunk].Calls[d.s.replayStep]
		d.s.replayStep++
		if capturedCall.FnName != d.Builtin.Name() {
			return nil, fmt.Errorf("dyson: replay expected %s, got %s", capturedCall.FnName, d.Builtin.Name())
		}
		if !reflect.DeepEqual(capturedCall.Args, serializedArgs) || !reflect.DeepEqual(capturedCall.Kwargs, serializedKwargs) {
			return nil, fmt.Errorf("dyson: replay inputs for %s differ from the recorded call", d.Builtin.Name())
		}
		if capturedCall.Err != nil {
			return nil, errors.New(capturedCall.Err.Message)
		}
		return d.s.codecs.Restore(capturedCall.Response)
	}
	if !d.s.recordingEnabled {
		return d.Builtin.CallInternal(thread, args, kwargs)
	}

	serializedArgs, serializedKwargs, err := d.s.serializeCallInputs(args, kwargs)
	if err != nil {
		return nil, err
	}

	resp, respErr := d.Builtin.CallInternal(thread, args, kwargs)
	var serializedResp codec.SerializedVal
	if respErr == nil && resp != nil {
		serializedResp, err = d.s.codecs.Serialize(resp)
		if err != nil {
			d.s.record(d.Builtin.Name(), serializedArgs, serializedKwargs, codec.SerializedVal{}, err)
			return nil, err
		}
	}
	d.s.record(d.Builtin.Name(), serializedArgs, serializedKwargs, serializedResp, respErr)
	return resp, respErr
}

func hostCallError(err error) *HostCallError {
	if err == nil {
		return nil
	}
	return &HostCallError{Message: err.Error()}
}

// serializeCallInputs validates and serializes inputs before the real host
// builtin runs. This is important for record-replay: unsupported inputs should
// fail before an external effect can occur.
func (s *Sphere) serializeCallInputs(args starlark.Tuple, kwargs []starlark.Tuple) (codec.SerializedVal, []codec.SerializedVal, error) {
	serializedArgs, err := s.codecs.Serialize(args)
	if err != nil {
		return codec.SerializedVal{}, nil, err
	}
	serializedKwargs := make([]codec.SerializedVal, len(kwargs))
	for i, kwarg := range kwargs {
		serializedKwargs[i], err = s.codecs.Serialize(kwarg)
		if err != nil {
			return codec.SerializedVal{}, nil, err
		}
	}
	return serializedArgs, serializedKwargs, nil
}
