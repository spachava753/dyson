package starlark

type Bool bool

const True Bool = true

type Value interface {
	String() string
	Type() string
	Freeze()
	Truth() Bool
	Hash() (uint32, error)
}

type Thread struct{}

type Builtin struct{}

type Tuple []Value

func (Tuple) String() string        { return "" }
func (Tuple) Type() string          { return "tuple" }
func (Tuple) Freeze()               {}
func (Tuple) Truth() Bool           { return true }
func (Tuple) Hash() (uint32, error) { return 0, nil }

type String string

func (String) String() string        { return "" }
func (String) Type() string          { return "string" }
func (String) Freeze()               {}
func (String) Truth() Bool           { return true }
func (String) Hash() (uint32, error) { return 0, nil }

type Int struct{}

func MakeInt(int) Int             { return Int{} }
func (Int) String() string        { return "" }
func (Int) Type() string          { return "int" }
func (Int) Freeze()               {}
func (Int) Truth() Bool           { return true }
func (Int) Hash() (uint32, error) { return 0, nil }

type List struct{}

func NewList([]Value) *List         { return &List{} }
func (*List) String() string        { return "" }
func (*List) Type() string          { return "list" }
func (*List) Freeze()               {}
func (*List) Truth() Bool           { return true }
func (*List) Hash() (uint32, error) { return 0, nil }

type StringDict map[string]Value
