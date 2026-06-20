//go:build e2e

// AWS integration tests — require real AWS credentials and a running EC2
// instance with the SSM Agent installed.
//
// Set the following environment variables before running:
//
//	E2E_INSTANCE_ID           — Linux/macOS instance ID to target (e.g. i-0123456789abcdef0)
//	E2E_WINDOWS_INSTANCE_ID   — Windows instance ID to target
//	E2E_S3_STAGING_URL        — s3://bucket[/prefix] used for staged transfer tests
//
// Region, profile, and all other AWS configuration are picked up from the
// standard AWS environment variables (AWS_DEFAULT_REGION, AWS_PROFILE, etc.)
// or ~/.aws/config — no extra test-specific variables are needed.
//
// Run with:
//
//	make e2e-aws

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// instanceID returns the E2E_INSTANCE_ID env var or skips the test.
func instanceID(t *testing.T) string {
	t.Helper()
	id := os.Getenv("E2E_INSTANCE_ID")
	if id == "" {
		t.Skip("E2E_INSTANCE_ID not set — skipping AWS integration test")
	}
	return id
}

func windowsInstanceID(t *testing.T) string {
	t.Helper()
	id := os.Getenv("E2E_WINDOWS_INSTANCE_ID")
	if id == "" {
		t.Skip("E2E_WINDOWS_INSTANCE_ID not set — skipping Windows AWS integration test")
	}
	return id
}

func stagingURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("E2E_S3_STAGING_URL")
	if url == "" {
		t.Skip("E2E_S3_STAGING_URL not set — skipping S3-backed AWS integration test")
	}
	return url
}

// ---------------------------------------------------------------------------
// run subcommand
// ---------------------------------------------------------------------------

func TestAWS_Run_Echo(t *testing.T) {
	id := instanceID(t)

	out, err := run("run", id, "--", "echo", "ssmctl-e2e-ok")
	if err != nil {
		t.Fatalf("run echo failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "ssmctl-e2e-ok") {
		t.Errorf("expected 'ssmctl-e2e-ok' in output:\n%s", out)
	}
}

func TestAWS_Run_NonZeroExitCode(t *testing.T) {
	id := instanceID(t)

	_, err := run("run", id, "--", "false")
	if err == nil {
		t.Fatal("expected non-zero exit from 'false' command, got nil")
	}
}

func TestAWS_Run_Stderr(t *testing.T) {
	id := instanceID(t)

	_, err := run("run", id, "--", "sh", "-c", "echo err >&2; exit 1")
	if err == nil {
		t.Fatal("expected non-zero exit, got nil")
	}
}

func TestAWS_Run_WindowsPowerShell(t *testing.T) {
	id := windowsInstanceID(t)

	out, err := run("run", id, "--", "Write-Output", "ssmctl-windows-e2e-ok")
	if err != nil {
		t.Fatalf("run windows powershell failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "ssmctl-windows-e2e-ok") {
		t.Errorf("expected 'ssmctl-windows-e2e-ok' in output:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// cp subcommand
// ---------------------------------------------------------------------------

func TestAWS_Cp_UploadAndDownload(t *testing.T) {
	id := instanceID(t)

	content := fmt.Sprintf("ssmctl-e2e-upload-%d", os.Getpid())
	localSrc := t.TempDir() + "/upload.txt"
	localDst := t.TempDir() + "/download.txt"
	remotePath := "/tmp/ssmctl-e2e-upload.txt"

	if err := os.WriteFile(localSrc, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Upload
	if out, err := run("cp", localSrc, fmt.Sprintf("%s:%s", id, remotePath)); err != nil {
		t.Fatalf("upload failed: %v\noutput: %s", err, out)
	}

	// Download
	if out, err := run("cp", fmt.Sprintf("%s:%s", id, remotePath), localDst); err != nil {
		t.Fatalf("download failed: %v\noutput: %s", err, out)
	}

	got, err := os.ReadFile(localDst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Errorf("downloaded content = %q, want %q", string(got), content)
	}
}

func TestAWS_Cp_DownloadNonExistentFile(t *testing.T) {
	id := instanceID(t)

	localDst := t.TempDir() + "/nope.txt"
	remotePath := "/tmp/ssmctl-e2e-does-not-exist-xyz.txt"

	_, err := run("cp", fmt.Sprintf("%s:%s", id, remotePath), localDst)
	if err == nil {
		t.Fatal("expected error when downloading non-existent remote file, got nil")
	}
}

func TestAWS_Cp_WindowsUploadAndDownload(t *testing.T) {
	id := windowsInstanceID(t)

	content := fmt.Sprintf("ssmctl-e2e-win-upload-%d", os.Getpid())
	localSrc := filepath.Join(t.TempDir(), "upload.txt")
	localDst := filepath.Join(t.TempDir(), "download.txt")
	remotePath := `C:\Windows\Temp\ssmctl-e2e-upload.txt`

	if err := os.WriteFile(localSrc, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if out, err := run("cp", localSrc, fmt.Sprintf("%s:%s", id, remotePath)); err != nil {
		t.Fatalf("windows upload failed: %v\noutput: %s", err, out)
	}

	if out, err := run("cp", fmt.Sprintf("%s:%s", id, remotePath), localDst); err != nil {
		t.Fatalf("windows download failed: %v\noutput: %s", err, out)
	}

	got, err := os.ReadFile(localDst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Errorf("downloaded content = %q, want %q", string(got), content)
	}
}

func TestAWS_Cp_WindowsUploadAndDownloadViaS3(t *testing.T) {
	id := windowsInstanceID(t)
	via := stagingURL(t)

	content := fmt.Sprintf("ssmctl-e2e-win-s3-%d", os.Getpid())
	localSrc := filepath.Join(t.TempDir(), "upload.txt")
	localDst := filepath.Join(t.TempDir(), "download.txt")
	remotePath := `C:\Windows\Temp\ssmctl-e2e-s3.txt`

	if err := os.WriteFile(localSrc, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if out, err := run("cp", "--via", via, localSrc, fmt.Sprintf("%s:%s", id, remotePath)); err != nil {
		t.Fatalf("windows s3 upload failed: %v\noutput: %s", err, out)
	}

	if out, err := run("cp", "--via", via, fmt.Sprintf("%s:%s", id, remotePath), localDst); err != nil {
		t.Fatalf("windows s3 download failed: %v\noutput: %s", err, out)
	}

	got, err := os.ReadFile(localDst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Errorf("downloaded content = %q, want %q", string(got), content)
	}
}
