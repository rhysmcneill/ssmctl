// Package middleware is to redact sensitive headers from the aws ClientLogMode logs. This happens in the transport layer when the sensitive headers will be present.
package middleware

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

var sensitiveHeaders = map[string]bool{
	"authorization":        true,
	"x-amz-security-token": true,
}

// RedactingTransport intercepts HTTP requests to redact sensitive headers.
type RedactingTransport struct {
	Wrapped http.RoundTripper
	Log     *log.Logger
}

// RoundTrip implements the http.RoundTripper interface by logging request
// headers and redacting sensitive headers before forwarding the
// request to the wrapped transport
func (t *RedactingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.Log.Printf("%s %s", req.Method, req.URL.String())
	for key := range req.Header {
		if sensitiveHeaders[strings.ToLower(key)] {
			t.Log.Printf("  %s: [REDACTED]", key)
		} else {
			t.Log.Printf("  %s: %s", key, req.Header.Get(key))
		}
	}
	resp, err := t.Wrapped.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("round trip: %w", err)
	}
	return resp, nil
}
