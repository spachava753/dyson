package pybytes

import (
	"strings"
	"testing"

	"github.com/nalgeon/be"
	"go.starlark.net/starlark"
)

func TestNativeBytesDecode(t *testing.T) {
	globals, err := starlark.ExecFile(&starlark.Thread{Name: "test"}, "test.star", `
utf8_text = b"caf\xc3\xa9".decode()
latin1_text = b"caf\xe9".decode("latin-1")
ascii_ignored = b"a\xffb".decode("ascii", "ignore")
ascii_replaced = b"a\xffb".decode(encoding="ascii", errors="replace")
utf8_ignored = b"a\xffb".decode("utf-8", "ignore")
utf8_replaced = b"a\xffb".decode("utf-8", "replace")
utf8_replaced_run = b"\xff\xff".decode("utf-8", "replace")
forward_equal = b"value" == b"value"
`, nil)
	be.Err(t, err, nil)
	be.Equal(t, globals["utf8_text"], starlark.Value(starlark.String("café")))
	be.Equal(t, globals["latin1_text"], starlark.Value(starlark.String("café")))
	be.Equal(t, globals["ascii_ignored"], starlark.Value(starlark.String("ab")))
	be.Equal(t, globals["ascii_replaced"], starlark.Value(starlark.String("a�b")))
	be.Equal(t, globals["utf8_ignored"], starlark.Value(starlark.String("ab")))
	be.Equal(t, globals["utf8_replaced"], starlark.Value(starlark.String("a�b")))
	be.Equal(t, globals["utf8_replaced_run"], starlark.Value(starlark.String("��")))
	be.Equal(t, globals["forward_equal"], starlark.Value(starlark.True))
}

func TestDecoderRetainsIncompleteUTF8AcrossChunks(t *testing.T) {
	decoder, err := NewDecoder("utf-8", "strict")
	be.Err(t, err, nil)

	text, err := decoder.Decode([]byte{0xe2}, false)
	be.Err(t, err, nil)
	be.Equal(t, text, "")
	text, err = decoder.Decode([]byte{0x82, 0xac}, false)
	be.Err(t, err, nil)
	be.Equal(t, text, "€")
	text, err = decoder.Decode(nil, true)
	be.Err(t, err, nil)
	be.Equal(t, text, "")
}

func TestDecoderReplacesIncompleteFinalUTF8Once(t *testing.T) {
	decoder, err := NewDecoder("utf-8", "replace")
	be.Err(t, err, nil)

	text, err := decoder.Decode([]byte{0xe2}, false)
	be.Err(t, err, nil)
	be.Equal(t, text, "")
	text, err = decoder.Decode([]byte{0x82}, true)
	be.Err(t, err, nil)
	be.Equal(t, text, "�")
}

func TestDecoderConsumesMalformedUTF8LikePython(t *testing.T) {
	for _, test := range []struct {
		name  string
		input []byte
		want  string
	}{
		{name: "three-byte prefix", input: []byte{0xe1, 0x80, 'A'}, want: "�A"},
		{name: "four-byte prefix", input: []byte{0xf0, 0x90, 0x80, 'A'}, want: "�A"},
		{name: "overlong sequence", input: []byte{0xe0, 0x80, 0x80}, want: "���"},
		{name: "surrogate sequence", input: []byte{0xed, 0xa0, 0x80}, want: "���"},
	} {
		t.Run(test.name, func(t *testing.T) {
			text, err := Decode(test.input, "utf-8", "replace")
			be.Err(t, err, nil)
			be.Equal(t, text, test.want)
		})
	}
}

func TestDecoderConsumesMalformedUTF8PrefixAcrossChunks(t *testing.T) {
	decoder, err := NewDecoder("utf-8", "replace")
	be.Err(t, err, nil)

	for _, chunk := range [][]byte{{0xe1}, {0x80}} {
		text, err := decoder.Decode(chunk, false)
		be.Err(t, err, nil)
		be.Equal(t, text, "")
	}
	text, err := decoder.Decode([]byte{'A'}, false)
	be.Err(t, err, nil)
	be.Equal(t, text, "�A")
}

func TestNativeBytesDecodeErrors(t *testing.T) {
	for _, test := range []struct {
		name string
		code string
		want string
	}{
		{name: "invalid UTF-8", code: `b"\xff".decode()`, want: "bytes.decode: 'utf-8' codec cannot decode byte 0xff at position 0"},
		{name: "unsupported encoding", code: `b"x".decode("utf-16")`, want: `bytes.decode: unsupported encoding "utf-16"`},
		{name: "unsupported errors", code: `b"x".decode(errors="backslashreplace")`, want: `bytes.decode: unsupported error handler "backslashreplace"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := starlark.ExecFile(&starlark.Thread{Name: "test"}, "test.star", test.code, nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ExecFile() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestNewReturnsNativeBytes(t *testing.T) {
	value := New([]byte("value"))
	be.Equal(t, value.Type(), "bytes")
	be.Equal(t, value, starlark.Bytes("value"))

	decode, err := value.Attr("decode")
	be.Err(t, err, nil)
	decoded, err := starlark.Call(&starlark.Thread{Name: "test"}, decode, nil, nil)
	be.Err(t, err, nil)
	be.Equal(t, decoded, starlark.Value(starlark.String("value")))

	globals, err := starlark.ExecFile(&starlark.Thread{Name: "test"}, "test.star", `
forward = data == b"value"
reverse = b"value" == data
`, starlark.StringDict{"data": value})
	be.Err(t, err, nil)
	be.Equal(t, globals["forward"], starlark.Value(starlark.True))
	be.Equal(t, globals["reverse"], starlark.Value(starlark.True))
}
