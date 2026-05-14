package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/rhysmcneill/ssmctl/internal/app"
	"github.com/rhysmcneill/ssmctl/internal/config"
	"github.com/rhysmcneill/ssmctl/internal/output"
)

func executeForwardCmd(ctx context.Context, a *app.App, args []string) error {
	root := &cobra.Command{Use: "ssmctl", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(forwardCmd())
	root.SetArgs(args)
	return root.ExecuteContext(context.WithValue(ctx, app.ContextKey{}, a)) //nolint:wrapcheck
}

// executeForwardCmdWithOutput captures the command's stdout into buf.
// For the text output path, forward writes to stderr (OutOrStderr); use
// root.SetErr(buf) to capture that instead.
func executeForwardCmdWithOutput(ctx context.Context, a *app.App, args []string, buf *bytes.Buffer) error {
	root := &cobra.Command{Use: "ssmctl", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(forwardCmd())
	root.SetArgs(args)
	root.SetOut(buf)
	root.SetErr(buf)
	if a.Printer != nil {
		a.Printer.Out = buf
	}
	return root.ExecuteContext(context.WithValue(ctx, app.ContextKey{}, a)) //nolint:wrapcheck
}

// baseApp returns a minimal App sufficient for forward validation tests that
// fail before any AWS calls are made.
func baseForwardApp() *app.App {
	return &app.App{Config: &config.Config{Timeout: 30 * time.Second}}
}

func TestForwardCmd_MissingLocalFlagReturnsError(t *testing.T) {
	err := executeForwardCmd(context.Background(), baseForwardApp(), []string{"forward", "i-123", "--remote", "5432"})
	if err == nil {
		t.Fatal("expected error for missing --local flag, got nil")
	}
}

func TestForwardCmd_MissingRemoteFlagReturnsError(t *testing.T) {
	err := executeForwardCmd(context.Background(), baseForwardApp(), []string{"forward", "i-123", "--local", "5432"})
	if err == nil {
		t.Fatal("expected error for missing --remote flag, got nil")
	}
}

func TestForwardCmd_LocalPortOutOfRange(t *testing.T) {
	err := executeForwardCmd(context.Background(), baseForwardApp(), []string{"forward", "i-123", "--local", "70000", "--remote", "5432"})
	if err == nil {
		t.Fatal("expected error for out-of-range local port, got nil")
	}
}

func TestForwardCmd_LocalPortZero(t *testing.T) {
	err := executeForwardCmd(context.Background(), baseForwardApp(), []string{"forward", "i-123", "--local", "0", "--remote", "5432"})
	if err == nil {
		t.Fatal("expected error for --local 0, got nil")
	}
}

func TestForwardCmd_InvalidRemoteEmptyHost(t *testing.T) {
	// ":5432" has an empty host, which ParseRemoteFlag rejects before any AWS call.
	err := executeForwardCmd(context.Background(), baseForwardApp(), []string{"forward", "i-123", "--local", "5432", "--remote", ":5432"})
	if err == nil {
		t.Fatal("expected error for empty remote host, got nil")
	}
}

// TestForwardCmd_JSONBanner verifies the JSON banner is printed before
// StartPortForwardingSession is reached. The session call fails immediately at
// the region check (Region == ""), which is the cheapest way to stop execution
// without any AWS or plugin-binary dependency.
func TestForwardCmd_JSONBanner(t *testing.T) {
	a := &app.App{
		Config:  &config.Config{Output: "json", Region: "", Timeout: 30 * time.Second},
		Printer: &output.Printer{Format: "json"},
		// EC2Client nil: ResolveTarget returns i-xxx IDs directly.
		// SSMClient nil: safe because StartPortForwardingSession errors at the
		// region check before touching the client.
	}

	var buf bytes.Buffer
	err := executeForwardCmdWithOutput(context.Background(), a,
		[]string{"forward", "i-123", "--local", "5432", "--remote", "5432"}, &buf)

	// We expect an error because no region is configured.
	if err == nil {
		t.Fatal("expected error (no region configured), got nil")
	}

	var got forwardBanner
	if jsonErr := json.Unmarshal(buf.Bytes(), &got); jsonErr != nil {
		t.Fatalf("banner is not valid JSON: %v\nraw: %s", jsonErr, buf.String())
	}
	if got.InstanceID != "i-123" {
		t.Errorf("instance_id = %q, want %q", got.InstanceID, "i-123")
	}
	if got.LocalPort != 5432 {
		t.Errorf("local_port = %d, want 5432", got.LocalPort)
	}
	if got.Document != "AWS-StartPortForwardingSession" {
		t.Errorf("document = %q, want AWS-StartPortForwardingSession", got.Document)
	}
}

func TestForwardCmd_JSONBannerWithRemoteHost(t *testing.T) {
	a := &app.App{
		Config:  &config.Config{Output: "json", Region: "", Timeout: 30 * time.Second},
		Printer: &output.Printer{Format: "json"},
	}

	var buf bytes.Buffer
	err := executeForwardCmdWithOutput(context.Background(), a,
		[]string{"forward", "i-123", "--local", "5432", "--remote", "db.internal:5432"}, &buf)

	if err == nil {
		t.Fatal("expected error (no region configured), got nil")
	}

	var got forwardBanner
	if jsonErr := json.Unmarshal(buf.Bytes(), &got); jsonErr != nil {
		t.Fatalf("banner is not valid JSON: %v\nraw: %s", jsonErr, buf.String())
	}
	if got.RemoteHost != "db.internal" {
		t.Errorf("remote_host = %q, want %q", got.RemoteHost, "db.internal")
	}
	if got.Document != "AWS-StartPortForwardingSessionToRemoteHost" {
		t.Errorf("document = %q, want AWS-StartPortForwardingSessionToRemoteHost", got.Document)
	}
}

func TestForwardCmd_TextOutputLine(t *testing.T) {
	// forward writes the text status line to cmd.OutOrStderr before calling
	// StartPortForwardingSession, which fails at the region check.
	a := &app.App{
		Config:  &config.Config{Output: "text", Region: "", Timeout: 30 * time.Second},
		Printer: &output.Printer{Format: "text"},
	}

	var buf bytes.Buffer
	err := executeForwardCmdWithOutput(context.Background(), a,
		[]string{"forward", "i-123", "--local", "5432", "--remote", "5432"}, &buf)

	if err == nil {
		t.Fatal("expected error (no region configured), got nil")
	}
	if !strings.Contains(buf.String(), "5432") {
		t.Errorf("expected status line in output, got: %s", buf.String())
	}
}

func TestForwardCmd_InvalidRemoteBadPort(t *testing.T) {
	// "db:notaport" has an invalid port string.
	err := executeForwardCmd(context.Background(), baseForwardApp(), []string{"forward", "i-123", "--local", "5432", "--remote", "db:notaport"})
	if err == nil {
		t.Fatal("expected error for non-numeric remote port, got nil")
	}
}
