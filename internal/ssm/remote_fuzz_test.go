package ssm

import (
	"strings"
	"testing"

	_ "github.com/AdamKorcz/go-118-fuzz-build/testing"
)

func FuzzParseRemoteFlag(f *testing.F) {
	testcases := []string{
		"",
		"5432",
		"8080",
		"65535",
		"1",
		"db:5432",
		"localhost:3306",
		"192.168.1.1:9000",
		"host:1",
		"host:65535",
		":5432",
		"5432:extra",
		"notaport",
		"65536",
		"0",
		"-1",
		"db:notaport",
		"db:0",
		"db:65536",
		"a:b:5432",
		"::1",
		"::",
	}
	for _, tc := range testcases {
		f.Add(tc)
	}

	f.Fuzz(func(t *testing.T, input string) {
		host, port, err := ParseRemoteFlag(input)

		// Invariant: if error is nil, port must be valid (1-65535)
		if err == nil {
			if port < 1 || port > 65535 {
				t.Errorf("ParseRemoteFlag(%q) returned invalid port %d", input, port)
			}

			if host != "" && strings.Contains(host, ":") {
				t.Errorf("ParseRemoteFlag(%q): host %q still contains a colon after parsing", input, host)
			}
		}
	})
}
