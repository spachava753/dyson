package dyson_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson"
)

type recordingHTTPClient struct {
	calls    int
	request  dyson.HTTPRequest
	response dyson.HTTPResponse
	err      error
}

func (c *recordingHTTPClient) Do(_ context.Context, request dyson.HTTPRequest) (dyson.HTTPResponse, error) {
	c.calls++
	c.request = request
	return c.response, c.err
}

func TestRequestsPostIsRecordedAndReplayed(t *testing.T) {
	client := &recordingHTTPClient{response: dyson.HTTPResponse{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       []byte(`{"replayed":true}`),
		URL:        "https://example.test/items?q=one&q=two",
		Reason:     "OK",
		History: []dyson.HTTPResponse{{
			StatusCode: 302,
			Header:     http.Header{"Location": {"https://example.test/items"}},
			URL:        "https://example.test/before",
			Reason:     "Found",
		}},
	}}
	modules := dyson.StdlibModules(dyson.StdlibConfig{HTTPClient: client})
	sphere := dyson.NewSphere(nil, modules, nil, dyson.DefaultCodecRegistry(), true)
	be.Err(t, sphere.Eval(t.Context(), `
load("requests.star", "requests")
response = requests.post(
    "https://example.test/items",
    params={"q": ["one", "two"]},
    json={"enabled": True},
    headers={"X-Test": "value"},
)
`), nil)

	be.Equal(t, client.calls, 1)
	be.Equal(t, client.request.Method, "POST")
	be.Equal(t, client.request.URL, "https://example.test/items?q=one&q=two")
	be.Equal(t, client.request.Header.Get("X-Test"), "value")
	be.Equal(t, client.request.Header.Get("Content-Type"), "application/json")
	if !strings.Contains(string(client.request.Body), `"enabled":true`) {
		t.Fatalf("request body = %q", client.request.Body)
	}

	replayed := dyson.NewSphere(nil, modules, nil, dyson.DefaultCodecRegistry(), true)
	be.Err(t, replayed.Replay(t.Context(), sphere.Log()), nil)
	be.Equal(t, client.calls, 1)
	be.Err(t, replayed.Eval(t.Context(), `
if response.status_code != 200 or response.json() != {"replayed": True}:
    fail("response did not survive replay")
if response.headers["content-type"] != "application/json":
    fail("headers are not normalized")
if response.headers.get("Content-Type") != None:
    fail("headers should be an ordinary case-sensitive dict")
if len(response.history) != 1 or response.history[0].status_code != 302:
    fail("redirect history did not survive replay")
`), nil)
}

func TestRequestsErrorIsReplayedWithoutRepeatingCall(t *testing.T) {
	client := &recordingHTTPClient{err: errors.New("network unavailable")}
	modules := dyson.StdlibModules(dyson.StdlibConfig{HTTPClient: client})
	sphere := dyson.NewSphere(nil, modules, nil, dyson.DefaultCodecRegistry(), true)
	err := sphere.Eval(t.Context(), `
load("requests.star", "requests")
requests.post("https://example.test/items", data="effect")
`)
	if err == nil || !strings.Contains(err.Error(), "network unavailable") {
		t.Fatalf("Eval() error = %v", err)
	}

	replayed := dyson.NewSphere(nil, modules, nil, dyson.DefaultCodecRegistry(), true)
	err = replayed.Replay(t.Context(), sphere.Log())
	if err == nil || !strings.Contains(err.Error(), "network unavailable") {
		t.Fatalf("Replay() error = %v", err)
	}
	be.Equal(t, client.calls, 1)
}
