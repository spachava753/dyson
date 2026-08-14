package requests

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/spachava753/dyson/internal/xctx"
	"github.com/spachava753/dyson/internal/xhttp"
	starlarkjson "github.com/spachava753/starlarkx/lib/json"
	"github.com/spachava753/starlarkx/starlark"
)

// requestBuiltin returns the requests.request implementation. It validates and
// normalizes the Python-compatible arguments, builds a buffered capability
// request, and delegates it with the active evaluation context.
func requestBuiltin(client xhttp.Client) func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if len(args) > 2 {
			return nil, fmt.Errorf("%s: accepts 2 positional arguments (%d given)", fn.Name(), len(args))
		}
		var methodValue, urlValue starlark.Value
		var params, data, headers, cookies, files, auth, timeout starlark.Value = starlark.None, starlark.None, starlark.None, starlark.None, starlark.None, starlark.None, starlark.None
		var proxies, hooks, stream, verify, cert, jsonValue starlark.Value = starlark.None, starlark.None, starlark.None, starlark.None, starlark.None, starlark.None
		allowRedirects := true
		if err := starlark.UnpackArgs(
			fn.Name(), args, kwargs,
			"method", &methodValue,
			"url", &urlValue,
			"params?", &params,
			"data?", &data,
			"headers?", &headers,
			"cookies?", &cookies,
			"files?", &files,
			"auth?", &auth,
			"timeout?", &timeout,
			"allow_redirects?", &allowRedirects,
			"proxies?", &proxies,
			"hooks?", &hooks,
			"stream?", &stream,
			"verify?", &verify,
			"cert?", &cert,
			"json?", &jsonValue,
		); err != nil {
			return nil, err
		}
		if err := rejectUnsupported(fn.Name(), files, proxies, hooks, stream, verify, cert); err != nil {
			return nil, err
		}

		method, err := stringOrBytes(methodValue)
		if err != nil {
			return nil, fmt.Errorf("%s: method must be a string or bytes", fn.Name())
		}
		method = strings.ToUpper(method)
		rawURL, err := stringOrBytes(urlValue)
		if err != nil || !utf8.ValidString(rawURL) {
			return nil, fmt.Errorf("%s: URL must be a UTF-8 string or bytes", fn.Name())
		}
		parsed, err := url.Parse(strings.TrimLeftFunc(rawURL, unicode.IsSpace))
		if err != nil || parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
			return nil, fmt.Errorf("%s: invalid HTTP URL %q", fn.Name(), rawURL)
		}
		if parsed.Path == "" {
			parsed.Path = "/"
		}
		var urlAuth starlark.Value = starlark.None
		if parsed.User != nil {
			username := parsed.User.Username()
			password, _ := parsed.User.Password()
			if username != "" || password != "" {
				urlAuth = starlark.Tuple{starlark.String(username), starlark.String(password)}
			}
			parsed.User = nil
		}
		if params != starlark.None && bool(params.Truth()) {
			query, err := encodeParams(params)
			if err != nil {
				return nil, fmt.Errorf("%s: params: %w", fn.Name(), err)
			}
			if parsed.RawQuery == "" {
				parsed.RawQuery = query
			} else if query != "" {
				parsed.RawQuery += "&" + query
			}
		}

		header := http.Header{
			"User-Agent": {"python-requests/2.34.2"},
			"Accept":     {"*/*"},
			"Connection": {"keep-alive"},
		}
		if err := applyHeaders(header, headers); err != nil {
			return nil, fmt.Errorf("%s: headers: %w", fn.Name(), err)
		}
		if header.Get("Cookie") == "" {
			cookie, err := cookieHeader(cookies)
			if err != nil {
				return nil, fmt.Errorf("%s: cookies: %w", fn.Name(), err)
			}
			if cookie != "" {
				header.Set("Cookie", cookie)
			}
		}
		effectiveAuth := auth
		if effectiveAuth == starlark.None {
			effectiveAuth = urlAuth
		}
		if effectiveAuth != starlark.None && bool(effectiveAuth.Truth()) {
			if err := applyBasicAuth(header, effectiveAuth); err != nil {
				return nil, fmt.Errorf("%s: auth: %w", fn.Name(), err)
			}
		}

		body, err := requestBody(thread, data, jsonValue)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fn.Name(), err)
		}
		if body.kind != "" && header.Get("Content-Type") == "" {
			header.Set("Content-Type", body.kind)
		}
		if len(body.data) > 0 {
			header.Set("Content-Length", strconv.Itoa(len(body.data)))
		} else if method != http.MethodGet && method != http.MethodHead {
			if _, supplied := header["Content-Length"]; !supplied {
				header.Set("Content-Length", "0")
			}
		}

		connectTimeout, readTimeout, err := timeouts(timeout)
		if err != nil {
			return nil, fmt.Errorf("%s: timeout: %w", fn.Name(), err)
		}
		if client == nil {
			return nil, fmt.Errorf("%s: HTTP requests are not configured", fn.Name())
		}
		ctx := xctx.FromLocal(thread)
		if ctx == nil {
			ctx = context.Background()
		}
		response, err := client.Do(ctx, xhttp.Request{
			Method: method, URL: parsed.String(), Header: header, Body: body.data,
			ConnectTimeout: connectTimeout, ReadTimeout: readTimeout, AllowRedirects: allowRedirects,
		})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fn.Name(), err)
		}
		return newResponseValue(response), nil
	}
}

