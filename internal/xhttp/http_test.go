package xhttp

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/nalgeon/be"
)

func TestHostClientStripsAuthorizationOnUnsafeRedirect(t *testing.T) {
	var authorization string
	destination := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		authorization = request.Header.Get("Authorization")
	}))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Redirect(response, request, destination.URL, http.StatusFound)
	}))
	defer source.Close()

	result, err := (HostClient{}).Do(t.Context(), Request{
		Method: "GET", URL: source.URL, Header: http.Header{"authorization": {"Basic secret"}}, AllowRedirects: true,
	})
	be.Err(t, err, nil)
	be.Equal(t, authorization, "")
	be.Equal(t, len(result.History), 1)
}

func TestBufferPreservesWireReason(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "https://example.test", nil)
	be.Err(t, err, nil)
	response, err := buffer(&http.Response{
		Status: "299 Custom Reason", StatusCode: 299, Header: http.Header{},
		Body: io.NopCloser(strings.NewReader("")), Request: request,
	})
	be.Err(t, err, nil)
	be.Equal(t, response.Reason, "Custom Reason")
}

func TestHostClientSendsExplicitHostHeader(t *testing.T) {
	var host string
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		host = request.Host
	}))
	defer server.Close()
	_, err := (HostClient{}).Do(t.Context(), Request{
		Method: http.MethodGet, URL: server.URL, Header: http.Header{"host": {"virtual.example.test"}},
	})
	be.Err(t, err, nil)
	be.Equal(t, host, "virtual.example.test")
}

func TestAuthorizationRedirectRules(t *testing.T) {
	parse := func(value string) *url.URL {
		result, err := url.Parse(value)
		be.Err(t, err, nil)
		return result
	}
	for _, test := range []struct {
		old, next string
		strip     bool
	}{
		{old: "http://example.test", next: "https://example.test", strip: false},
		{old: "https://example.test", next: "https://example.test:443", strip: false},
		{old: "https://example.test", next: "https://sub.example.test", strip: true},
		{old: "https://example.test", next: "https://example.test:444", strip: true},
	} {
		be.Equal(t, shouldStripAuthorization(parse(test.old), parse(test.next)), test.strip)
	}
}

func TestReadTimeoutRefreshesAfterEachRead(t *testing.T) {
	reader, writer := net.Pipe()
	defer reader.Close()
	defer writer.Close()
	go func() {
		for _, value := range []byte("abcd") {
			time.Sleep(20 * time.Millisecond)
			_, _ = writer.Write([]byte{value})
		}
	}()

	connection := &readTimeoutConn{Conn: reader, timeout: 50 * time.Millisecond}
	connection.enabled.Store(true)
	buffer := make([]byte, 4)
	_, err := io.ReadFull(connection, buffer)
	be.Err(t, err, nil)
	be.Equal(t, string(buffer), "abcd")
}

func TestReadTimeoutActivatesThroughTLSAfterHandshake(t *testing.T) {
	reader, writer := net.Pipe()
	defer reader.Close()
	defer writer.Close()
	connection := &readTimeoutConn{Conn: reader, timeout: time.Second}
	findReadTimeoutConn(tls.Client(connection, &tls.Config{})).enable()
	be.Equal(t, connection.enabled.Load(), true)
}

func TestReadTimeoutActivationWakesBlockedRead(t *testing.T) {
	reader, writer := net.Pipe()
	defer reader.Close()
	defer writer.Close()
	connection := &readTimeoutConn{Conn: reader, timeout: 10 * time.Millisecond}
	result := make(chan error, 1)
	go func() {
		_, err := connection.Read(make([]byte, 1))
		result <- err
	}()
	time.Sleep(10 * time.Millisecond)
	connection.enable()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("blocked read completed without a timeout")
		}
	case <-time.After(time.Second):
		t.Fatal("activation did not apply a deadline to the blocked read")
	}
}

func TestHostClientUsesRequestsRedirectMethods(t *testing.T) {
	var method, body, contentType string
	destination := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		method = request.Method
		contentType = request.Header.Get("Content-Type")
		data, _ := io.ReadAll(request.Body)
		body = string(data)
	}))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Location", destination.URL)
		response.WriteHeader(http.StatusMovedPermanently)
	}))
	defer source.Close()

	_, err := (HostClient{}).Do(t.Context(), Request{
		Method: "DELETE", URL: source.URL, Header: http.Header{"Content-Type": {"text/plain"}}, Body: []byte("discard"), AllowRedirects: true,
	})
	be.Err(t, err, nil)
	be.Equal(t, method, "DELETE")
	be.Equal(t, body, "")
	be.Equal(t, contentType, "")
}
