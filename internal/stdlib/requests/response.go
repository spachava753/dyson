package requests

import (
	"encoding/binary"
	"fmt"
	"mime"
	"net/http"
	"slices"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/spachava753/dyson/internal/xhttp"
	starlarkjson "github.com/spachava753/starlarkx/lib/json"
	"github.com/spachava753/starlarkx/starlark"
)

const responseTypeName = "requests.Response"

var responseMethods = map[string]*starlark.Builtin{
	"close":            starlark.NewBuiltin("requests.Response.close", responseClose),
	"json":             starlark.NewBuiltin("requests.Response.json", responseJSON),
	"raise_for_status": starlark.NewBuiltin("requests.Response.raise_for_status", responseRaiseForStatus),
}

type responseValue struct {
	statusCode int
	headers    *starlark.Dict
	content    starlark.Bytes
	url        string
	reason     string
	encoding   starlark.Value
	history    *starlark.List
	frozen     bool
}

func newResponseValue(response xhttp.Response) *responseValue {
	history := make([]starlark.Value, len(response.History))
	for i, previous := range response.History {
		prefix := slices.Clone(history[:i])
		history[i] = responseValueFromHTTP(previous, starlark.NewList(prefix))
	}
	return responseValueFromHTTP(response, starlark.NewList(history))
}

func responseValueFromHTTP(response xhttp.Response, history *starlark.List) *responseValue {
	headers := starlark.NewDict(len(response.Header))
	for name, values := range response.Header {
		_ = headers.SetKey(starlark.String(strings.ToLower(name)), starlark.String(strings.Join(values, ", ")))
	}
	value := &responseValue{
		statusCode: response.StatusCode,
		headers:    headers,
		content:    starlark.Bytes(string(response.Body)),
		url:        response.URL,
		reason:     response.Reason,
		encoding:   starlark.None,
		history:    history,
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "" {
		if mediaType, parameters, err := mime.ParseMediaType(contentType); err == nil {
			if charset := parameters["charset"]; charset != "" {
				value.encoding = starlark.String(charset)
			} else if mediaType == "application/json" || strings.HasSuffix(mediaType, "+json") {
				value.encoding = starlark.String(jsonEncoding(response.Body))
			} else if strings.HasPrefix(mediaType, "text/") {
				value.encoding = starlark.String("iso-8859-1")
			}
		}
	}
	return value
}

// String returns the Python-style response representation.
func (r *responseValue) String() string { return fmt.Sprintf("<Response [%d]>", r.statusCode) }

// Type returns the Starlark type name for responses.
func (r *responseValue) Type() string { return responseTypeName }

// Freeze recursively freezes response collections and prevents encoding changes.
func (r *responseValue) Freeze() {
	r.headers.Freeze()
	r.history.Freeze()
	r.frozen = true
}

// Truth follows requests semantics: responses below 400 or at least 600 are true.
func (r *responseValue) Truth() starlark.Bool {
	return starlark.Bool(r.statusCode < http.StatusBadRequest || r.statusCode >= 600)
}

// Hash reports that responses are not hashable.
func (r *responseValue) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable: %s", r.Type())
}

// Attr returns a response field or bound method, or nil for an unknown attribute.
func (r *responseValue) Attr(name string) (starlark.Value, error) {
	switch name {
	case "content":
		return r.content, nil
	case "encoding":
		return r.encoding, nil
	case "headers":
		return r.headers, nil
	case "history":
		return r.history, nil
	case "is_redirect":
		_, hasLocation, _ := r.headers.Get(starlark.String("location"))
		return starlark.Bool(slices.Contains([]int{301, 302, 303, 307, 308}, r.statusCode) && hasLocation), nil
	case "ok":
		return r.Truth(), nil
	case "reason":
		return starlark.String(r.reason), nil
	case "status_code":
		return starlark.MakeInt(r.statusCode), nil
	case "text":
		return starlark.String(r.text()), nil
	case "url":
		return starlark.String(r.url), nil
	}
	if method, ok := responseMethods[name]; ok {
		return method.BindReceiver(r), nil
	}
	return nil, nil
}

