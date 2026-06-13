package ssm

import "testing"

func FuzzNormalizeWindowsPath(f *testing.F) {
	testcases := []string{
		"",
		`C:\Users\Admin\file.txt`,
		"C:/Users/Admin/file.txt",
		"/var/log/app.log",
		"path/to/file",
		`path\to\file`,
		"//double//slash//path",
		`\\double\backslash\path`,
		"C:/mixed\\path/file.txt",
		"file.txt",
		"/",
		"\\",
		"///",
		`\\\`,
	}
	for _, tc := range testcases {
		f.Add(tc)
	}

	f.Fuzz(func(t *testing.T, input string) {
		result := normalizeWindowsPath(input)

		// Invariant: result should not contain forward slashes
		if len(result) > 0 && len(input) > 0 {
			for i := 0; i < len(result); i++ {
				if result[i] == '/' {
					t.Errorf("normalizeWindowsPath(%q): result contains forward slash at position %d", input, i)
				}
			}
		}

		// Invariant: result length should equal input length (we're replacing, not adding/removing)
		if len(result) != len(input) {
			t.Errorf("normalizeWindowsPath(%q): length changed from %d to %d", input, len(input), len(result))
		}

		// Invariant: all backslashes from input should be preserved
		inputBackslashes := 0
		for i := 0; i < len(input); i++ {
			if input[i] == '\\' {
				inputBackslashes++
			}
		}
		resultBackslashes := 0
		for i := 0; i < len(result); i++ {
			if result[i] == '\\' {
				resultBackslashes++
			}
		}
		if resultBackslashes < inputBackslashes {
			t.Errorf("normalizeWindowsPath(%q): lost backslashes", input)
		}
	})
}
