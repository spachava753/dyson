package pybytes

import (
	"fmt"
	"strings"
	"unicode/utf8"
	_ "unsafe"

	"go.starlark.net/starlark"
)

// starlarkBytesMethods is the pinned Starlark runtime's native bytes method
// table. Starlark currently has no public seam for extending built-in methods.
//
//go:linkname starlarkBytesMethods go.starlark.net/starlark.bytesMethods
var starlarkBytesMethods map[string]*starlark.Builtin

func init() {
	if _, exists := starlarkBytesMethods["decode"]; !exists {
		starlarkBytesMethods["decode"] = starlark.NewBuiltin("decode", decodeBuiltin)
	}
}

// New returns native Starlark bytes with Python-compatible decode support.
func New(data []byte) starlark.Bytes {
	return starlark.Bytes(string(data))
}

// NewString returns native Starlark bytes from a raw byte string.
func NewString(data string) starlark.Bytes {
	return starlark.Bytes(data)
}

// ValidateEncoding reports whether encoding is supported by Decode.
func ValidateEncoding(encoding string) error {
	_, err := canonicalEncoding(encoding)
	return err
}

// ValidateErrors reports whether errors is supported by Decode.
func ValidateErrors(errors string) error {
	if errors != "strict" && errors != "ignore" && errors != "replace" {
		return fmt.Errorf("unsupported error handler %q", errors)
	}
	return nil
}

// Decoder incrementally converts one supported encoding to UTF-8 text.
type Decoder struct {
	encoding string
	errors   string
	pending  []byte
	offset   int
}

// NewDecoder returns an incremental decoder for encoding and errors.
func NewDecoder(encoding, errors string) (*Decoder, error) {
	encoding, err := canonicalEncoding(encoding)
	if err != nil {
		return nil, err
	}
	if err := ValidateErrors(errors); err != nil {
		return nil, err
	}
	return &Decoder{encoding: encoding, errors: errors}, nil
}

// MaxBytesPerCharacter returns a bound suitable for sized reads.
func (d *Decoder) MaxBytesPerCharacter() int {
	if d.encoding == "utf-8" {
		return utf8.UTFMax
	}
	return 1
}

// Decode converts the next input chunk. final must be true at end of input so
// an incomplete final sequence follows the configured error policy.
func (d *Decoder) Decode(data []byte, final bool) (string, error) {
	base := d.offset - len(d.pending)
	input := make([]byte, 0, len(d.pending)+len(data))
	input = append(input, d.pending...)
	input = append(input, data...)
	d.pending = nil
	d.offset += len(data)

	switch d.encoding {
	case "utf-8":
		var decoded strings.Builder
		for i := 0; i < len(input); {
			width, valid, incomplete := utf8Sequence(input[i:], final)
			if incomplete {
				d.pending = append(d.pending, input[i:]...)
				break
			}
			if valid {
				decoded.Write(input[i : i+width])
				i += width
				continue
			}
			switch d.errors {
			case "strict":
				return "", fmt.Errorf("'utf-8' codec cannot decode byte 0x%02x at position %d", input[i], base+i)
			case "replace":
				decoded.WriteRune(utf8.RuneError)
			}
			i += width
		}
		return decoded.String(), nil

	case "ascii":
		var decoded strings.Builder
		for i, value := range input {
			if value < utf8.RuneSelf {
				decoded.WriteByte(value)
				continue
			}
			switch d.errors {
			case "strict":
				return "", fmt.Errorf("'ascii' codec cannot decode byte 0x%02x at position %d", value, base+i)
			case "replace":
				decoded.WriteRune(utf8.RuneError)
			}
		}
		return decoded.String(), nil

	case "latin-1":
		decoded := make([]rune, len(input))
		for i, value := range input {
			decoded[i] = rune(value)
		}
		return string(decoded), nil
	}
	panic("unreachable")
}

// Decode converts data to UTF-8 text using a supported Python encoding and
// error handler. Supported encodings are UTF-8, ASCII, and Latin-1.
func Decode(data []byte, encoding, errors string) (string, error) {
	decoder, err := NewDecoder(encoding, errors)
	if err != nil {
		return "", err
	}
	return decoder.Decode(data, true)
}

func decodeBuiltin(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	encoding := "utf-8"
	errors := "strict"
	if err := starlark.UnpackArgs("bytes.decode", args, kwargs, "encoding?", &encoding, "errors?", &errors); err != nil {
		return nil, err
	}
	data, ok := fn.Receiver().(starlark.Bytes)
	if !ok {
		return nil, fmt.Errorf("bytes.decode: receiver is %T, want bytes", fn.Receiver())
	}
	decoded, err := Decode([]byte(data), encoding, errors)
	if err != nil {
		return nil, fmt.Errorf("bytes.decode: %w", err)
	}
	return starlark.String(decoded), nil
}

func utf8Sequence(input []byte, final bool) (width int, valid, incomplete bool) {
	first := input[0]
	if first < utf8.RuneSelf {
		return 1, true, false
	}

	size := 0
	secondMin, secondMax := byte(0x80), byte(0xbf)
	switch {
	case first >= 0xc2 && first <= 0xdf:
		size = 2
	case first == 0xe0:
		size, secondMin = 3, 0xa0
	case first >= 0xe1 && first <= 0xec, first >= 0xee && first <= 0xef:
		size = 3
	case first == 0xed:
		size, secondMax = 3, 0x9f
	case first == 0xf0:
		size, secondMin = 4, 0x90
	case first >= 0xf1 && first <= 0xf3:
		size = 4
	case first == 0xf4:
		size, secondMax = 4, 0x8f
	default:
		return 1, false, false
	}

	for i := 1; i < size; i++ {
		if i == len(input) {
			if !final {
				return 0, false, true
			}
			return len(input), false, false
		}
		minimum, maximum := byte(0x80), byte(0xbf)
		if i == 1 {
			minimum, maximum = secondMin, secondMax
		}
		if input[i] < minimum || input[i] > maximum {
			return i, false, false
		}
	}
	return size, true, false
}

func canonicalEncoding(encoding string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(encoding))
	normalized = strings.NewReplacer("_", "-", " ", "-").Replace(normalized)
	switch normalized {
	case "utf-8", "utf8", "u8":
		return "utf-8", nil
	case "ascii", "us-ascii", "646":
		return "ascii", nil
	case "latin-1", "latin1", "iso-8859-1", "iso8859-1":
		return "latin-1", nil
	default:
		return "", fmt.Errorf("unsupported encoding %q", encoding)
	}
}
