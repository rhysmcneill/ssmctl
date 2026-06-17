package ssm

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func FuzzRemoteBaseName(f *testing.F) {
	testcases := []string{
		"file.txt",
		"/path/to/file.txt",
		`C:\Users\Admin\file.txt`,
		"/",
		`\`,
		".",
		"..",
		"/absolute/path/to/file.txt",
		`C:\absolute\path\to\file.txt`,
		"relative/path/to/file.txt",
		`relative\path\to\file.txt`,
		"//double//slash//file.txt",
		`\\double\backslash\file.txt`,
		"file.multiple.dots.txt",
		".hidden",
		"",
		"/file",
		`\file`,
		"file",
		"path/",
		`path\`,
	}
	for _, tc := range testcases {
		f.Add(tc)
	}

	f.Fuzz(func(t *testing.T, input string) {
		unixTarget := TargetInfo{InstanceID: "i-123", Platform: ""}
		windowsTarget := TargetInfo{InstanceID: "i-456", Platform: types.PlatformValuesWindows}

		unixResult := remoteBaseName(unixTarget, input)
		windowsResult := remoteBaseName(windowsTarget, input)

		// Only check for separators that match the platform
		if unixResult == "" {
			t.Errorf("remoteBaseName(Unix, %q): returned empty string", input)
		}

		if windowsResult == "" {
			t.Errorf("remoteBaseName(Windows, %q): returned empty string", input)
		}
	})
}
