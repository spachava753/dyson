package dyson

import "go.starlark.net/starlark"

// HostCall is one recorded crossing from Starlark into a wrapped Go builtin.
// Args, kwargs, and response are serialized before the event is appended so the
// log contains durable payloads rather than pointers into the running VM.
type HostCall struct {
	FnName   string
	Args     SerializedVal
	Kwargs   []SerializedVal
	Response SerializedVal
	Err      any
}

// ReplChunk records one submitted REPL chunk and the host calls it produced.
// Failed chunks are still retained because mutations before a builtin error may
// have become visible in the session globals.
type ReplChunk struct {
	Code  string
	Calls []HostCall
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
	args SerializedVal,
	kwargs []SerializedVal,
	resp SerializedVal,
	respErr error,
) {
	currChunk := s.log[len(s.log)-1]
	currChunk.Calls = append(currChunk.Calls, HostCall{
		FnName:   fnname,
		Args:     args,
		Kwargs:   kwargs,
		Response: resp,
		Err:      respErr,
	})
	s.log[len(s.log)-1] = currChunk
}

// CallInternal records the durable boundary around a host builtin. Inputs are
// serialized before the call so unsupported values abort without performing the
// host effect; outputs or Go errors are recorded after the call returns.
// TODO: do we need to override [starlark.Builtin.BindReceiver] too?
func (d *DurableBuiltin) CallInternal(thread *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if d.s.replaying {
		capturedCall := d.s.log[d.s.replayChunk].Calls[d.s.replayStep]
		d.s.replayStep++
		// TODO: verify hash of input is the same
		// TODO: maybe if we know that this builtin called callbacks before, we can re-execute? We would need to store the fact that this called a call back before
		return d.s.codecs[capturedCall.Response.Type].Restore(capturedCall.Response)
	}
	serializedArgs, serializedKwargs, err := d.s.serializeCallInputs(args, kwargs)
	if err != nil {
		return nil, err
	}

	resp, respErr := d.Builtin.CallInternal(thread, args, kwargs)
	var serializedResp SerializedVal
	if resp != nil {
		serializedResp, err = d.s.codecs.Serialize(resp)
		if err != nil {
			return nil, err
		}
	}
	d.s.record(d.Builtin.Name(), serializedArgs, serializedKwargs, serializedResp, respErr)
	return resp, respErr
}

// serializeCallInputs validates and serializes inputs before the real host
// builtin runs. This is important for record-replay: unsupported inputs should
// fail before an external effect can occur.
func (s *Sphere) serializeCallInputs(args starlark.Tuple, kwargs []starlark.Tuple) (SerializedVal, []SerializedVal, error) {
	serializedArgs, err := s.codecs.Serialize(args)
	if err != nil {
		return SerializedVal{}, nil, err
	}
	serializedKwargs := make([]SerializedVal, len(kwargs))
	for i, kwarg := range kwargs {
		serializedKwargs[i], err = s.codecs.Serialize(kwarg)
		if err != nil {
			return SerializedVal{}, nil, err
		}
	}
	return serializedArgs, serializedKwargs, nil
}
