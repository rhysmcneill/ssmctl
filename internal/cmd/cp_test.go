package cmd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

type mockS3CmdClient struct {
	putObjectFn    func(context.Context, *s3.PutObjectInput) error
	getObjectFn    func(context.Context, *s3.GetObjectInput) ([]byte, error)
	deleteObjectFn func(context.Context, *s3.DeleteObjectInput) error
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

func (m *mockS3CmdClient) PutObject(ctx context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if m.putObjectFn != nil {
		if err := m.putObjectFn(ctx, in); err != nil {
			return nil, err
		}
	}
	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3CmdClient) GetObject(ctx context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if m.getObjectFn != nil {
		body, err := m.getObjectFn(ctx, in)
		if err != nil {
			return nil, err
		}
		return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(body))}, nil
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(nil))}, nil
}

func (m *mockS3CmdClient) DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	if m.deleteObjectFn != nil {
		if err := m.deleteObjectFn(ctx, in); err != nil {
			return nil, err
		}
	}
	return &s3.DeleteObjectOutput{}, nil
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

func TestCpCmd_WindowsUploadUsesPowerShellDocument(t *testing.T) {
	ssmlib.SetPollInterval(10 * time.Millisecond)
	t.Cleanup(func() { ssmlib.SetPollInterval(2 * time.Second) })

	localFile := filepath.Join(t.TempDir(), "upload.txt")
	if err := os.WriteFile(localFile, []byte("hello windows cp"), 0o600); err != nil {
		t.Fatal(err)
	}

	var gotDocument string
	client := &mockSSMCmdClient{
		sendCommandFn: func(_ context.Context, in *awsssm.SendCommandInput, _ ...func(*awsssm.Options)) (*awsssm.SendCommandOutput, error) {
			gotDocument = aws.ToString(in.DocumentName)
			return &awsssm.SendCommandOutput{
				Command: &types.Command{CommandId: aws.String("cmd-win-cp")},
			}, nil
		},
		getCommandInvocationFn: func(_ context.Context, _ *awsssm.GetCommandInvocationInput, _ ...func(*awsssm.Options)) (*awsssm.GetCommandInvocationOutput, error) {
			return &awsssm.GetCommandInvocationOutput{
				Status:       types.CommandInvocationStatusSuccess,
				ResponseCode: 0,
			}, nil
		},
	}

	ec2Client := &mockEC2CmdClient{
		fn: func(_ context.Context, _ *awsec2.DescribeInstancesInput, _ ...func(*awsec2.Options)) (*awsec2.DescribeInstancesOutput, error) {
			return &awsec2.DescribeInstancesOutput{
				Reservations: []ec2types.Reservation{
					{
						Instances: []ec2types.Instance{
							{
								InstanceId: aws.String("i-wincp"),
								Platform:   ec2types.PlatformValuesWindows,
							},
						},
					},
				},
			}, nil
		},
	}

	a := &app.App{
		Config:    &config.Config{Output: "json", Timeout: 30 * time.Second},
		SSMClient: client,
		EC2Client: ec2Client,
		Printer:   &output.Printer{Format: "json"},
	}

	var buf bytes.Buffer
	if err := executeCpCmdWithOutput(context.Background(), a, []string{"cp", localFile, `i-wincp:C:\Temp\upload.txt`}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotDocument != "AWS-RunPowerShellScript" {
		t.Fatalf("DocumentName = %q, want %q", gotDocument, "AWS-RunPowerShellScript")
	}
}

func TestCpCmd_UploadViaS3RoutesThroughS3Path(t *testing.T) {
	ssmlib.SetPollInterval(10 * time.Millisecond)
	t.Cleanup(func() { ssmlib.SetPollInterval(2 * time.Second) })

	localFile := filepath.Join(t.TempDir(), "upload.txt")
	if err := os.WriteFile(localFile, []byte("hello via s3"), 0o600); err != nil {
		t.Fatal(err)
	}

	client := alwaysSucceedSSMClient()
	s3Client := &mockS3CmdClient{
		putObjectFn: func(_ context.Context, in *s3.PutObjectInput) error {
			if aws.ToString(in.Bucket) != "bucket" {
				t.Fatalf("PutObject bucket = %q, want %q", aws.ToString(in.Bucket), "bucket")
			}
			return nil
		},
	}

	a := &app.App{
		Config:    &config.Config{Output: "json", Timeout: 30 * time.Second},
		SSMClient: client,
		S3Client:  s3Client,
		Printer:   &output.Printer{Format: "json"},
	}

	var buf bytes.Buffer
	if err := executeCpCmdWithOutput(context.Background(), a, []string{"cp", "--via", "s3://bucket/tmp", localFile, "i-123:/tmp/upload.txt"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got ssmlib.TransferResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, buf.String())
	}
	if got.Via != "s3" {
		t.Fatalf("Via = %q, want %q", got.Via, "s3")
	}
}

func TestCpCmd_DownloadViaS3RoutesThroughS3Path(t *testing.T) {
	ssmlib.SetPollInterval(10 * time.Millisecond)
	t.Cleanup(func() { ssmlib.SetPollInterval(2 * time.Second) })

	client := alwaysSucceedSSMClient()
	s3Client := &mockS3CmdClient{
		getObjectFn: func(_ context.Context, in *s3.GetObjectInput) ([]byte, error) {
			if aws.ToString(in.Bucket) != "bucket" {
				t.Fatalf("GetObject bucket = %q, want %q", aws.ToString(in.Bucket), "bucket")
			}
			return []byte("downloaded via s3"), nil
		},
	}

	localFile := filepath.Join(t.TempDir(), "download.txt")
	a := &app.App{
		Config:    &config.Config{Output: "json", Timeout: 30 * time.Second},
		SSMClient: client,
		S3Client:  s3Client,
		Printer:   &output.Printer{Format: "json"},
	}

	var buf bytes.Buffer
	if err := executeCpCmdWithOutput(context.Background(), a, []string{"cp", "--via", "s3://bucket/tmp", "i-123:/remote/file.txt", localFile}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got ssmlib.TransferResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, buf.String())
	}
	if got.Via != "s3" {
		t.Fatalf("Via = %q, want %q", got.Via, "s3")
	}
}

