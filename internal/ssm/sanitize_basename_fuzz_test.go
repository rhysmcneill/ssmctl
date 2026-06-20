package ssm

import (
	"testing"

	_ "github.com/AdamKorcz/go-118-fuzz-build/testing"
)

func FuzzSanitizeBasename(f *testing.F) {
	testcases := []string{
		"file.txt",
		"/path/to/file.txt",
		`C:\Users\Admin\file.txt`,
		"../../../etc/passwd",
		"./file.txt",
		"/",
		`\`,
		".",
		"..",
		"file with spaces.txt",
		"file-with-dashes.txt",
		"file_with_underscores.txt",
		"/absolute/path/to/file.txt",
		`relative\path\to\file.txt`,
		"//double//slash//file.txt",
		`\\double\backslash\file.txt`,
		"file.multiple.dots.txt",
		".hidden",
		"..hidden",
		"",
	}
	for _, tc := range testcases {
		f.Add(tc)
	}

	f.Fuzz(func(t *testing.T, input string) {
		result := sanitizeBasename(input)

		// Invariant: result should never be empty
		if result == "" {
			t.Errorf("sanitizeBasename(%q): returned empty string", input)
		}

		// Invariant: result should not be "." or "/" or "\" (directory traversal prevention)
		if result == "." || result == "/" || result == `\` {
			t.Errorf("sanitizeBasename(%q): returned dangerous value: %q", input, result)
		}
	})
}
