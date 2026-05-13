package cmd

import (
	"os"
	"testing"
)

// TestRun_Help exercises all of the command setup code in Run() (flag
// registration, AddCommand) without triggering PersistentPreRunE, which
// requires real AWS credentials.
func TestRun_Help(t *testing.T) {
	old := os.Args
	defer func() { os.Args = old }()
	os.Args = []string{"ssmctl", "--help"}

	if err := Run(); err != nil {
		t.Fatalf("unexpected error for --help: %v", err)
	}
}

// TestRun_UnknownSubcommand exercises the error-return path in Run().
func TestRun_UnknownSubcommand(t *testing.T) {
	old := os.Args
	defer func() { os.Args = old }()
	os.Args = []string{"ssmctl", "nonexistent-subcommand-xyz"}

	if err := Run(); err == nil {
		t.Fatal("expected error for unknown subcommand, got nil")
	}
}
