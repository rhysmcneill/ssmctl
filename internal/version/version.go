// Package version contains version information for ssmctl.
// These variables are typically set during the build process using ldflags.
package version

var (
	// Version is the semantic version of ssmctl.
	Version = "0.0.1"
	// Commit is the git commit hash from which ssmctl was built.
	Commit = "none"
	// BuildDate is the date and time when ssmctl was built.
	BuildDate = "unknown"
)
