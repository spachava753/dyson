package builtins

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// textDecoder incrementally converts UTF-8 to text for bounded file reads.
type textDecoder struct {
	errors  string
	pending []byte
	offset  int
}

func newTextDecoder(encoding, errors string) (*textDecoder, error) {
	if !isUTF8Encoding(encoding) {
		return nil, fmt.Errorf("unsupported encoding %q", encoding)
	}
	if errors != "strict" && errors != "ignore" && errors != "replace" {
		return nil, fmt.Errorf("unsupported error handler %q", errors)
	}
	return &textDecoder{errors: errors}, nil
}

func (*textDecoder) maxBytesPerCharacter() int { return utf8.UTFMax }

func (d *textDecoder) decode(data []byte, final bool) (string, error) {
	base := d.offset - len(d.pending)
	input := make([]byte, 0, len(d.pending)+len(data))
	input = append(input, d.pending...)
	input = append(input, data...)
	d.pending = nil
	d.offset += len(data)

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
}

// utf8Sequence classifies the first non-empty input sequence under RFC 3629.
// width is the number of bytes to consume for a valid or malformed sequence;
// incomplete is reported only when a non-final chunk may provide more bytes.
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

func isUTF8Encoding(name string) bool {
	var normalized strings.Builder
	separator := false
	for _, character := range name {
		if character >= 'A' && character <= 'Z' {
			character += 'a' - 'A'
		}
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '.' {
			if separator && normalized.Len() > 0 {
				normalized.WriteByte('_')
			}
			normalized.WriteRune(character)
			separator = false
		} else {
			separator = true
		}
	}

	switch normalized.String() {
	case "utf_8", "utf8", "u8", "utf", "cp65001", "utf8_ucs2", "utf8_ucs4":
		return true
	default:
		return false
	}
}
