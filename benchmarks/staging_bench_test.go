package benchmarks

import (
	"testing"

	ssmlib "github.com/rhysmcneill/ssmctl/internal/ssm"
)

// ─── StagingKey ──────────────────────────────────────────────────────────────

func BenchmarkStagingKey_NoPrefix(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkString, sinkErr = ssmlib.StagingKey("", "database-dump.tar.gz")
	}
}

func BenchmarkStagingKey_WithPrefix(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkString, sinkErr = ssmlib.StagingKey("staging/area", "database-dump.tar.gz")
	}
}
