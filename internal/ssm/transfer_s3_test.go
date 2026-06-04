package ssm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// ---------------------------------------------------------------------------
// ParseS3URL
// ---------------------------------------------------------------------------

func TestParseS3URL(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantBucket string
		wantPrefix string
		wantErr    bool
	}{
		{name: "bucket only", input: "s3://my-bucket", wantBucket: "my-bucket"},
		{name: "bucket with prefix", input: "s3://my-bucket/tmp", wantBucket: "my-bucket", wantPrefix: "tmp"},
		{name: "bucket with nested prefix", input: "s3://my-bucket/staging/area", wantBucket: "my-bucket", wantPrefix: "staging/area"},
		{name: "trailing slash", input: "s3://my-bucket/tmp/", wantBucket: "my-bucket", wantPrefix: "tmp"},
		{name: "missing scheme", input: "my-bucket/tmp", wantErr: true},
		{name: "wrong scheme", input: "https://my-bucket/tmp", wantErr: true},
		{name: "missing bucket", input: "s3://", wantErr: true},
		{name: "leading slash only", input: "s3:///tmp", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, err := ParseS3URL(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (loc=%+v)", loc)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if loc.Bucket != tt.wantBucket {
				t.Errorf("bucket = %q, want %q", loc.Bucket, tt.wantBucket)
			}
			if loc.Prefix != tt.wantPrefix {
				t.Errorf("prefix = %q, want %q", loc.Prefix, tt.wantPrefix)
			}
		})
	}
}

