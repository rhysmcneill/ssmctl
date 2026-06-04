package ssm

import "strings"

// shellQuote wraps s in single quotes for safe inclusion inside a POSIX shell
// command, escaping any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ShellQuote is the exported form of shellQuote, provided for use by the
// benchmarks package and command-layer argument joining.
func ShellQuote(s string) string {
	return shellQuote(s)
}

func powerShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// PowerShellQuote wraps s in single quotes for safe inclusion inside a
// PowerShell command, doubling any embedded single quotes.
func PowerShellQuote(s string) string {
	return powerShellQuote(s)
}
