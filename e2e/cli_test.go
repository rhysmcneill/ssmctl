package e2e

import (
	"os"
	"path/filepath"
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
	for _, want := range []string{"run", "AWS-RunShellScript", "AWS-RunPowerShellScript"} {
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
	for _, want := range []string{"cp", "Windows targets", "PowerShell", "--via", "--keep-staging"} {
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
	localA := filepath.Join(os.TempDir(), "a.txt")
	localB := filepath.Join(os.TempDir(), "b.txt")
	out, err := run("cp", localA, localB)
	if err == nil {
		t.Fatalf("expected non-zero exit for cp with two local paths, got nil\noutput: %s", out)
	}
	if !strings.Contains(out, "remote") {
		t.Errorf("expected error mentioning remote path:\n%s", out)
	}
}

func TestCp_MissingArgs(t *testing.T) {
	localA := filepath.Join(os.TempDir(), "a.txt")
	out, err := run("cp", localA)
	if err == nil {
		t.Fatalf("expected non-zero exit for cp with one arg, got nil\noutput: %s", out)
	}
	_ = out
}

func TestCp_KeepStagingWithoutVia(t *testing.T) {
	localA := filepath.Join(os.TempDir(), "a.txt")
	out, err := run("cp", "--keep-staging", localA, "server:/tmp/b.txt")
	if err == nil {
		t.Fatalf("expected non-zero exit when --keep-staging is used without --via, got nil\noutput: %s", out)
	}
	if !strings.Contains(out, "--keep-staging requires --via") {
		t.Errorf("expected error mentioning --keep-staging requires --via:\n%s", out)
	}
}

func TestCp_InvalidViaURL(t *testing.T) {
	localA := filepath.Join(os.TempDir(), "a.txt")
	out, err := run("cp", "--via", "not-an-s3-url", localA, "server:/tmp/b.txt")
	if err == nil {
		t.Fatalf("expected non-zero exit for invalid --via URL, got nil\noutput: %s", out)
	}
	if !strings.Contains(out, "must start with s3://") {
		t.Errorf("expected error mentioning s3:// scheme:\n%s", out)
	}
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

// ---------------------------------------------------------------------------
// forward subcommand
// ---------------------------------------------------------------------------

func TestHelp_ForwardSubcommand(t *testing.T) {
	out, err := run("forward", "--help")
	if err != nil {
		t.Fatalf("forward --help exited with error: %v\noutput: %s", err, out)
	}
	for _, want := range []string{"forward", "--local", "--remote", "Session Manager plugin", "Ctrl-C"} {
		if !strings.Contains(out, want) {
			t.Errorf("forward --help output missing %q:\n%s", want, out)
		}
	}
}

func TestForward_MissingTarget(t *testing.T) {
	out, err := run("forward", "--local", "5432", "--remote", "5432")
	if err == nil {
		t.Fatalf("expected non-zero exit when target is missing, got nil\noutput: %s", out)
	}
	_ = out
}

func TestForward_MissingLocalFlag(t *testing.T) {
	out, err := run("forward", "web-1", "--remote", "5432")
	if err == nil {
		t.Fatalf("expected non-zero exit when --local is missing, got nil\noutput: %s", out)
	}
	if !strings.Contains(out, "--local") {
		t.Errorf("expected error mentioning --local flag:\n%s", out)
	}
}

func TestForward_MissingRemoteFlag(t *testing.T) {
	out, err := run("forward", "web-1", "--local", "5432")
	if err == nil {
		t.Fatalf("expected non-zero exit when --remote is missing, got nil\noutput: %s", out)
	}
	if !strings.Contains(out, "--remote") {
		t.Errorf("expected error mentioning --remote flag:\n%s", out)
	}
}

func TestForward_InvalidRemote(t *testing.T) {
	out, err := run("forward", "web-1", "--local", "5432", "--remote", "not:a:valid:port")
	if err == nil {
		t.Fatalf("expected non-zero exit for invalid --remote, got nil\noutput: %s", out)
	}
	if !strings.Contains(out, "invalid port") {
		t.Errorf("expected error mentioning invalid port:\n%s", out)
	}
}

func TestForward_LocalPortOutOfRange(t *testing.T) {
	out, err := run("forward", "web-1", "--local", "0", "--remote", "5432")
	if err == nil {
		t.Fatalf("expected non-zero exit for --local 0, got nil\noutput: %s", out)
	}
	if !strings.Contains(out, "65535") {
		t.Errorf("expected error mentioning port range:\n%s", out)
	}
}
