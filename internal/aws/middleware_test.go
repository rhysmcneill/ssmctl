package aws

import (
	"bytes"
	"log"
	"net/http"
	"strings"
	"testing"
)

func TestRedactSensitiveHeaders_RedactsAuthorizationHeader(t *testing.T) {
	var logOutput bytes.Buffer
	debugLog := log.New(&logOutput, "[DEBUG AWS] ", log.LstdFlags)

	req, _ := http.NewRequest("POST", "https://ssm.us-east-1.amazonaws.com/", nil)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/...") // pragma: allowlist secret
	req.Header.Set("Content-Type", "application/json")

	redactSensitiveHeaders(req.Header, debugLog)

	output := logOutput.String()

	if !strings.Contains(output, "[REDACTED]") {
		t.Error("expected Authorization header to be redacted")
	}
	if strings.Contains(output, "AKIAIOSFODNN7EXAMPLE") { // pragma: allowlist secret
		t.Error("Authorization header should not contain credentials")
	}
}

func TestRedactSensitiveHeaders_RedactsSecurityToken(t *testing.T) {
	var logOutput bytes.Buffer
	debugLog := log.New(&logOutput, "[DEBUG AWS] ", log.LstdFlags)

	req, _ := http.NewRequest("POST", "https://ssm.us-east-1.amazonaws.com/", nil)
	req.Header.Set("X-Amz-Security-Token", "SESSION_TOKEN_12345")

	redactSensitiveHeaders(req.Header, debugLog)

	output := logOutput.String()

	if !strings.Contains(output, "[REDACTED]") {
		t.Error("expected X-Amz-Security-Token to be redacted")
	}
	if strings.Contains(output, "SESSION_TOKEN_12345") {
		t.Error("X-Amz-Security-Token should not be visible")
	}
}

func TestRedactSensitiveHeaders_LogsNonSensitiveHeaders(t *testing.T) {
	var logOutput bytes.Buffer
	debugLog := log.New(&logOutput, "[DEBUG AWS] ", log.LstdFlags)

	req, _ := http.NewRequest("POST", "https://ssm.us-east-1.amazonaws.com/", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "aws-cli/2.0.0")

	redactSensitiveHeaders(req.Header, debugLog)

	output := logOutput.String()

	if !strings.Contains(output, "Content-Type") || !strings.Contains(output, "application/json") {
		t.Error("Content-Type header should be logged with value")
	}
	if !strings.Contains(output, "User-Agent") || !strings.Contains(output, "aws-cli/2.0.0") {
		t.Error("User-Agent header should be logged with value")
	}
}
