// Package aws contains the middleware needed for the debug flag so it does not print out sensitive headers.
package aws

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/smithy-go/middleware"
)

// RedactingLogger returns a middleware that logs AWS requests while redacting sensitive headers.
func RedactingLogger() func(next middleware.SerializeHandler) middleware.SerializeHandler {
	debugLog := log.New(os.Stderr, "[DEBUG AWS] ", log.LstdFlags)

	return func(next middleware.SerializeHandler) middleware.SerializeHandler {
		return middleware.SerializeHandlerFunc(func(ctx context.Context, in middleware.SerializeInput) (middleware.SerializeOutput, middleware.Metadata, error) {
			req := in.Request.(*http.Request)

			// Log the request method and URL
			debugLog.Printf("%s %s\n", req.Method, req.URL.String())

			// Log and redact sensitive headers
			redactSensitiveHeaders(req.Header, debugLog)

			// Call the next handler in the middleware chain
			return next.HandleSerialize(ctx, in)
		})
	}
}

// redactSensitiveHeaders logs all headers, redacting those that contain credentials.
func redactSensitiveHeaders(headers http.Header, debugLog *log.Logger) {
	sensitiveHeaders := map[string]bool{
		"authorization":        true,
		"x-amz-security-token": true,
	}

	for key := range headers {
		if sensitiveHeaders[strings.ToLower(key)] {
			debugLog.Printf("  %s: [REDACTED]\n", key)
		} else {
			// Log non-sensitive headers with their values
			values := headers.Get(key)
			debugLog.Printf("  %s: %s\n", key, values)
		}
	}
}