func TestCpCmd_WindowsDownloadViaS3UsesPowerShellDocument(t *testing.T) {
	ssmlib.SetPollInterval(10 * time.Millisecond)
	t.Cleanup(func() { ssmlib.SetPollInterval(2 * time.Second) })

	var gotDocument string
	client := &mockSSMCmdClient{
		sendCommandFn: func(_ context.Context, in *awsssm.SendCommandInput, _ ...func(*awsssm.Options)) (*awsssm.SendCommandOutput, error) {
			gotDocument = aws.ToString(in.DocumentName)
			return &awsssm.SendCommandOutput{
				Command: &types.Command{CommandId: aws.String("cmd-win-dl-s3")},
			}, nil
		},
		getCommandInvocationFn: func(_ context.Context, _ *awsssm.GetCommandInvocationInput, _ ...func(*awsssm.Options)) (*awsssm.GetCommandInvocationOutput, error) {
			return &awsssm.GetCommandInvocationOutput{
				Status:       types.CommandInvocationStatusSuccess,
				ResponseCode: 0,
			}, nil
		},
	}

	ec2Client := &mockEC2CmdClient{
		fn: func(_ context.Context, _ *awsec2.DescribeInstancesInput, _ ...func(*awsec2.Options)) (*awsec2.DescribeInstancesOutput, error) {
			return &awsec2.DescribeInstancesOutput{
				Reservations: []ec2types.Reservation{
					{
						Instances: []ec2types.Instance{
							{
								InstanceId: aws.String("i-win-dl"),
								Platform:   ec2types.PlatformValuesWindows,
							},
						},
					},
				},
			}, nil
		},
	}

	s3Client := &mockS3CmdClient{
		getObjectFn: func(_ context.Context, in *s3.GetObjectInput) ([]byte, error) {
			if aws.ToString(in.Bucket) != "bucket" {
				t.Fatalf("GetObject bucket = %q, want %q", aws.ToString(in.Bucket), "bucket")
			}
			return []byte("windows download via s3"), nil
		},
	}

	localFile := filepath.Join(t.TempDir(), "download.txt")
	a := &app.App{
		Config:    &config.Config{Output: "json", Timeout: 30 * time.Second},
		SSMClient: client,
		EC2Client: ec2Client,
		S3Client:  s3Client,
		Printer:   &output.Printer{Format: "json"},
	}

	var buf bytes.Buffer
	if err := executeCpCmdWithOutput(context.Background(), a, []string{"cp", "--via", "s3://bucket/tmp", `i-win-dl:C:\Temp\file.txt`, localFile}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotDocument != "AWS-RunPowerShellScript" {
		t.Fatalf("DocumentName = %q, want %q", gotDocument, "AWS-RunPowerShellScript")
	}
}

func TestCpCmd_HelpMentionsWindowsTargetSupport(t *testing.T) {
	out := cpCmd().Long
	for _, want := range []string{"Windows targets", "PowerShell", "AWS CLI"} {
		if !strings.Contains(out, want) {
			t.Fatalf("cp help text missing %q:\n%s", want, out)
		}
	}
}
