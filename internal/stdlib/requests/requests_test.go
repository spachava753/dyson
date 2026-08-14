package requests

import (
	"context"
	"encoding/base64"
	"net/http"
	"testing"
	"time"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson/internal/xhttp"
	"github.com/spachava753/starlarkx/starlark"
)

type recordingClient struct {
	request xhttp.Request
}

func (c *recordingClient) Do(_ context.Context, request xhttp.Request) (xhttp.Response, error) {
	c.request = request
	return xhttp.Response{StatusCode: 200, Header: http.Header{}, URL: request.URL, Reason: "OK"}, nil
}

func TestRequestPreparation(t *testing.T) {
	client := new(recordingClient)
	module := MakeModule(client)
	thread := &starlark.Thread{Name: "requests-test"}
	request := module.Members["request"]

	_, err := starlark.Call(thread, request, starlark.Tuple{
		starlark.String("get"), starlark.String("  https://example.test/items "),
	}, nil)
	be.Err(t, err, nil)
	be.Equal(t, client.request.URL, "https://example.test/items%20")

	_, err = starlark.Call(thread, request, starlark.Tuple{
		starlark.String("get"), starlark.String("https://example.test"),
	}, nil)
	be.Err(t, err, nil)
	be.Equal(t, client.request.URL, "https://example.test/")

	_, err = starlark.Call(thread, request, starlark.Tuple{
		starlark.String("post"), starlark.String("https://example.test/items"),
	}, []starlark.Tuple{{starlark.String("auth"), starlark.Tuple{starlark.String("café"), starlark.String("päss")}}})
	be.Err(t, err, nil)
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte{'c', 'a', 'f', 0xe9, ':', 'p', 0xe4, 's', 's'})
	be.Equal(t, client.request.Header.Get("Authorization"), wantAuth)
	be.Equal(t, client.request.Header.Get("Content-Length"), "0")

	_, err = starlark.Call(thread, request, starlark.Tuple{
		starlark.String("get"), starlark.String("https://user:pass@example.test/items"),
	}, nil)
	be.Err(t, err, nil)
	be.Equal(t, client.request.URL, "https://example.test/items")
	be.Equal(t, client.request.Header.Get("Authorization"), "Basic dXNlcjpwYXNz")

	_, err = starlark.Call(thread, request, starlark.Tuple{
		starlark.String("get"), starlark.String("https://user:pass@example.test/items"),
	}, []starlark.Tuple{{starlark.String("auth"), starlark.Tuple{}}})
	be.Err(t, err, nil)
	be.Equal(t, client.request.Header.Get("Authorization"), "")

	_, err = starlark.Call(thread, module.Members["head"], starlark.Tuple{starlark.String("https://example.test")}, nil)
	be.Err(t, err, nil)
	be.Equal(t, client.request.Method, "HEAD")
	be.Equal(t, client.request.AllowRedirects, false)
	be.Equal(t, client.request.Header.Get("Content-Length"), "")
}

func TestResponseTextEncoding(t *testing.T) {
	for _, test := range []struct {
		name        string
		contentType string
		body        []byte
		encoding    string
		text        string
	}{
		{name: "text defaults to latin1", contentType: "text/plain", body: []byte{0xe9}, encoding: "iso-8859-1", text: "é"},
		{name: "json utf8 bom", contentType: "application/json", body: append([]byte{0xef, 0xbb, 0xbf}, []byte(`{"x":1}`)...), encoding: "utf-8-sig", text: `{"x":1}`},
		{name: "json utf16 bom", contentType: "application/json", body: []byte{0xff, 0xfe, '{', 0, '}', 0}, encoding: "utf-16-le", text: `{}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := newResponseValue(xhttp.Response{StatusCode: 200, Header: http.Header{"Content-Type": {test.contentType}}, Body: test.body})
			encoding, err := response.Attr("encoding")
			be.Err(t, err, nil)
			be.Equal(t, string(encoding.(starlark.String)), test.encoding)
			text, err := response.Attr("text")
			be.Err(t, err, nil)
			be.Equal(t, string(text.(starlark.String)), test.text)
		})
	}
}

func TestTimeoutRejectsUnrepresentableDurations(t *testing.T) {
	for _, seconds := range []float64{0.5e-9, 1e20} {
		_, err := timeout(starlark.Float(seconds))
		if err == nil {
			t.Fatalf("timeout(%g) succeeded", seconds)
		}
	}
	duration, err := timeout(starlark.Float(1e-9))
	be.Err(t, err, nil)
	be.Equal(t, duration, time.Nanosecond)
}
