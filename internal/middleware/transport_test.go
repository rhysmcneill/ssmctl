package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockRoundTripper is a test double that returns a successful response without making a real request.
type mockRoundTripper struct{}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Body:       http.NoBody,
		Request:    req,
	}, nil
}

func TestRedactingTransport_RedactsAuthorizationHeader(t *testing.T) {
	var buf bytes.Buffer
	debugLog := log.New(&buf, "[DEBUG] ", log.LstdFlags)

	rt := &RedactingTransport{
		Wrapped: &mockRoundTripper{},
		Log:     debugLog,
	}

	req := httptest.NewRequest("GET", "https://example.com", nil)
	req.Header.Set("Authorization", "Bearer secret-token-12345")
	req.Header.Set("User-Agent", "test-client")

	_, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logOutput := buf.String()

	// Verify [REDACTED] appears for Authorization
	if !strings.Contains(logOutput, "[REDACTED]") {
		t.Error("expected [REDACTED] in log output for Authorization header")
	}

	// Verify raw credential does NOT appear
	if strings.Contains(logOutput, "secret-token-12345") {
		t.Error("raw Authorization token should not appear in logs")
	}

	// Verify User-Agent is logged normally (not redacted)
	if !strings.Contains(logOutput, "test-client") {
		t.Error("expected User-Agent value in log output")
	}
}

func TestRedactingTransport_RedactsXAmzSecurityToken(t *testing.T) {
	var buf bytes.Buffer
	debugLog := log.New(&buf, "[DEBUG] ", log.LstdFlags)

	rt := &RedactingTransport{
		Wrapped: &mockRoundTripper{},
		Log:     debugLog,
	}

	req := httptest.NewRequest("GET", "https://example.com", nil)
	req.Header.Set("X-Amz-Security-Token", "FwoGZXIvYXdzE...")
	req.Header.Set("X-Amz-Date", "20240101T000000Z")

	_, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logOutput := buf.String()

	// Verify [REDACTED] appears for X-Amz-Security-Token
	if !strings.Contains(logOutput, "[REDACTED]") {
		t.Error("expected [REDACTED] in log output for X-Amz-Security-Token header")
	}

	// Verify raw token does NOT appear
	if strings.Contains(logOutput, "FwoGZXIvYXdzE") {
		t.Error("raw X-Amz-Security-Token should not appear in logs")
	}

	// Verify X-Amz-Date is logged normally (not redacted)
	if !strings.Contains(logOutput, "20240101T000000Z") {
		t.Error("expected X-Amz-Date value in log output")
	}
}

func TestRedactingTransport_LogsRequestMethod(t *testing.T) {
	var buf bytes.Buffer
	debugLog := log.New(&buf, "[DEBUG] ", log.LstdFlags)

	rt := &RedactingTransport{
		Wrapped: &mockRoundTripper{},
		Log:     debugLog,
	}

	req := httptest.NewRequest("POST", "https://api.example.com/test", nil)

	_, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logOutput := buf.String()

	if !strings.Contains(logOutput, "POST") {
		t.Error("expected request method to be logged")
	}

	if !strings.Contains(logOutput, "https://api.example.com/test") {
		t.Error("expected request URL to be logged")
	}
}

func TestRedactingTransport_CaseInsensitiveSensitiveHeaders(t *testing.T) {
	var buf bytes.Buffer
	debugLog := log.New(&buf, "[DEBUG] ", log.LstdFlags)

	rt := &RedactingTransport{
		Wrapped: &mockRoundTripper{},
		Log:     debugLog,
	}

	req := httptest.NewRequest("GET", "https://example.com", nil)
	// Use different casing to verify case-insensitive matching
	req.Header.Set("AUTHORIZATION", "Bearer secret-token")
	req.Header.Set("x-amz-security-token", "token-value")

	_, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logOutput := buf.String()

	// Both should be redacted regardless of casing
	redactedCount := strings.Count(logOutput, "[REDACTED]")
	if redactedCount < 2 {
		t.Errorf("expected at least 2 [REDACTED] entries, got %d", redactedCount)
	}

	// Raw values should not appear
	if strings.Contains(logOutput, "secret-token") || strings.Contains(logOutput, "token-value") {
		t.Error("raw credential values should not appear in logs")
	}
}
