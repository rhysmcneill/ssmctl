package ssm

import "testing"

func FuzzParseS3URL(f *testing.F) {
	testcases := []string{
		"s3://bucket",
		"s3://bucket/prefix",
		"s3://bucket/prefix/nested/path",
		"s3://my-bucket-123",
		"s3://bucket/",
		"s3://bucket///",
		"s3://bucket/prefix/",
		"s3://",
		"s3:///",
		"http://bucket/prefix",
		"s3://bucket/prefix/file.txt",
		"S3://bucket",
		"s3://BUCKET/PREFIX",
		"s3://bucket-with-dashes",
		"s3://bucket.with.dots",
	}
	for _, tc := range testcases {
		f.Add(tc)
	}

	f.Fuzz(func(t *testing.T, input string) {
		loc, err := ParseS3URL(input)

		// Invariant: if no error, bucket must not be empty
		if err == nil && loc.Bucket == "" {
			t.Errorf("ParseS3URL(%q): returned empty bucket without error", input)
		}

		// Invariant: URL() should produce consistent output
		if err == nil {
			url := loc.URL()
			roundtrip, roundtripErr := ParseS3URL(url)
			if roundtripErr != nil {
				t.Errorf("ParseS3URL(%q): URL() produced unparseable output %q: %v", input, url, roundtripErr)
			}
			if roundtrip.Bucket != loc.Bucket {
				t.Errorf("ParseS3URL(%q): bucket changed after roundtrip", input)
			}
		}
	})
}
