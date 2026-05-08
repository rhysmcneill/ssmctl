package benchmarks

import (
	"testing"

	ssmlib "github.com/rhysmcneill/ssmctl/internal/ssm"
)

// Sink variables prevent the compiler from eliminating benchmark calls as dead code.
var (
	sinkString string
	sinkBool   bool
	sinkInt    int
	sinkErr    error
)

// ─── ParseArg ────────────────────────────────────────────────────────────────

func BenchmarkParseArg_Local(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, sinkString, sinkBool = ssmlib.ParseArg("./local/file.txt")
	}
}

func BenchmarkParseArg_Remote(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkString, _, sinkBool = ssmlib.ParseArg("i-0123456789abcdef0:/tmp/file.txt")
	}
}

// ─── ParseS3URL ──────────────────────────────────────────────────────────────

func BenchmarkParseS3URL_BucketOnly(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var loc ssmlib.S3Location
		loc, sinkErr = ssmlib.ParseS3URL("s3://my-bucket")
		sinkString = loc.Bucket
	}
}

func BenchmarkParseS3URL_BucketWithPrefix(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var loc ssmlib.S3Location
		loc, sinkErr = ssmlib.ParseS3URL("s3://my-bucket/staging/area")
		sinkString = loc.Prefix
	}
}

// ─── ParseRemoteFlag ─────────────────────────────────────────────────────────

func BenchmarkParseRemoteFlag_BarePort(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkString, sinkInt, sinkErr = ssmlib.ParseRemoteFlag("5432")
	}
}

func BenchmarkParseRemoteFlag_HostPort(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkString, sinkInt, sinkErr = ssmlib.ParseRemoteFlag("rds.internal.example.com:5432")
	}
}

// ─── ShellQuote ──────────────────────────────────────────────────────────────

func BenchmarkShellQuote_Simple(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkString = ssmlib.ShellQuote("/var/log/app.log")
	}
}

func BenchmarkShellQuote_WithSingleQuotes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkString = ssmlib.ShellQuote("/var/log/it's-a-log.log")
	}
}