// rejectUnsupported fails closed for requests options Dyson cannot honor. False
// stream and true verify values are accepted because they preserve supported
// buffered-response and default-TLS behavior.
func rejectUnsupported(fn string, files, proxies, hooks, stream, verify, cert starlark.Value) error {
	if files != starlark.None {
		return fmt.Errorf("%s: files is not supported", fn)
	}
	if proxies != starlark.None {
		return fmt.Errorf("%s: proxies is not supported", fn)
	}
	if hooks != starlark.None {
		return fmt.Errorf("%s: hooks is not supported", fn)
	}
	if stream != starlark.None && bool(stream.Truth()) {
		return fmt.Errorf("%s: stream=True is not supported", fn)
	}
	if verify != starlark.None {
		enabled, ok := verify.(starlark.Bool)
		if !ok || !bool(enabled) {
			return fmt.Errorf("%s: custom TLS verification is not supported", fn)
		}
	}
	if cert != starlark.None {
		return fmt.Errorf("%s: cert is not supported", fn)
	}
	return nil
}

func stringOrBytes(value starlark.Value) (string, error) {
	switch value := value.(type) {
	case starlark.String:
		return string(value), nil
	case starlark.Bytes:
		return string(value), nil
	default:
		return "", fmt.Errorf("got %s", value.Type())
	}
}

func applyHeaders(header http.Header, value starlark.Value) error {
	if value == starlark.None {
		return nil
	}
	mapping, ok := value.(starlark.IterableMapping)
	if !ok {
		return fmt.Errorf("must be a mapping")
	}
	return eachMapping(mapping, func(key, value starlark.Value) error {
		name, err := stringOrBytes(key)
		if err != nil {
			return fmt.Errorf("name must be a string or bytes")
		}
		if value == starlark.None {
			header.Del(name)
			return nil
		}
		text, err := stringOrBytes(value)
		if err != nil {
			return fmt.Errorf("%s value must be a string, bytes, or None", name)
		}
		header.Set(name, text)
		return nil
	})
}

func cookieHeader(value starlark.Value) (string, error) {
	if value == starlark.None {
		return "", nil
	}
	mapping, ok := value.(starlark.IterableMapping)
	if !ok {
		return "", fmt.Errorf("must be a mapping")
	}
	var parts []string
	err := eachMapping(mapping, func(key, value starlark.Value) error {
		name, ok := scalarText(key)
		if !ok {
			return fmt.Errorf("name must be scalar")
		}
		if value == starlark.None {
			parts = append(parts, name)
			return nil
		}
		text, ok := scalarText(value)
		if !ok {
			return fmt.Errorf("%s value must be scalar", name)
		}
		parts = append(parts, name+"="+text)
		return nil
	})
	return strings.Join(parts, "; "), err
}

func applyBasicAuth(header http.Header, value starlark.Value) error {
	pair, ok := value.(starlark.Indexable)
	if !ok || pair.Len() != 2 {
		return fmt.Errorf("must be a two-item sequence")
	}
	username, err := basicAuthPart(pair.Index(0))
	if err != nil {
		return fmt.Errorf("username must be scalar")
	}
	password, err := basicAuthPart(pair.Index(1))
	if err != nil {
		return fmt.Errorf("password must be scalar")
	}
	credentials := make([]byte, 0, len(username)+1+len(password))
	credentials = append(credentials, username...)
	credentials = append(credentials, ':')
	credentials = append(credentials, password...)
	header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString(credentials))
	return nil
}

func basicAuthPart(value starlark.Value) ([]byte, error) {
	if bytes, ok := value.(starlark.Bytes); ok {
		return []byte(bytes), nil
	}
	text, ok := scalarText(value)
	if !ok {
		return nil, fmt.Errorf("not scalar")
	}
	encoded := make([]byte, 0, len(text))
	for _, character := range text {
		if character > 0xff {
			return nil, fmt.Errorf("not Latin-1")
		}
		encoded = append(encoded, byte(character))
	}
	return encoded, nil
}

type preparedBody struct {
	data []byte
	kind string
}

