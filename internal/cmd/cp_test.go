package cmd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/spf13/cobra"

	"github.com/rhysmcneill/ssmctl/internal/app"
	"github.com/rhysmcneill/ssmctl/internal/config"
	"github.com/rhysmcneill/ssmctl/internal/output"
	ssmlib "github.com/rhysmcneill/ssmctl/internal/ssm"
)

func executeCpCmdWithOutput(ctx context.Context, a *app.App, args []string, buf *bytes.Buffer) error {
	root := &cobra.Command{Use: "ssmctl", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(cpCmd())
	root.SetArgs(args)
	root.SetOut(buf)
	if a.Printer != nil {
		a.Printer.Out = buf
	}
	return root.ExecuteContext(context.WithValue(ctx, app.ContextKey{}, a)) //nolint:wrapcheck
}

func alwaysSucceedSSMClient() *mockSSMCmdClient {
	return &mockSSMCmdClient{
		sendCommandFn: func(_ context.Context, _ *awsssm.SendCommandInput, _ ...func(*awsssm.Options)) (*awsssm.SendCommandOutput, error) {
			return &awsssm.SendCommandOutput{
				Command: &types.Command{CommandId: aws.String("cmd-cp")},
			}, nil
		},
		getCommandInvocationFn: func(_ context.Context, _ *awsssm.GetCommandInvocationInput, _ ...func(*awsssm.Options)) (*awsssm.GetCommandInvocationOutput, error) {
			return &awsssm.GetCommandInvocationOutput{
				Status:                types.CommandInvocationStatusSuccess,
				StandardOutputContent: aws.String(""),
				ResponseCode:          0,
			}, nil
		},
	}
}

func TestCpCmd_UploadJSONOutput(t *testing.T) {
	ssmlib.SetPollInterval(10 * time.Millisecond)
	t.Cleanup(func() { ssmlib.SetPollInterval(2 * time.Second) })

	localFile := filepath.Join(t.TempDir(), "upload.txt")
	if err := os.WriteFile(localFile, []byte("hello cp upload"), 0o600); err != nil {
		t.Fatal(err)
	}

	a := &app.App{
		Config:    &config.Config{Output: "json", Timeout: 30 * time.Second},
		SSMClient: alwaysSucceedSSMClient(),
		Printer:   &output.Printer{Format: "json"},
	}

	var buf bytes.Buffer
	err := executeCpCmdWithOutput(context.Background(), a, []string{"cp", localFile, "i-123:/tmp/upload.txt"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got ssmlib.TransferResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, buf.String())
	}
	if got.Direction != "upload" {
		t.Errorf("direction = %q, want %q", got.Direction, "upload")
	}
	if got.Bytes != int64(len("hello cp upload")) {
		t.Errorf("bytes = %d, want %d", got.Bytes, len("hello cp upload"))
	}
	if got.Chunks < 1 {
		t.Errorf("chunks = %d, want >= 1", got.Chunks)
	}
}

func TestCpCmd_DownloadJSONOutput(t *testing.T) {
	ssmlib.SetPollInterval(10 * time.Millisecond)
	t.Cleanup(func() { ssmlib.SetPollInterval(2 * time.Second) })

	content := []byte("hello cp download")
	encoded := base64.StdEncoding.EncodeToString(content)

	client := &mockSSMCmdClient{
		sendCommandFn: func(_ context.Context, _ *awsssm.SendCommandInput, _ ...func(*awsssm.Options)) (*awsssm.SendCommandOutput, error) {
			return &awsssm.SendCommandOutput{
				Command: &types.Command{CommandId: aws.String("cmd-dl")},
			}, nil
		},
		getCommandInvocationFn: func(_ context.Context, _ *awsssm.GetCommandInvocationInput, _ ...func(*awsssm.Options)) (*awsssm.GetCommandInvocationOutput, error) {
			return &awsssm.GetCommandInvocationOutput{
				Status:                types.CommandInvocationStatusSuccess,
				StandardOutputContent: aws.String(encoded),
				ResponseCode:          0,
			}, nil
		},
	}

	localFile := filepath.Join(t.TempDir(), "download.txt")

	a := &app.App{
		Config:    &config.Config{Output: "json", Timeout: 30 * time.Second},
		SSMClient: client,
		Printer:   &output.Printer{Format: "json"},
	}

	var buf bytes.Buffer
	err := executeCpCmdWithOutput(context.Background(), a, []string{"cp", "i-123:/remote/file.txt", localFile}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got ssmlib.TransferResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, buf.String())
	}
	if got.Direction != "download" {
		t.Errorf("direction = %q, want %q", got.Direction, "download")
	}
	if got.Bytes != int64(len(content)) {
		t.Errorf("bytes = %d, want %d", got.Bytes, len(content))
	}
}

