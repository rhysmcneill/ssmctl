package ssm

import (
	"encoding/base64"
	"strings"
	"testing"

	_ "github.com/AdamKorcz/go-118-fuzz-build/testing"
)

func FuzzDownloadBase64Decoding(f *testing.F) {
	f.Add("")                             // empty string
	f.Add("SGVsbG8gV29ybGQ=")             // valid base64: "Hello World"
	f.Add("   SGVsbG8gV29ybGQ=   ")       // valid base64 with whitespace
	f.Add("\n\tSGVsbG8gV29ybGQ=\n\t")     // valid base64 with tabs/newlines
	f.Add("SGVsbG8gV29ybGQ")              // valid base64 without padding
	f.Add("!!!invalid!!!")                // invalid base64 characters
	f.Add("SGVs=bG8gV29ybGQ=")            // invalid base64 structure
	f.Add("A")                            // single character
	f.Add("====")                         // only padding
	f.Add("SGVsbG8gV29ybGQ=\x00poisoned") // null byte injection

	f.Fuzz(func(t *testing.T, encoded string) {
		// Simulate what Download() does
		trimmed := strings.TrimSpace(encoded)

		decoded, err := base64.StdEncoding.DecodeString(trimmed)

		// Invariant: if decoding succeeds, decoded is valid binary data
		if err == nil {
			// Invariant: decoded content is never nil (may be empty slice)
			if decoded == nil {
				t.Errorf("FuzzDownloadBase64Decoding: decoded is nil for input %q", encoded)
			}

			// Invariant: re-decoding the re-encoded data gives the same result
			reencoded := base64.StdEncoding.EncodeToString(decoded)
			redecoded, err := base64.StdEncoding.DecodeString(reencoded)
			if err != nil {
				t.Errorf("FuzzDownloadBase64Decoding: re-encoded string failed to decode: %v", err)
			}
			if string(decoded) != string(redecoded) {
				t.Errorf("FuzzDownloadBase64Decoding: round-trip mismatch for %q", encoded)
			}
			return
		}

	})
}
