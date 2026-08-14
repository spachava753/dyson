package builtins

import (
	"testing"

	"github.com/nalgeon/be"
)

func TestTextDecoderRetainsIncompleteUTF8AcrossChunks(t *testing.T) {
	decoder, err := newTextDecoder("utf-8", "strict")
	be.Err(t, err, nil)

	text, err := decoder.decode([]byte{0xe2}, false)
	be.Err(t, err, nil)
	be.Equal(t, text, "")
	text, err = decoder.decode([]byte{0x82, 0xac}, false)
	be.Err(t, err, nil)
	be.Equal(t, text, "\u20ac")
	text, err = decoder.decode(nil, true)
	be.Err(t, err, nil)
	be.Equal(t, text, "")
}

func TestTextDecoderReplacesIncompleteFinalUTF8Once(t *testing.T) {
	decoder, err := newTextDecoder("utf-8", "replace")
	be.Err(t, err, nil)

	text, err := decoder.decode([]byte{0xe2}, false)
	be.Err(t, err, nil)
	be.Equal(t, text, "")
	text, err = decoder.decode([]byte{0x82}, true)
	be.Err(t, err, nil)
	be.Equal(t, text, "\ufffd")
}

func TestTextDecoderConsumesMalformedUTF8LikePython(t *testing.T) {
	for _, test := range []struct {
		name  string
		input []byte
		want  string
	}{
		{name: "three-byte prefix", input: []byte{0xe1, 0x80, 'A'}, want: "\ufffdA"},
		{name: "four-byte prefix", input: []byte{0xf0, 0x90, 0x80, 'A'}, want: "\ufffdA"},
		{name: "overlong sequence", input: []byte{0xe0, 0x80, 0x80}, want: "\ufffd\ufffd\ufffd"},
		{name: "surrogate sequence", input: []byte{0xed, 0xa0, 0x80}, want: "\ufffd\ufffd\ufffd"},
	} {
		t.Run(test.name, func(t *testing.T) {
			decoder, err := newTextDecoder("utf-8", "replace")
			be.Err(t, err, nil)
			text, err := decoder.decode(test.input, true)
			be.Err(t, err, nil)
			be.Equal(t, text, test.want)
		})
	}
}

func TestTextDecoderConsumesMalformedUTF8PrefixAcrossChunks(t *testing.T) {
	decoder, err := newTextDecoder("utf-8", "replace")
	be.Err(t, err, nil)

	for _, chunk := range [][]byte{{0xe1}, {0x80}} {
		text, err := decoder.decode(chunk, false)
		be.Err(t, err, nil)
		be.Equal(t, text, "")
	}
	text, err := decoder.decode([]byte{'A'}, false)
	be.Err(t, err, nil)
	be.Equal(t, text, "\ufffdA")
}
