package ssm

import (
	"strings"
	"testing"
)

func FuzzShellQuote(f *testing.F) {
	testcases := []string{
		"",
		"simple",
		"with spaces",
		"with'single'quotes",
		`with"double"quotes`,
		"with\\backslash",
		"with\nnewline",
		"with\ttab",
		"special!@#$%^&*()",
		"'",
		"''",
		"'''",
	}
	for _, tc := range testcases {
		f.Add(tc)
	}

	f.Fuzz(func(t *testing.T, input string) {
		result := ShellQuote(input)

		// Basic invariant: result should always start and end with single quotes
		if !strings.HasPrefix(result, "'") || !strings.HasSuffix(result, "'") {
			t.Errorf("ShellQuote(%q) = %q, doesn't have surrounding quotes", input, result)
		}

		// The result should never be empty
		if result == "" {
			t.Errorf("ShellQuote(%q) returned empty string", input)
		}
	})
}

func FuzzPowerShellQuote(f *testing.F) {
	testcases := []string{
		"",
		"simple",
		"with spaces",
		"with'single'quotes",
		`with"double"quotes`,
		"with\\backslash",
		"with\nnewline",
		"with\ttab",
		"special!@#$%^&*()",
		"'",
		"''",
		"'''",
	}
	for _, tc := range testcases {
		f.Add(tc)
	}

	f.Fuzz(func(t *testing.T, input string) {
		result := PowerShellQuote(input)

		// Basic invariant: result should always start and end with single quotes
		if !strings.HasPrefix(result, "'") || !strings.HasSuffix(result, "'") {
			t.Errorf("PowerShellQuote(%q) = %q, doesn't have surrounding quotes", input, result)
		}

		// The result should never be empty
		if result == "" {
			t.Errorf("PowerShellQuote(%q) returned empty string", input)
		}
	})
}
