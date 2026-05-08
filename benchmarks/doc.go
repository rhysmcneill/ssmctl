// Package benchmarks contains Go benchmarks for ssmctl's pure-Go hot paths:
// parsing helpers (ParseArg, ParseS3URL, ParseRemoteFlag, ShellQuote), staging
// key generation (StagingKey), and end-to-end transfer chunking (Upload /
// Download at 1 KB, 100 KB, and 1 MB payloads) using a no-op mock SSM client.
//
// Run locally with:
//
//	make bench
//
// Nightly CI executes the same benchmarks via the workflow at
// .github/workflows/benchmark.yml, compares results against a cached
// baseline using benchstat, and opens a GitHub issue when any benchmark
// regresses by ≥10% with statistical significance (p < 0.05).
package benchmarks