func TestCpCmd_BothLocalPathsReturnsError(t *testing.T) {
	a := &app.App{
		Config:  &config.Config{Output: "text", Timeout: 30 * time.Second},
		Printer: &output.Printer{Format: "text"},
	}

	var buf bytes.Buffer
	err := executeCpCmdWithOutput(context.Background(), a, []string{"cp", "/tmp/a.txt", "/tmp/b.txt"}, &buf)
	if err == nil {
		t.Fatal("expected error when both src and dst are local paths, got nil")
	}
}

func TestCpCmd_BothRemotePathsReturnsError(t *testing.T) {
	a := &app.App{
		Config:  &config.Config{Output: "text", Timeout: 30 * time.Second},
		Printer: &output.Printer{Format: "text"},
	}

	var buf bytes.Buffer
	err := executeCpCmdWithOutput(context.Background(), a, []string{"cp", "i-123:/a.txt", "i-456:/b.txt"}, &buf)
	if err == nil {
		t.Fatal("expected error when both src and dst are remote paths, got nil")
	}
}

func TestCpCmd_KeepStagingWithoutViaReturnsError(t *testing.T) {
	localFile := filepath.Join(t.TempDir(), "upload.txt")
	if err := os.WriteFile(localFile, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	a := &app.App{
		Config:  &config.Config{Output: "text", Timeout: 30 * time.Second},
		Printer: &output.Printer{Format: "text"},
	}

	var buf bytes.Buffer
	err := executeCpCmdWithOutput(context.Background(), a, []string{"cp", "--keep-staging", localFile, "i-123:/tmp/upload.txt"}, &buf)
	if err == nil {
		t.Fatal("expected error for --keep-staging without --via, got nil")
	}
}

func TestCpCmd_InvalidViaURLReturnsError(t *testing.T) {
	localFile := filepath.Join(t.TempDir(), "upload.txt")
	if err := os.WriteFile(localFile, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	a := &app.App{
		Config:  &config.Config{Output: "text", Timeout: 30 * time.Second},
		Printer: &output.Printer{Format: "text"},
	}

	var buf bytes.Buffer
	err := executeCpCmdWithOutput(context.Background(), a, []string{"cp", "--via", "not-an-s3-url", localFile, "i-123:/tmp/upload.txt"}, &buf)
	if err == nil {
		t.Fatal("expected error for invalid --via URL, got nil")
	}
}

func TestCpCmd_UploadTextOutputSilent(t *testing.T) {
	ssmlib.SetPollInterval(10 * time.Millisecond)
	t.Cleanup(func() { ssmlib.SetPollInterval(2 * time.Second) })

	localFile := filepath.Join(t.TempDir(), "upload.txt")
	if err := os.WriteFile(localFile, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}

	a := &app.App{
		Config:    &config.Config{Output: "text", Timeout: 30 * time.Second},
		SSMClient: alwaysSucceedSSMClient(),
		Printer:   &output.Printer{Format: "text"},
	}

	var buf bytes.Buffer
	err := executeCpCmdWithOutput(context.Background(), a, []string{"cp", localFile, "i-123:/tmp/upload.txt"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output for text format, got: %s", buf.String())
	}
}