func requestBody(thread *starlark.Thread, data, jsonValue starlark.Value) (preparedBody, error) {
	if data != starlark.None && bool(data.Truth()) {
		if raw, err := stringOrBytes(data); err == nil {
			return preparedBody{data: []byte(raw)}, nil
		}
		encoded, err := encodeParams(data)
		if err != nil {
			return preparedBody{}, fmt.Errorf("data: %w", err)
		}
		return preparedBody{data: []byte(encoded), kind: "application/x-www-form-urlencoded"}, nil
	}
	if jsonValue != starlark.None {
		encoded, err := starlark.Call(thread, starlarkjson.Module.Members["encode"], starlark.Tuple{jsonValue}, nil)
		if err != nil {
			return preparedBody{}, fmt.Errorf("json: %w", err)
		}
		text, _ := starlark.AsString(encoded)
		return preparedBody{data: []byte(text), kind: "application/json"}, nil
	}
	return preparedBody{}, nil
}

// encodeParams converts requests-style parameters into form/query encoding.
// Strings and bytes pass through unchanged; mappings and pair sequences expand
// non-string sequence values, omit None values, and percent-encode scalars.
func encodeParams(value starlark.Value) (string, error) {
	if raw, err := stringOrBytes(value); err == nil {
		return raw, nil
	}
	var pairs [][2]string
	appendValue := func(key string, value starlark.Value) error {
		if value == starlark.None {
			return nil
		}
		if sequence, ok := value.(starlark.Indexable); ok {
			if _, stringLike := value.(starlark.String); !stringLike {
				if _, bytesLike := value.(starlark.Bytes); !bytesLike {
					for i := range sequence.Len() {
						if err := appendValueScalar(&pairs, key, sequence.Index(i)); err != nil {
							return err
						}
					}
					return nil
				}
			}
		}
		return appendValueScalar(&pairs, key, value)
	}
	if mapping, ok := value.(starlark.IterableMapping); ok {
		err := eachMapping(mapping, func(key, value starlark.Value) error {
			text, ok := scalarText(key)
			if !ok {
				return fmt.Errorf("key must be scalar")
			}
			return appendValue(text, value)
		})
		if err != nil {
			return "", err
		}
	} else if sequence, ok := value.(starlark.Indexable); ok {
		for i := range sequence.Len() {
			pair, ok := sequence.Index(i).(starlark.Indexable)
			if !ok || pair.Len() != 2 {
				return "", fmt.Errorf("item %d must be a pair", i)
			}
			key, ok := scalarText(pair.Index(0))
			if !ok {
				return "", fmt.Errorf("item %d key must be scalar", i)
			}
			if err := appendValue(key, pair.Index(1)); err != nil {
				return "", err
			}
		}
	} else {
		return "", fmt.Errorf("must be a mapping, sequence of pairs, string, or bytes")
	}
	encoded := make([]string, len(pairs))
	for i, pair := range pairs {
		encoded[i] = url.QueryEscape(pair[0]) + "=" + url.QueryEscape(pair[1])
	}
	return strings.Join(encoded, "&"), nil
}

func appendValueScalar(pairs *[][2]string, key string, value starlark.Value) error {
	if value == starlark.None {
		return nil
	}
	text, ok := scalarText(value)
	if !ok {
		return fmt.Errorf("value for %s must be scalar", key)
	}
	*pairs = append(*pairs, [2]string{key, text})
	return nil
}

func eachMapping(mapping starlark.IterableMapping, visit func(starlark.Value, starlark.Value) error) error {
	iterator := mapping.Iterate()
	defer iterator.Done()
	var key starlark.Value
	for iterator.Next(&key) {
		value, found, err := mapping.Get(key)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		if err := visit(key, value); err != nil {
			return err
		}
	}
	return nil
}

func scalarText(value starlark.Value) (string, bool) {
	if text, err := stringOrBytes(value); err == nil {
		return text, true
	}
	switch value := value.(type) {
	case starlark.Bool, starlark.Int, starlark.Float, starlark.NoneType:
		return value.String(), true
	default:
		return "", false
	}
}

func timeouts(value starlark.Value) (time.Duration, time.Duration, error) {
	if value == starlark.None {
		return 0, 0, nil
	}
	if pair, ok := value.(starlark.Indexable); ok {
		if _, stringLike := value.(starlark.String); !stringLike {
			if pair.Len() != 2 {
				return 0, 0, fmt.Errorf("pair must contain connect and read values")
			}
			connect, err := timeout(pair.Index(0))
			if err != nil {
				return 0, 0, err
			}
			read, err := timeout(pair.Index(1))
			return connect, read, err
		}
	}
	duration, err := timeout(value)
	return duration, duration, err
}

func timeout(value starlark.Value) (time.Duration, error) {
	if value == starlark.None {
		return 0, nil
	}
	seconds, ok := starlark.AsFloat(value)
	if !ok || seconds <= 0 || math.IsInf(seconds, 0) || math.IsNaN(seconds) {
		return 0, fmt.Errorf("must be a positive finite number or None")
	}
	nanoseconds := seconds * float64(time.Second)
	if nanoseconds < 1 || nanoseconds >= float64(uint64(1)<<63) {
		return 0, fmt.Errorf("must fit in a positive time.Duration")
	}
	return time.Duration(nanoseconds), nil
}
