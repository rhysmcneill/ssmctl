package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/rhysmcneill/ssmctl/internal/config"
)

// setAWSTestEnv configures the process environment so that LoadDefaultConfig
// succeeds using fake static credentials and the supplied region, without
// reading real credential/config files from the developer's home directory.
func setAWSTestEnv(t *testing.T, region string) {
	t.Helper()
	dir := t.TempDir()                                                             // nonexistent files inside this dir avoid real file reads
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIATESTKEY000000000")                          // pragma: allowlist secret
	t.Setenv("AWS_SECRET_ACCESS_KEY", "testsecretkey0000000000000000000000000000") // pragma: allowlist secret
	t.Setenv("AWS_SESSION_TOKEN", "")
	t.Setenv("AWS_DEFAULT_REGION", region)
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_SDK_LOAD_CONFIG", "")
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
}

func TestNew_RegionFromConfig(t *testing.T) {
	setAWSTestEnv(t, "us-east-1")

	cfg := &config.Config{Region: "eu-west-1", Output: "text", Timeout: 30 * time.Second}
	a, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil App")
	}
	// An explicit region in the config must not be overwritten by the AWS SDK.
	if cfg.Region != "eu-west-1" {
		t.Errorf("cfg.Region = %q, want %q", cfg.Region, "eu-west-1")
	}
}

func TestNew_RegionSyncedFromEnv(t *testing.T) {
	setAWSTestEnv(t, "ap-southeast-1")

	// No region in Config — it should be back-filled from AWS_DEFAULT_REGION.
	cfg := &config.Config{Output: "text", Timeout: 30 * time.Second}
	_, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Region != "ap-southeast-1" {
		t.Errorf("cfg.Region = %q, want %q", cfg.Region, "ap-southeast-1")
	}
}

func TestNew_WithDebug(t *testing.T) {
	setAWSTestEnv(t, "us-east-1")

	cfg := &config.Config{Debug: true, Output: "text", Timeout: 30 * time.Second}
	a, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil App")
	}
}

func TestNew_ClientsInitialized(t *testing.T) {
	setAWSTestEnv(t, "us-east-1")

	cfg := &config.Config{Region: "us-east-1", Output: "text", Timeout: 30 * time.Second}
	a, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if a.SSMClient == nil {
		t.Error("expected SSMClient to be initialized")
	}
	if a.ListClient == nil {
		t.Error("expected ListClient to be initialized")
	}
	if a.EC2Client == nil {
		t.Error("expected EC2Client to be initialized")
	}
	if a.S3Client == nil {
		t.Error("expected S3Client to be initialized")
	}
	if a.Printer == nil {
		t.Error("expected Printer to be initialized")
	}
}

func TestNew_PrinterFormatMatchesConfig(t *testing.T) {
	setAWSTestEnv(t, "us-east-1")

	for _, format := range []string{"text", "json"} {
		t.Run(format, func(t *testing.T) {
			cfg := &config.Config{Output: format, Timeout: 30 * time.Second}
			a, err := New(cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if a.Printer.Format != format {
				t.Errorf("Printer.Format = %q, want %q", a.Printer.Format, format)
			}
		})
	}
}

func TestNew_WithDebugInitializesRedactingTransport(t *testing.T) {
	setAWSTestEnv(t, "us-east-1")

	cfg := &config.Config{Debug: true, Output: "text", Timeout: 30 * time.Second}
	a, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil App")
	}
	// HTTPClient is now configured with RedactingTransport when Debug is true.
	// The transport itself is tested in internal/middleware/transport_test.go.
	// Here we just verify that App creation succeeds without panicking.
}

func TestNew_RegionExplicitlySet(t *testing.T) {
	setAWSTestEnv(t, "us-west-2")

	cfg := &config.Config{Region: "us-west-2", Output: "text", Timeout: 30 * time.Second}
	a, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil App")
	}
	if cfg.Region != "us-west-2" {
		t.Errorf("cfg.Region = %q, want %q", cfg.Region, "us-west-2")
	}
}