func TestS3LocationURL(t *testing.T) {
	tests := []struct {
		loc  S3Location
		want string
	}{
		{loc: S3Location{Bucket: "b"}, want: "s3://b"},
		{loc: S3Location{Bucket: "b", Prefix: "tmp"}, want: "s3://b/tmp"},
		{loc: S3Location{Bucket: "b", Prefix: "/tmp"}, want: "s3://b/tmp"},
	}
	for _, tt := range tests {
		if got := tt.loc.URL(); got != tt.want {
			t.Errorf("URL() = %q, want %q", got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// staging key generation
// ---------------------------------------------------------------------------

func TestDefaultStagingKey_FormatAndUniqueness(t *testing.T) {
	k1, err := defaultStagingKey("tmp", "file.tar.gz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	k2, err := defaultStagingKey("tmp", "file.tar.gz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if k1 == k2 {
		t.Errorf("expected unique staging keys, got identical: %q", k1)
	}
	if !strings.HasPrefix(k1, "tmp/ssmctl-") {
		t.Errorf("expected prefix 'tmp/ssmctl-', got %q", k1)
	}
	if !strings.HasSuffix(k1, "-file.tar.gz") {
		t.Errorf("expected suffix '-file.tar.gz', got %q", k1)
	}
}

func TestDefaultStagingKey_NoPrefix(t *testing.T) {
	k, err := defaultStagingKey("", "file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(k, "/") {
		t.Errorf("expected no slash with empty prefix, got %q", k)
	}
	if !strings.HasPrefix(k, "ssmctl-") || !strings.HasSuffix(k, "-file.txt") {
		t.Errorf("unexpected key format: %q", k)
	}
}

func TestSanitizeBasename_StripsPathSeparators(t *testing.T) {
	k, err := defaultStagingKey("tmp", "/etc/../../passwd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(strings.TrimPrefix(k, "tmp/"), "/") {
		t.Errorf("expected no path separators in basename portion, got %q", k)
	}
}

// ---------------------------------------------------------------------------
// shellQuote
// ---------------------------------------------------------------------------

func TestShellQuote(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{in: "/tmp/file.txt", want: "'/tmp/file.txt'"},
		{in: "name with spaces", want: "'name with spaces'"},
		{in: "it's a path", want: `'it'\''s a path'`},
	}
	for _, tt := range tests {
		if got := shellQuote(tt.in); got != tt.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Mock S3 client
// ---------------------------------------------------------------------------

type mockS3Client struct {
	mu           sync.Mutex
	objects      map[string][]byte
	putErr       error
	getErr       error
	deleteErr    error
	deleteCalls  int
	deletedKeys  []string
	putCalls     int
	getCalls     int
	capturedPuts []string
	capturedGets []string
}

func newMockS3Client() *mockS3Client {
	return &mockS3Client{objects: make(map[string][]byte)}
}

func (m *mockS3Client) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.putCalls++
	m.capturedPuts = append(m.capturedPuts, aws.ToString(in.Bucket)+"/"+aws.ToString(in.Key))
	if m.putErr != nil {
		return nil, m.putErr
	}
	body, err := io.ReadAll(in.Body)
	if err != nil {
		return nil, fmt.Errorf("read PutObject body: %w", err)
	}
	m.objects[aws.ToString(in.Bucket)+"/"+aws.ToString(in.Key)] = body
	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3Client) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getCalls++
	m.capturedGets = append(m.capturedGets, aws.ToString(in.Bucket)+"/"+aws.ToString(in.Key))
	if m.getErr != nil {
		return nil, m.getErr
	}
	data, ok := m.objects[aws.ToString(in.Bucket)+"/"+aws.ToString(in.Key)]
	if !ok {
		return nil, errors.New("NoSuchKey")
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(data))}, nil
}

func (m *mockS3Client) DeleteObject(_ context.Context, in *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteCalls++
	m.deletedKeys = append(m.deletedKeys, aws.ToString(in.Bucket)+"/"+aws.ToString(in.Key))
	if m.deleteErr != nil {
		return nil, m.deleteErr
	}
	delete(m.objects, aws.ToString(in.Bucket)+"/"+aws.ToString(in.Key))
	return &s3.DeleteObjectOutput{}, nil
}

// ---------------------------------------------------------------------------
// UploadViaS3
// ---------------------------------------------------------------------------

func withFixedStagingKey(t *testing.T, key string) {
	t.Helper()
	prev := stagingKeyFunc
	stagingKeyFunc = func(_, _ string) (string, error) { return key, nil }
	t.Cleanup(func() { stagingKeyFunc = prev })
}

func ssmRunCapturer(captured *[]string, exit int32, stderr string) RunAPI {
	return &mockSSMRunClient{
		sendCommandFn: func(_ context.Context, in *ssm.SendCommandInput, _ ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
			if captured != nil {
				*captured = append(*captured, in.Parameters["commands"]...)
			}
			return &ssm.SendCommandOutput{Command: &types.Command{CommandId: aws.String("cmd-1")}}, nil
		},
		getCommandInvocationFn: func(_ context.Context, _ *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
			return &ssm.GetCommandInvocationOutput{
				Status:               types.CommandInvocationStatusSuccess,
				StandardErrorContent: aws.String(stderr),
				ResponseCode:         exit,
			}, nil
		},
	}
}

func TestUploadViaS3_Success(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	withFixedStagingKey(t, "tmp/ssmctl-aaaa-file.tar.gz")

	content := []byte("the quick brown fox")
	localFile := filepath.Join(t.TempDir(), "file.tar.gz")
	if err := os.WriteFile(localFile, content, 0o600); err != nil {
		t.Fatal(err)
	}

	mockS3 := newMockS3Client()
	var ranCmds []string
	mockSSM := ssmRunCapturer(&ranCmds, 0, "")

	result, err := UploadViaS3(
		context.Background(),
		mockSSM, mockS3,
		TargetInfo{InstanceID: "i-123"},
		localFile, "/var/data/payload.tar.gz",
		S3Location{Bucket: "my-bucket", Prefix: "tmp"},
		false,
		30*time.Second,
	)
	if err != nil {
		t.Fatalf("UploadViaS3 error: %v", err)
	}

	if mockS3.putCalls != 1 {
		t.Errorf("PutObject calls = %d, want 1", mockS3.putCalls)
	}
	if mockS3.deleteCalls != 1 {
		t.Errorf("DeleteObject calls = %d, want 1 (cleanup)", mockS3.deleteCalls)
	}
	if got := mockS3.capturedPuts[0]; got != "my-bucket/tmp/ssmctl-aaaa-file.tar.gz" {
		t.Errorf("PutObject target = %q, want my-bucket/tmp/ssmctl-aaaa-file.tar.gz", got)
	}
	if len(ranCmds) != 1 {
		t.Fatalf("expected 1 SSM command, got %d", len(ranCmds))
	}
	if !strings.Contains(ranCmds[0], "aws s3 cp 's3://my-bucket/tmp/ssmctl-aaaa-file.tar.gz' '/var/data/payload.tar.gz'") {
		t.Errorf("unexpected ssm command: %q", ranCmds[0])
	}
	if !strings.Contains(ranCmds[0], "mkdir -p '/var/data'") {
		t.Errorf("expected mkdir -p '/var/data' in ssm command: %q", ranCmds[0])
	}

	if result.Direction != "upload" {
		t.Errorf("Direction = %q, want upload", result.Direction)
	}
	if result.Via != "s3" {
		t.Errorf("Via = %q, want s3", result.Via)
	}
	if result.StagingURL != "s3://my-bucket/tmp/ssmctl-aaaa-file.tar.gz" {
		t.Errorf("StagingURL = %q", result.StagingURL)
	}
	if result.Bytes != int64(len(content)) {
		t.Errorf("Bytes = %d, want %d", result.Bytes, len(content))
	}
	if result.Destination != "i-123:/var/data/payload.tar.gz" {
		t.Errorf("Destination = %q", result.Destination)
	}
	if result.KeptStaging {
		t.Error("KeptStaging = true, want false")
	}
}

func TestUploadViaS3_KeepStaging(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	withFixedStagingKey(t, "ssmctl-keep-file.bin")

	localFile := filepath.Join(t.TempDir(), "file.bin")
	if err := os.WriteFile(localFile, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}

	mockS3 := newMockS3Client()
	mockSSM := ssmRunCapturer(nil, 0, "")

	result, err := UploadViaS3(
		context.Background(),
		mockSSM, mockS3,
		TargetInfo{InstanceID: "i-keep"},
		localFile, "/tmp/file.bin",
		S3Location{Bucket: "b"},
		true,
		30*time.Second,
	)
	if err != nil {
		t.Fatalf("UploadViaS3 error: %v", err)
	}
	if mockS3.deleteCalls != 0 {
		t.Errorf("DeleteObject calls = %d, want 0 with --keep-staging", mockS3.deleteCalls)
	}
	if !result.KeptStaging {
		t.Error("KeptStaging = false, want true")
	}
}

func TestUploadViaS3_PutObjectFails(t *testing.T) {
	withFixedStagingKey(t, "ssmctl-x-file.txt")

	localFile := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(localFile, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	mockS3 := newMockS3Client()
	mockS3.putErr = errors.New("AccessDenied")

	mockSSM := ssmRunCapturer(nil, 0, "")

	_, err := UploadViaS3(
		context.Background(),
		mockSSM, mockS3,
		TargetInfo{InstanceID: "i-123"},
		localFile, "/tmp/file.txt",
		S3Location{Bucket: "b"},
		false,
		30*time.Second,
	)
	if err == nil {
		t.Fatal("expected error from PutObject failure, got nil")
	}
	if !strings.Contains(err.Error(), "stage file in S3") {
		t.Errorf("error should mention staging: %v", err)
	}
}

func TestUploadViaS3_RemoteCommandFails_LeavesStaging(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	withFixedStagingKey(t, "ssmctl-fail-file.txt")

	localFile := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(localFile, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	mockS3 := newMockS3Client()
	mockSSM := ssmRunCapturer(nil, 1, "AccessDenied")

	_, err := UploadViaS3(
		context.Background(),
		mockSSM, mockS3,
		TargetInfo{InstanceID: "i-fail"},
		localFile, "/tmp/file.txt",
		S3Location{Bucket: "b"},
		false,
		30*time.Second,
	)
	if err == nil {
		t.Fatal("expected error from remote command failure, got nil")
	}
	if mockS3.deleteCalls != 0 {
		t.Errorf("expected staging object to remain on failure (delete calls = %d)", mockS3.deleteCalls)
	}
}

// ---------------------------------------------------------------------------
// DownloadViaS3
// ---------------------------------------------------------------------------

func TestDownloadViaS3_Success(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	withFixedStagingKey(t, "tmp/ssmctl-bbbb-app.log")

	mockS3 := newMockS3Client()
	mockS3.objects["my-bucket/tmp/ssmctl-bbbb-app.log"] = []byte("LARGE LOG CONTENT")

	var ranCmds []string
	mockSSM := ssmRunCapturer(&ranCmds, 0, "")

	localFile := filepath.Join(t.TempDir(), "downloaded.log")

	result, err := DownloadViaS3(
		context.Background(),
		mockSSM, mockS3,
		TargetInfo{InstanceID: "i-456"},
		"/var/log/app.log", localFile,
		S3Location{Bucket: "my-bucket", Prefix: "tmp"},
		false,
		30*time.Second,
	)
	if err != nil {
		t.Fatalf("DownloadViaS3 error: %v", err)
	}

	got, err := os.ReadFile(localFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "LARGE LOG CONTENT" {
		t.Errorf("downloaded content = %q, want %q", string(got), "LARGE LOG CONTENT")
	}

	if mockS3.deleteCalls != 1 {
		t.Errorf("DeleteObject calls = %d, want 1", mockS3.deleteCalls)
	}
	if len(ranCmds) != 1 {
		t.Fatalf("expected 1 SSM command, got %d", len(ranCmds))
	}
	if !strings.Contains(ranCmds[0], "aws s3 cp '/var/log/app.log' 's3://my-bucket/tmp/ssmctl-bbbb-app.log'") {
		t.Errorf("unexpected SSM command: %q", ranCmds[0])
	}

	if result.Direction != "download" {
		t.Errorf("Direction = %q, want download", result.Direction)
	}
	if result.Via != "s3" {
		t.Errorf("Via = %q, want s3", result.Via)
	}
	if result.StagingURL != "s3://my-bucket/tmp/ssmctl-bbbb-app.log" {
		t.Errorf("StagingURL = %q", result.StagingURL)
	}
	if result.Bytes != int64(len("LARGE LOG CONTENT")) {
		t.Errorf("Bytes = %d", result.Bytes)
	}
}

func TestDownloadViaS3_KeepStaging(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	withFixedStagingKey(t, "ssmctl-keep.log")

	mockS3 := newMockS3Client()
	mockS3.objects["b/ssmctl-keep.log"] = []byte("hello")

	mockSSM := ssmRunCapturer(nil, 0, "")

	localFile := filepath.Join(t.TempDir(), "out.log")
	result, err := DownloadViaS3(
		context.Background(),
		mockSSM, mockS3,
		TargetInfo{InstanceID: "i-keep"},
		"/remote/file.log", localFile,
		S3Location{Bucket: "b"},
		true,
		30*time.Second,
	)
	if err != nil {
		t.Fatalf("DownloadViaS3 error: %v", err)
	}
	if mockS3.deleteCalls != 0 {
		t.Errorf("DeleteObject calls = %d, want 0", mockS3.deleteCalls)
	}
	if !result.KeptStaging {
		t.Error("KeptStaging = false, want true")
	}
}

func TestDownloadViaS3_RemotePushFails(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	withFixedStagingKey(t, "ssmctl-fail.log")

	mockS3 := newMockS3Client()
	mockSSM := ssmRunCapturer(nil, 1, "no such file")

	localFile := filepath.Join(t.TempDir(), "out.log")
	_, err := DownloadViaS3(
		context.Background(),
		mockSSM, mockS3,
		TargetInfo{InstanceID: "i-fail"},
		"/missing.log", localFile,
		S3Location{Bucket: "b"},
		false,
		30*time.Second,
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "remote aws s3 cp failed") {
		t.Errorf("expected remote cp error, got: %v", err)
	}
	if mockS3.getCalls != 0 {
		t.Errorf("GetObject must not be called when remote push fails (got %d calls)", mockS3.getCalls)
	}
}

func TestDownloadViaS3_GetObjectFails(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	withFixedStagingKey(t, "ssmctl-gone.log")

	mockS3 := newMockS3Client()
	mockS3.getErr = errors.New("NoSuchKey")

	mockSSM := ssmRunCapturer(nil, 0, "")

	localFile := filepath.Join(t.TempDir(), "out.log")
	_, err := DownloadViaS3(
		context.Background(),
		mockSSM, mockS3,
		TargetInfo{InstanceID: "i-x"},
		"/remote/file.log", localFile,
		S3Location{Bucket: "b"},
		false,
		30*time.Second,
	)
	if err == nil {
		t.Fatal("expected error from GetObject failure, got nil")
	}
	if mockS3.deleteCalls != 0 {
		t.Errorf("expected to leave staging for diagnostics on Get failure, got %d delete calls", mockS3.deleteCalls)
	}
}

func TestUploadViaS3_WindowsUsesPowerShellAndNormalizesPath(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	withFixedStagingKey(t, "tmp/ssmctl-win-file.zip")

	localFile := filepath.Join(t.TempDir(), "file.zip")
	if err := os.WriteFile(localFile, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}

	mockS3 := newMockS3Client()
	var ranCmds []string
	mockSSM := ssmRunCapturer(&ranCmds, 0, "")

	target := TargetInfo{InstanceID: "i-win", Platform: ec2types.PlatformValuesWindows}
	if _, err := UploadViaS3(
		context.Background(),
		mockSSM, mockS3,
		target,
		localFile, `C:/Artifacts/build/file.zip`,
		S3Location{Bucket: "my-bucket", Prefix: "tmp"},
		false,
		30*time.Second,
	); err != nil {
		t.Fatalf("UploadViaS3 error: %v", err)
	}

	if len(ranCmds) != 1 {
		t.Fatalf("expected 1 SSM command, got %d", len(ranCmds))
	}
	if !strings.Contains(ranCmds[0], "New-Item -ItemType Directory -Force") || !strings.Contains(ranCmds[0], `C:\Artifacts\build\file.zip`) {
		t.Fatalf("unexpected Windows S3 upload command: %q", ranCmds[0])
	}
}

func TestDownloadViaS3_WindowsUsesPowerShellAndWindowsBasename(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	withFixedStagingKey(t, "tmp/ssmctl-win-app.log")

	mockS3 := newMockS3Client()
	mockS3.objects["my-bucket/tmp/ssmctl-win-app.log"] = []byte("LOG")

	var ranCmds []string
	mockSSM := ssmRunCapturer(&ranCmds, 0, "")

	localFile := filepath.Join(t.TempDir(), "out.log")
	target := TargetInfo{InstanceID: "i-win", Platform: ec2types.PlatformValuesWindows}
	if _, err := DownloadViaS3(
		context.Background(),
		mockSSM, mockS3,
		target,
		`C:\Logs\app.log`, localFile,
		S3Location{Bucket: "my-bucket", Prefix: "tmp"},
		false,
		30*time.Second,
	); err != nil {
		t.Fatalf("DownloadViaS3 error: %v", err)
	}

	if got := mockS3.capturedGets[0]; got != "my-bucket/tmp/ssmctl-win-app.log" {
		t.Fatalf("GetObject target = %q, want %q", got, "my-bucket/tmp/ssmctl-win-app.log")
	}
	if len(ranCmds) != 1 || !strings.Contains(ranCmds[0], `C:\Logs\app.log`) {
		t.Fatalf("unexpected Windows S3 download command: %#v", ranCmds)
	}
}