// AttrNames returns the fields and methods exposed by a response.
func (r *responseValue) AttrNames() []string {
	names := []string{"content", "encoding", "headers", "history", "is_redirect", "ok", "reason", "status_code", "text", "url"}
	for name := range responseMethods {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// SetField updates the mutable encoding override and rejects all other assignments.
func (r *responseValue) SetField(name string, value starlark.Value) error {
	if name != "encoding" {
		return starlark.NoSuchAttrError(fmt.Sprintf("%s has no writable attribute %q", r.Type(), name))
	}
	if r.frozen {
		return fmt.Errorf("cannot assign to encoding field of frozen %s", r.Type())
	}
	if value != starlark.None {
		if _, ok := starlark.AsString(value); !ok {
			return fmt.Errorf("requests.Response.encoding: got %s, want string or None", value.Type())
		}
	}
	r.encoding = value
	return nil
}

// text decodes buffered response content using the mutable encoding selection.
// Invalid input is replaced, matching requests' user-facing text behavior.
func (r *responseValue) text() string {
	content := []byte(r.content)
	encoding, _ := starlark.AsString(r.encoding)
	switch normalizeEncoding(encoding) {
	case "iso-8859-1", "latin-1", "latin1":
		runes := make([]rune, len(content))
		for i, value := range content {
			runes[i] = rune(value)
		}
		return string(runes)
	case "ascii", "us-ascii":
		runes := make([]rune, len(content))
		for i, value := range content {
			if value > 0x7f {
				runes[i] = utf8.RuneError
			} else {
				runes[i] = rune(value)
			}
		}
		return string(runes)
	case "utf-8-sig":
		content = trimPrefix(content, []byte{0xef, 0xbb, 0xbf})
		return strings.ToValidUTF8(string(content), string(utf8.RuneError))
	case "utf-16", "utf-16-le", "utf16", "utf16le":
		return decodeUTF16(content, binary.LittleEndian)
	case "utf-16-be", "utf16be":
		return decodeUTF16(content, binary.BigEndian)
	case "utf-32", "utf-32-le", "utf32", "utf32le":
		return decodeUTF32(content, binary.LittleEndian)
	case "utf-32-be", "utf32be":
		return decodeUTF32(content, binary.BigEndian)
	default:
		return strings.ToValidUTF8(string(content), string(utf8.RuneError))
	}
}

func normalizeEncoding(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "_", "-"))
}

// jsonEncoding infers JSON Unicode encoding from a byte-order mark and defaults
// to UTF-8 when no supported mark is present.
func jsonEncoding(content []byte) string {
	switch {
	case len(content) >= 4 && string(content[:4]) == "\x00\x00\xfe\xff":
		return "utf-32-be"
	case len(content) >= 4 && string(content[:4]) == "\xff\xfe\x00\x00":
		return "utf-32-le"
	case len(content) >= 3 && string(content[:3]) == "\xef\xbb\xbf":
		return "utf-8-sig"
	case len(content) >= 2 && string(content[:2]) == "\xfe\xff":
		return "utf-16-be"
	case len(content) >= 2 && string(content[:2]) == "\xff\xfe":
		return "utf-16-le"
	default:
		return "utf-8"
	}
}

func decodeUTF16(content []byte, order binary.ByteOrder) string {
	if len(content) >= 2 {
		switch string(content[:2]) {
		case "\xfe\xff":
			order, content = binary.BigEndian, content[2:]
		case "\xff\xfe":
			order, content = binary.LittleEndian, content[2:]
		}
	}
	units := make([]uint16, len(content)/2)
	for i := range units {
		units[i] = order.Uint16(content[i*2:])
	}
	result := utf16.Decode(units)
	if len(content)%2 != 0 {
		result = append(result, utf8.RuneError)
	}
	return string(result)
}

func decodeUTF32(content []byte, order binary.ByteOrder) string {
	if len(content) >= 4 {
		switch string(content[:4]) {
		case "\x00\x00\xfe\xff":
			order, content = binary.BigEndian, content[4:]
		case "\xff\xfe\x00\x00":
			order, content = binary.LittleEndian, content[4:]
		}
	}
	runes := make([]rune, 0, (len(content)+3)/4)
	for len(content) >= 4 {
		character := rune(order.Uint32(content))
		if !utf8.ValidRune(character) {
			character = utf8.RuneError
		}
		runes = append(runes, character)
		content = content[4:]
	}
	if len(content) != 0 {
		runes = append(runes, utf8.RuneError)
	}
	return string(runes)
}

func trimPrefix(content, prefix []byte) []byte {
	if len(content) < len(prefix) || string(content[:len(prefix)]) != string(prefix) {
		return content
	}
	return content[len(prefix):]
}

func responseClose(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if _, err := responseReceiver(fn); err != nil {
		return nil, err
	}
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	return starlark.None, nil
}

func responseJSON(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	response, err := responseReceiver(fn)
	if err != nil {
		return nil, err
	}
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	return starlark.Call(thread, starlarkjson.Module.Members["decode"], starlark.Tuple{starlark.String(response.text())}, nil)
}

func responseRaiseForStatus(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	response, err := responseReceiver(fn)
	if err != nil {
		return nil, err
	}
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	if response.statusCode >= 400 && response.statusCode < 600 {
		category := "Client"
		if response.statusCode >= 500 {
			category = "Server"
		}
		return nil, fmt.Errorf("%d %s Error: %s for url: %s", response.statusCode, category, response.reason, response.url)
	}
	return starlark.None, nil
}

func responseReceiver(fn *starlark.Builtin) (*responseValue, error) {
	response, ok := fn.Receiver().(*responseValue)
	if !ok {
		return nil, fmt.Errorf("%s: receiver is %T, want %s", fn.Name(), fn.Receiver(), responseTypeName)
	}
	return response, nil
}

var _ starlark.HasSetField = (*responseValue)(nil)
