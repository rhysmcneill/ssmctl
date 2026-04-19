package ssm

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

func TestParseArg(t *testing.T) {
	tests := []struct {
		input      string
		wantInst   string
		wantPath   string
		wantRemote bool
	}{
		{
			input:      "i-1234567890abcdef0:/tmp/file.txt",
			wantInst:   "i-1234567890abcdef0",
			wantPath:   "/tmp/file.txt",
			wantRemote: true,
		},
		{
			input:      "my-server:/var/log/app.log",
			wantInst:   "my-server",
			wantPath:   "/var/log/app.log",
			wantRemote: true,
		},
		{
			input:      "./local/file.txt",
			wantInst:   "",
			wantPath:   "./local/file.txt",
			wantRemote: false,
		},
		{
			input:      "/absolute/path.txt",
			wantInst:   "",
			wantPath:   "/absolute/path.txt",
			wantRemote: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			inst, path, remote := ParseArg(tt.input)
			if inst != tt.wantInst {
				t.Errorf("instance = %q, want %q", inst, tt.wantInst)
			}
			if path != tt.wantPath {
				t.Errorf("path = %q, want %q", path, tt.wantPath)
			}
			if remote != tt.wantRemote {
				t.Errorf("isRemote = %v, want %v", remote, tt.wantRemote)
			}
		})
	}
}

// alwaysSucceedClient returns success for every RunCommand call.
func alwaysSucceedClient() SSMRunAPI {
	return &mockSSMRunClient{
		sendCommandFn: func(_ context.Context, _ *ssm.SendCommandInput, _ ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
			return &ssm.SendCommandOutput{
				Command: &types.Command{CommandId: aws.String("cmd-abc")},
			}, nil
		},
		getCommandInvocationFn: func(_ context.Context, _ *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
			return &ssm.GetCommandInvocationOutput{
				Status:                types.CommandInvocationStatusSuccess,
				StandardOutputContent: aws.String(""),
				ResponseCode:          0,
			}, nil
		},
	}
}

func TestUpload(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	localFile := filepath.Join(t.TempDir(), "upload.txt")
	if err := os.WriteFile(localFile, []byte("hello upload"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Upload(context.Background(), alwaysSucceedClient(), "i-123", localFile, "/tmp/upload.txt", 30*time.Second)
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
}

func TestDownload(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	content := "hello download"
	encoded := base64.StdEncoding.EncodeToString([]byte(content))

	client := &mockSSMRunClient{
		sendCommandFn: func(_ context.Context, _ *ssm.SendCommandInput, _ ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
			return &ssm.SendCommandOutput{
				Command: &types.Command{CommandId: aws.String("cmd-dl")},
			}, nil
		},
		getCommandInvocationFn: func(_ context.Context, _ *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
			return &ssm.GetCommandInvocationOutput{
				Status:                types.CommandInvocationStatusSuccess,
				StandardOutputContent: aws.String(encoded),
				ResponseCode:          0,
			}, nil
		},
	}

	localFile := filepath.Join(t.TempDir(), "download.txt")

	err := Download(context.Background(), client, "i-123", "/remote/file.txt", localFile, 30*time.Second)
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	got, err := os.ReadFile(localFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Errorf("downloaded content = %q, want %q", string(got), content)
	}
}

func TestDownload_RemoteCommandFails(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	client := &mockSSMRunClient{
		sendCommandFn: func(_ context.Context, _ *ssm.SendCommandInput, _ ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
			return &ssm.SendCommandOutput{
				Command: &types.Command{CommandId: aws.String("cmd-fail")},
			}, nil
		},
		getCommandInvocationFn: func(_ context.Context, _ *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
			return &ssm.GetCommandInvocationOutput{
				Status:               types.CommandInvocationStatusFailed,
				StandardErrorContent: aws.String("No such file or directory"),
				ResponseCode:         1,
			}, nil
		},
	}

	localFile := filepath.Join(t.TempDir(), "should_not_exist.txt")
	err := Download(context.Background(), client, "i-123", "/nonexistent/file.txt", localFile, 30*time.Second)
	if err == nil {
		t.Fatal("expected error for failed remote command, got nil")
	}
}
