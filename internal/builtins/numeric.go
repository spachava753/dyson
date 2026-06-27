package builtins

import (
	"fmt"
	"math"
	"math/big"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// absBuiltin returns the absolute value of an int or float.
//
// It mirrors the supported subset of Python's absBuiltin:
// https://docs.python.org/3/library/functions.html#absBuiltin
func absBuiltin(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var val starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "x", &val); err != nil {
		return nil, err
	}
	if v, ok := val.(starlark.Int); ok {
		if v.Sign() < 0 {
			return starlark.MakeInt(0).Sub(v), nil
		}
		return v, nil
	}
	if v, ok := starlark.AsFloat(val); ok {
		return starlark.Float(math.Abs(v)), nil
	}
	return nil, fmt.Errorf("%s: bad operand type for abs(): %s", fn.Name(), val.Type())
}

// roundBuiltin implements the Starlark round builtin for int and float values,
// returning an int when ndigits is omitted and preserving float output when
// ndigits is supplied for a float input.
//
// It mirrors the supported subset of Python's round:
// https://docs.python.org/3/library/functions.html#round
func roundBuiltin(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var number starlark.Value
	ndigits := starlark.Value(starlark.None)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "number", &number, "ndigits?", &ndigits); err != nil {
		return nil, err
	}
	if ndigits == starlark.None {
		switch v := number.(type) {
		case starlark.Int:
			return v, nil
		case starlark.Float:
			return starlark.MakeInt64(int64(math.Round(float64(v)))), nil
		default:
			return nil, fmt.Errorf("%s: number must be int or float, got %s", fn.Name(), number.Type())
		}
	}
	digits, err := int64Value(fn.Name(), "ndigits", ndigits)
	if err != nil {
		return nil, err
	}
	switch v := number.(type) {
	case starlark.Int:
		return v, nil
	case starlark.Float:
		pow := math.Pow10(int(digits))
		return starlark.Float(math.Round(float64(v)*pow) / pow), nil
	default:
		return nil, fmt.Errorf("%s: number must be int or float, got %s", fn.Name(), number.Type())
	}
}

// sumBuiltin implements the Starlark sum builtin, adding values from an
// iterable to an optional start value using Starlark's + operator.
//
// It mirrors the supported subset of Python's sum:
// https://docs.python.org/3/library/functions.html#sum
func sumBuiltin(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var iterable starlark.Value
	var start starlark.Value = starlark.MakeInt(0)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "iterable", &iterable, "start?", &start); err != nil {
		return nil, err
	}
	values, err := iterableValues(fn.Name(), iterable)
	if err != nil {
		return nil, err
	}
	total := start
	for _, value := range values {
		next, err := starlark.Binary(syntax.PLUS, total, value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fn.Name(), err)
		}
		total = next
	}
	return total, nil
}

// powBuiltin implements the Starlark pow builtin for int and float values,
// including Python-compatible modular exponentiation for integer arguments.
//
// It mirrors the supported subset of Python's pow:
// https://docs.python.org/3/library/functions.html#pow
func powBuiltin(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var base, exp starlark.Value
	var mod starlark.Value = starlark.None
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "base", &base, "exp", &exp, "mod?", &mod); err != nil {
		return nil, err
	}
	baseInt, baseIsInt := base.(starlark.Int)
	expInt, expIsInt := exp.(starlark.Int)
	if mod != starlark.None {
		modInt, err := intValue(fn.Name(), "mod", mod)
		if err != nil {
			return nil, err
		}
		if !baseIsInt || !expIsInt {
			return nil, fmt.Errorf("%s: pow() 3rd argument not allowed unless all arguments are integers", fn.Name())
		}
		if expInt.Sign() < 0 {
			return nil, fmt.Errorf("%s: exponent must be non-negative when modulus is present", fn.Name())
		}
		if modInt.Sign() == 0 {
			return nil, fmt.Errorf("%s: 3rd argument cannot be 0", fn.Name())
		}
		result := new(big.Int).Exp(baseInt.BigInt(), expInt.BigInt(), modInt.BigInt())
		return starlark.MakeBigInt(result), nil
	}
	if baseIsInt && expIsInt && expInt.Sign() >= 0 {
		result := new(big.Int).Exp(baseInt.BigInt(), expInt.BigInt(), nil)
		return starlark.MakeBigInt(result), nil
	}
	baseFloat, ok := starlark.AsFloat(base)
	if !ok {
		return nil, fmt.Errorf("%s: base must be int or float, got %s", fn.Name(), base.Type())
	}
	expFloat, ok := starlark.AsFloat(exp)
	if !ok {
		return nil, fmt.Errorf("%s: exp must be int or float, got %s", fn.Name(), exp.Type())
	}
	return starlark.Float(math.Pow(baseFloat, expFloat)), nil
}

// binBuiltin implements the Starlark bin builtin, formatting an int with
// Python's 0b prefix and sign placement.
//
// It mirrors the supported subset of Python's bin:
// https://docs.python.org/3/library/functions.html#bin
func binBuiltin(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return baseString(fn.Name(), args, kwargs, 2, "0b")
}

// octBuiltin implements the Starlark oct builtin, formatting an int with
// Python's 0o prefix and sign placement.
//
// It mirrors the supported subset of Python's oct:
// https://docs.python.org/3/library/functions.html#oct
func octBuiltin(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return baseString(fn.Name(), args, kwargs, 8, "0o")
}

// baseString formats a Starlark int in the requested base while preserving
// Python's prefix-after-sign representation for negative values.
func baseString(name string, args starlark.Tuple, kwargs []starlark.Tuple, base int, prefix string) (starlark.Value, error) {
	var value starlark.Value
	if err := starlark.UnpackArgs(name, args, kwargs, "number", &value); err != nil {
		return nil, err
	}
	v, err := intValue(name, "number", value)
	if err != nil {
		return nil, err
	}
	n := v.BigInt()
	if n.Sign() < 0 {
		n = new(big.Int).Neg(n)
		return starlark.String("-" + prefix + n.Text(base)), nil
	}
	return starlark.String(prefix + n.Text(base)), nil
}

// chrBuiltin implements the Starlark chr builtin, converting a Unicode code
// point integer to a one-character string.
//
// It mirrors the supported subset of Python's chr:
// https://docs.python.org/3/library/functions.html#chr
func chrBuiltin(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var value starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "i", &value); err != nil {
		return nil, err
	}
	i, err := int64Value(fn.Name(), "i", value)
	if err != nil {
		return nil, err
	}
	if i < 0 || i > 0x10ffff {
		return nil, fmt.Errorf("%s: arg not in range(0x110000)", fn.Name())
	}
	return starlark.String(string(rune(i))), nil
}

// ordBuiltin implements the Starlark ord builtin, converting a one-character
// string to its Unicode code point integer.
//
// It mirrors the supported subset of Python's ord:
// https://docs.python.org/3/library/functions.html#ord
func ordBuiltin(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var value starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "c", &value); err != nil {
		return nil, err
	}
	s, ok := value.(starlark.String)
	if !ok {
		return nil, fmt.Errorf("%s: expected string of length 1, got %s", fn.Name(), value.Type())
	}
	runes := []rune(string(s))
	if len(runes) != 1 {
		return nil, fmt.Errorf("%s: expected string of length 1", fn.Name())
	}
	return starlark.MakeInt(int(runes[0])), nil
}
