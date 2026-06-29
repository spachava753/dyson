package dyson

import (
	"context"

	"go.starlark.net/repl"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// Sphere is a type for defining the REPL to use
type Sphere struct {
	Log []ReplChunk

	t     *starlark.Thread
	g     starlark.StringDict
	fopts *syntax.FileOptions
}

func NewSphere(
	print func(thread *starlark.Thread, msg string),
	modules map[string]starlark.StringDict,
) *Sphere {
	fopts := &syntax.FileOptions{
		Set:               true,
		While:             true,
		TopLevelControl:   true,
		GlobalReassign:    true,
		LoadBindsGlobally: false,
		Recursion:         false,
	}
	s := &Sphere{fopts: fopts, g: make(starlark.StringDict)}

	for _, sd := range modules {
		for member, val := range sd {
			b, ok := val.(*starlark.Builtin)
			if !ok {
				continue
			}
			sd[member] = &DurableBuiltin{Builtin: b, s: s}
		}
	}

	l := repl.MakeLoadOptions(fopts)
	s.t = &starlark.Thread{
		Name:  "dyson.sphere",
		Print: print,
		Load: func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
			if m, ok := modules[module]; ok {
				return m, nil
			}
			return l(thread, module)
		},
	}
	return s
}

// Eval evaluated submitted starlark code.
//
// Note: when using [starlark.ExecREPLChunk] it automatically
// brings in [starlark.Universe]
func (s *Sphere) Eval(ctx context.Context, code string) error {
	f, err := s.fopts.Parse("<dyson_sphere_repl>", code, 0)
	if err != nil {
		return err
	}
	s.Log = append(s.Log, ReplChunk{
		Code: code,
	})
	return starlark.ExecREPLChunk(f, s.t, s.g)
}

func (s *Sphere) record(
	fnname string,
	args starlark.Tuple,
	kwargs []starlark.Tuple,
	resp starlark.Value,
	respErr error,
) {
	currChunk := s.Log[len(s.Log)-1]
	k := make([]SerializedVal, len(kwargs))
	for i, x := range kwargs {
		k[i] = serialize(x)
	}
	var response SerializedVal
	if resp != nil {
		response = serialize(resp)
	}
	currChunk.Calls = append(currChunk.Calls, HostCall{
		FnName:   fnname,
		Args:     serialize(args),
		Kwargs:   k,
		Response: response,
		Err:      respErr,
	})
	s.Log[len(s.Log)-1] = currChunk
}

type DurableBuiltin struct {
	*starlark.Builtin
	s *Sphere
}

// TODO: do we need to override [starlark.Builtin.BindReceiver] too?
func (d *DurableBuiltin) CallInternal(thread *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	resp, err := d.Builtin.CallInternal(thread, args, kwargs)
	d.s.record(d.Builtin.Name(), args, kwargs, resp, err)
	return resp, err
}

func serialize(val starlark.Value) SerializedVal {
	if val == nil {
		return SerializedVal{}
	}
	if val == starlark.None {
		return serializeHashable(val)
	}
	switch val.(type) {
	case starlark.Bool, starlark.Int, starlark.Float, starlark.String, starlark.Bytes:
		return serializeHashable(val)
	}
	if v, ok := val.(starlark.Indexable); ok {
		sv := SerializedVal{
			Type: v.Type(),
			List: make([]SerializedVal, v.Len()),
		}
		for i := range v.Len() {
			sv.List[i] = serialize(v.Index(i))
		}
		return sv
	}
	if v, ok := val.(starlark.IterableMapping); ok {
		items := v.Items()
		sv := SerializedVal{
			Type:       v.Type(),
			DictKeys:   make([]SerializedVal, len(items)),
			DictValues: make([]SerializedVal, len(items)),
		}
		for i := range items {
			sv.DictKeys[i] = serialize(items[i][0])
			sv.DictValues[i] = serialize(items[i][1])
		}
		return sv
	}
	return serializeHashable(val)
}

func serializeHashable(val starlark.Value) SerializedVal {
	hash, err := val.Hash()
	if err == nil {
		return SerializedVal{
			Type: val.Type(),
			Hash: hash,
		}
	}
	return SerializedVal{
		Type: val.Type(),
		// TODO: hash not supported, how can serialize?
	}
}

type Serializer interface {
	Serialize() SerializedVal
}

type SerializedVal struct {
	Type       string
	Hash       uint32
	List       []SerializedVal
	DictKeys   []SerializedVal
	DictValues []SerializedVal
	Data       any
}

type HostCall struct {
	FnName   string
	Args     SerializedVal
	Kwargs   []SerializedVal
	Response SerializedVal
	Err      any
}

// TODO: how do we represent a cancelled thread?
type ReplChunk struct {
	Code  string
	Calls []HostCall
}
