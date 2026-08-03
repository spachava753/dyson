// Package xhttp defines the HTTP capability exposed to Starlark modules.
package xhttp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/publicsuffix"
)

// Request is the buffered request passed to a Client.
type Request struct {
	Method         string
	URL            string
	Header         http.Header
	Body           []byte
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	AllowRedirects bool
}

// Response is a buffered HTTP response. History is ordered oldest to newest.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	URL        string
	Reason     string
	History    []Response
}

// Client performs an HTTP request and honors context cancellation.
type Client interface {
	Do(context.Context, Request) (Response, error)
}

// HostClient performs requests with the host network stack.
type HostClient struct{}

const maxBodyBytes int64 = 64 << 20

// BodyLimitError reports that a buffered response was too large.
type BodyLimitError struct {
	Limit int64
}

// Error returns a message containing the configured response-body limit.
func (e *BodyLimitError) Error() string {
	return fmt.Sprintf("xhttp: body exceeds limit of %d bytes", e.Limit)
}

// Do sends request, follows at most 30 redirects, and buffers each response.
func (HostClient) Do(ctx context.Context, request Request) (Response, error) {
	req, err := http.NewRequestWithContext(ctx, request.Method, request.URL, bytes.NewReader(request.Body))
	if err != nil {
		return Response{}, err
	}
	req.Header = request.Header.Clone()
	for name, values := range req.Header {
		if strings.EqualFold(name, "Host") {
			if len(values) != 0 {
				req.Host = values[0]
			}
			delete(req.Header, name)
		}
	}
	if request.ReadTimeout > 0 {
		var timeoutConnection atomic.Pointer[readTimeoutConn]
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{
			GotConn: func(info httptrace.GotConnInfo) {
				connection := findReadTimeoutConn(info.Conn)
				if connection != nil {
					connection.enabled.Store(false)
					_ = connection.SetReadDeadline(time.Time{})
				}
				timeoutConnection.Store(connection)
			},
			WroteRequest: func(httptrace.WroteRequestInfo) {
				if connection := timeoutConnection.Load(); connection != nil {
					connection.enable()
				}
			},
		}))
	}

	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		transport = &http.Transport{}
	}
	transport = transport.Clone()
	if request.ConnectTimeout > 0 || request.ReadTimeout > 0 {
		dialer := &net.Dialer{Timeout: request.ConnectTimeout, KeepAlive: 30 * time.Second}
		transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			connection, err := dialer.DialContext(ctx, network, address)
			if err != nil || request.ReadTimeout <= 0 {
				return connection, err
			}
			return &readTimeoutConn{Conn: connection, timeout: request.ReadTimeout}, nil
		}
	}
	transport.TLSHandshakeTimeout = request.ConnectTimeout
	defer transport.CloseIdleConnections()

	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return Response{}, err
	}
	client := &http.Client{Transport: transport, Jar: jar}

	var history []Response
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if !request.AllowRedirects {
			return http.ErrUseLastResponse
		}
		if len(via) > 30 {
			return fmt.Errorf("requests: exceeded 30 redirects")
		}
		priorRequest := via[len(via)-1]
		if next.Response.StatusCode == http.StatusMovedPermanently && priorRequest.Method != http.MethodPost {
			next.Method = priorRequest.Method
		}
		if next.Response.StatusCode == http.StatusMovedPermanently ||
			next.Response.StatusCode == http.StatusFound ||
			next.Response.StatusCode == http.StatusSeeOther {
			next.Body = nil
			next.GetBody = nil
			next.ContentLength = 0
			next.TransferEncoding = nil
			deleteHeaders(next.Header, "Content-Length", "Content-Type", "Transfer-Encoding")
		}
		if len(via) != 0 && shouldStripAuthorization(via[len(via)-1].URL, next.URL) {
			deleteHeaders(next.Header, "Authorization")
		}
		// Redirect URL credentials must not bypass the prepared-request auth policy.
		next.URL.User = nil
		previousResponse, err := buffer(next.Response)
		if err != nil {
			return err
		}
		history = append(history, previousResponse)
		return nil
	}

	response, err := client.Do(req)
	if err != nil {
		return Response{}, err
	}
	result, err := buffer(response)
	if err != nil {
		return Response{}, err
	}
	result.History = history
	return result, nil
}

type readTimeoutConn struct {
	net.Conn
	timeout time.Duration
	enabled atomic.Bool
}

// Read applies the configured read deadline after timeout enforcement is enabled.
func (c *readTimeoutConn) Read(buffer []byte) (int, error) {
	if c.enabled.Load() {
		if err := c.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
			return 0, err
		}
	}
	return c.Conn.Read(buffer)
}

func (c *readTimeoutConn) enable() {
	c.enabled.Store(true)
	_ = c.SetReadDeadline(time.Now().Add(c.timeout))
}

func findReadTimeoutConn(connection net.Conn) *readTimeoutConn {
	for {
		if timed, ok := connection.(*readTimeoutConn); ok {
			return timed
		}
		wrapper, ok := connection.(interface{ NetConn() net.Conn })
		if !ok {
			return nil
		}
		connection = wrapper.NetConn()
	}
}

func shouldStripAuthorization(oldURL, newURL *url.URL) bool {
	if !strings.EqualFold(oldURL.Hostname(), newURL.Hostname()) {
		return true
	}
	oldScheme, newScheme := strings.ToLower(oldURL.Scheme), strings.ToLower(newURL.Scheme)
	if oldScheme == "http" && newScheme == "https" && defaultPort(oldURL) == "80" && defaultPort(newURL) == "443" {
		return false
	}
	return oldScheme != newScheme || defaultPort(oldURL) != defaultPort(newURL)
}

func defaultPort(target *url.URL) string {
	if port := target.Port(); port != "" {
		return port
	}
	switch strings.ToLower(target.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func deleteHeaders(header http.Header, names ...string) {
	for key := range header {
		for _, name := range names {
			if strings.EqualFold(key, name) {
				delete(header, key)
				break
			}
		}
	}
}

func buffer(response *http.Response) (Response, error) {
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
	if err != nil {
		return Response{}, err
	}
	if int64(len(body)) > maxBodyBytes {
		return Response{}, &BodyLimitError{Limit: maxBodyBytes}
	}
	reason, found := strings.CutPrefix(response.Status, strconv.Itoa(response.StatusCode)+" ")
	if !found {
		reason = http.StatusText(response.StatusCode)
	}
	return Response{
		StatusCode: response.StatusCode,
		Header:     response.Header.Clone(),
		Body:       body,
		URL:        response.Request.URL.String(),
		Reason:     reason,
	}, nil
}
