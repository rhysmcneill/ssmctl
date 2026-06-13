package ssm

import "testing"

func FuzzParseArg(f *testing.F) {
	testcases := []string{
		"",
		"i-1234567890abcdef0:/tmp/file.txt",
		"my-server:/var/log/app.log",
		"./local/file.txt",
		"/absolute/path.txt",
		`C:\Users\Admin\file.txt`,
		"c:/Users/Admin/file.txt",
		`D:\`,
		":",
		"::",
		"host:",
		":path",
		"C:",
		"C:\\",
		"C:/",
		"a:/path",
		"Z:\\file",
		"host:8080:/path",
		"192.168.1.1:/file",
		"[::1]:/file",
	}
	for _, tc := range testcases {
		f.Add(tc)
	}

	f.Fuzz(func(t *testing.T, input string) {
		instance, path, isRemote := ParseArg(input)

		// Invariant: if isRemote is true, instance must not be empty
		if isRemote && instance == "" {
			t.Errorf("ParseArg(%q): isRemote=true but instance is empty", input)
		}

		// Invariant: path should never be empty if input is not empty
		if input != "" && path == "" {
			t.Errorf("ParseArg(%q): returned empty path for non-empty input", input)
		}

		// Invariant: Windows drive paths should never be marked as remote
		isWindowsDrive := len(input) >= 3 && input[1] == ':' && (input[2] == '\\' || input[2] == '/') &&
			((input[0] >= 'A' && input[0] <= 'Z') || (input[0] >= 'a' && input[0] <= 'z'))
		if isWindowsDrive && isRemote {
			t.Errorf("ParseArg(%q): Windows drive path marked as remote", input)
		}
	})
}
