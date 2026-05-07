package ssm

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
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
func alwaysSucceedClient() RunAPI {
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

	content := []byte("hello upload")
	localFile := filepath.Join(t.TempDir(), "upload.txt")
	if err := os.WriteFile(localFile, content, 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := Upload(context.Background(), alwaysSucceedClient(), "i-123", localFile, "/tmp/upload.txt", 30*time.Second)
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if result == nil {
		t.Fatal("Upload() returned nil result")
	}
	if result.Direction != "upload" {
		t.Errorf("Direction = %q, want %q", result.Direction, "upload")
	}
	if result.Source != localFile {
		t.Errorf("Source = %q, want %q", result.Source, localFile)
	}
	if result.Destination != "i-123:/tmp/upload.txt" {
		t.Errorf("Destination = %q, want %q", result.Destination, "i-123:/tmp/upload.txt")
	}
	if result.Bytes != int64(len(content)) {
		t.Errorf("Bytes = %d, want %d", result.Bytes, len(content))
	}
	if result.Chunks < 1 {
		t.Errorf("Chunks = %d, want >= 1", result.Chunks)
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

	result, err := Download(context.Background(), client, "i-123", "/remote/file.txt", localFile, 30*time.Second)
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
	if result == nil {
		t.Fatal("Download() returned nil result")
	}
	if result.Direction != "download" {
		t.Errorf("Direction = %q, want %q", result.Direction, "download")
	}
	if result.Source != "i-123:/remote/file.txt" {
		t.Errorf("Source = %q, want %q", result.Source, "i-123:/remote/file.txt")
	}
	if result.Destination != localFile {
		t.Errorf("Destination = %q, want %q", result.Destination, localFile)
	}
	if result.Bytes != int64(len(content)) {
		t.Errorf("Bytes = %d, want %d", result.Bytes, len(content))
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
	_, err := Download(context.Background(), client, "i-123", "/nonexistent/file.txt", localFile, 30*time.Second)
	if err == nil {
		t.Fatal("expected error for failed remote command, got nil")
	}
}

// Test case for chunks with special characters.
func TestUploadChunkWithSpecialBase64Characters(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	// Craft data that encodes to base64 with +, /, and = characters.
	// This ensures the heredoc pattern safely handles all base64 character classes.
	testData := []byte{0xff, 0xfe, 0xfd, 0xfc, 0xfb, 0xfa}
	encoded := base64.StdEncoding.EncodeToString(testData)

	// Verify our test data actually contains special base64 characters.
	hasSpecialChars := false
	for _, ch := range encoded {
		if ch == '+' || ch == '/' || ch == '=' {
			hasSpecialChars = true
			break
		}
	}
	if !hasSpecialChars {
		t.Fatalf("test data doesn't contain special base64 chars: %s", encoded)
	}

	// Create a temporary local file with the test data.
	localFile := filepath.Join(t.TempDir(), "special_chars.bin")
	if err := os.WriteFile(localFile, testData, 0o600); err != nil {
		t.Fatal(err)
	}

	// Mock client that verifies at least one chunk command uses heredoc syntax.
	heredocFound := false
	client := &mockSSMRunClient{
		sendCommandFn: func(_ context.Context, input *ssm.SendCommandInput, _ ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
			for _, cmd := range input.Parameters["commands"] {
				if strings.Contains(cmd, "<< 'EOF'") {
					heredocFound = true
					break
				}
			}
			return &ssm.SendCommandOutput{
				Command: &types.Command{CommandId: aws.String("cmd-special")},
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

	// Upload should succeed with special base64 characters safely embedded in heredoc.
	_, err := Upload(context.Background(), client, "i-123", localFile, "/tmp/special.bin", 30*time.Second)
	if err != nil {
		t.Fatalf("Upload() with special base64 chars error = %v", err)
	}
	if !heredocFound {
		t.Error("expected at least one chunk command to use heredoc syntax, got none")
	}
}
