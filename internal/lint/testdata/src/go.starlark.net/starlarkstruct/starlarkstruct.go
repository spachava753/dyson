package starlarkstruct

import "go.starlark.net/starlark"

type Struct struct{}

func FromStringDict(starlark.String, starlark.StringDict) *Struct { return &Struct{} }

func (*Struct) String() string        { return "" }
func (*Struct) Type() string          { return "struct" }
func (*Struct) Freeze()               {}
func (*Struct) Truth() starlark.Bool  { return true }
func (*Struct) Hash() (uint32, error) { return 0, nil }
