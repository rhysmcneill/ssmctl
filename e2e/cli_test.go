package e2e

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Help / global flags
// ---------------------------------------------------------------------------

func TestHelp_RootCommand(t *testing.T) {
	out, err := run("--help")
	if err != nil {
		t.Fatalf("--help exited with error: %v\noutput: %s", err, out)
	}
	for _, want := range []string{"ssmctl", "connect", "run", "cp", "version"} {
		if !strings.Contains(out, want) {
			t.Errorf("--help output missing %q:\n%s", want, out)
		}
	}
}

func TestHelp_ConnectSubcommand(t *testing.T) {
	out, err := run("connect", "--help")
	if err != nil {
		t.Fatalf("connect --help exited with error: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "connect") {
		t.Errorf("connect --help output missing 'connect':\n%s", out)
	}
}

func TestHelp_RunSubcommand(t *testing.T) {
	out, err := run("run", "--help")
	if err != nil {
		t.Fatalf("run --help exited with error: %v\noutput: %s", err, out)
	}
	for _, want := range []string{"run", "Linux/macOS targets", "AWS-RunPowerShellScript"} {
		if !strings.Contains(out, want) {
			t.Errorf("run --help output missing %q:\n%s", want, out)
		}
	}
}

func TestHelp_CpSubcommand(t *testing.T) {
	out, err := run("cp", "--help")
	if err != nil {
		t.Fatalf("cp --help exited with error: %v\noutput: %s", err, out)
	}
	for _, want := range []string{"cp", "Linux/macOS targets only", "POSIX utilities"} {
		if !strings.Contains(out, want) {
			t.Errorf("cp --help output missing %q:\n%s", want, out)
		}
	}
}

// ---------------------------------------------------------------------------
// version subcommand
// ---------------------------------------------------------------------------

func TestVersion_OutputFormat(t *testing.T) {
	out, err := run("version")
	if err != nil {
		t.Fatalf("version exited with error: %v\noutput: %s", err, out)
	}
	for _, want := range []string{"version:", "commit:", "built:"} {
		if !strings.Contains(out, want) {
			t.Errorf("version output missing field %q:\n%s", want, out)
		}
	}
}

// ---------------------------------------------------------------------------
// Global flag validation
// ---------------------------------------------------------------------------

func TestGlobalFlag_InvalidOutput(t *testing.T) {
	out, err := run("--output", "yaml", "version")
	if err == nil {
		t.Fatalf("expected non-zero exit for invalid --output flag, got nil\noutput: %s", out)
	}
	if !strings.Contains(out, "invalid output format") {
		t.Errorf("expected error about invalid output format:\n%s", out)
	}
}

func TestGlobalFlag_ProfilePassthrough(t *testing.T) {
	// We cannot call AWS, but we can verify the flag is accepted without error
	// before the command would reach the AWS layer.  We use "version" which
	// does not require AWS credentials.
	out, err := run("--profile", "nonexistent-profile", "version")
	if err != nil {
		t.Fatalf("--profile flag rejected unexpectedly: %v\noutput: %s", err, out)
	}
}

func TestGlobalFlag_RegionPassthrough(t *testing.T) {
	out, err := run("--region", "us-east-1", "version")
	if err != nil {
		t.Fatalf("--region flag rejected unexpectedly: %v\noutput: %s", err, out)
	}
}

// ---------------------------------------------------------------------------
// Argument validation — run subcommand
// ---------------------------------------------------------------------------

func TestRun_MissingDashSeparator(t *testing.T) {
	out, err := run("run", "some-target", "uname", "-a")
	if err == nil {
		t.Fatalf("expected non-zero exit when -- separator is missing, got nil\noutput: %s", out)
	}
	if !strings.Contains(out, "--") {
		t.Errorf("expected error mentioning -- separator:\n%s", out)
	}
}

func TestRun_MissingTarget(t *testing.T) {
	out, err := run("run")
	if err == nil {
		t.Fatalf("expected non-zero exit when target is missing, got nil\noutput: %s", out)
	}
	_ = out
}

// ---------------------------------------------------------------------------
// Argument validation — cp subcommand
// ---------------------------------------------------------------------------

func TestCp_BothRemote(t *testing.T) {
	out, err := run("cp", "server1:/tmp/a.txt", "server2:/tmp/b.txt")
	if err == nil {
		t.Fatalf("expected non-zero exit for cp with two remote paths, got nil\noutput: %s", out)
	}
	if !strings.Contains(out, "remote") {
		t.Errorf("expected error mentioning remote path:\n%s", out)
	}
}

func TestCp_BothLocal(t *testing.T) {
	out, err := run("cp", "/tmp/a.txt", "/tmp/b.txt")
	if err == nil {
		t.Fatalf("expected non-zero exit for cp with two local paths, got nil\noutput: %s", out)
	}
	if !strings.Contains(out, "remote") {
		t.Errorf("expected error mentioning remote path:\n%s", out)
	}
}

func TestCp_MissingArgs(t *testing.T) {
	out, err := run("cp", "/tmp/a.txt")
	if err == nil {
		t.Fatalf("expected non-zero exit for cp with one arg, got nil\noutput: %s", out)
	}
	_ = out
}

// ---------------------------------------------------------------------------
// Argument validation — connect subcommand
// ---------------------------------------------------------------------------

func TestConnect_MissingTarget(t *testing.T) {
	out, err := run("connect")
	if err == nil {
		t.Fatalf("expected non-zero exit when target is missing, got nil\noutput: %s", out)
	}
	_ = out
}
