package benchmarks

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"

	ssmlib "github.com/rhysmcneill/ssmctl/internal/ssm"
)

// ─── Mock SSM client ──────────────────────────────────────────────────────────

// benchSSMClient is a minimal no-op RunAPI mock that always reports success.
// stdout is returned verbatim as StandardOutputContent by GetCommandInvocation.
type benchSSMClient struct {
	stdout string
}

func (c *benchSSMClient) SendCommand(_ context.Context, _ *ssm.SendCommandInput, _ ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
	return &ssm.SendCommandOutput{
		Command: &types.Command{CommandId: aws.String("bench-cmd")},
	}, nil
}

func (c *benchSSMClient) GetCommandInvocation(_ context.Context, _ *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
	return &ssm.GetCommandInvocationOutput{
		Status:                types.CommandInvocationStatusSuccess,
		StandardOutputContent: aws.String(c.stdout),
		ResponseCode:          0,
	}, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func init() {
	// Keep the SSM poller from adding real wall-clock time to each benchmark.
	ssmlib.SetPollInterval(1 * time.Microsecond)
}

func writeTemp(b *testing.B, size int) string {
	b.Helper()
	data := bytes.Repeat([]byte("x"), size)
	f := filepath.Join(b.TempDir(), "payload.bin")
	if err := os.WriteFile(f, data, 0o600); err != nil {
		b.Fatal(err)
	}
	return f
}

// encodedPayload returns a base64.StdEncoding string of `size` repeated bytes,
// matching the format Download expects from `cat file | base64` on the remote.
func encodedPayload(size int) string {
	data := bytes.Repeat([]byte("x"), size)
	return base64.StdEncoding.EncodeToString(data)
}

// ─── Upload benchmarks ────────────────────────────────────────────────────────

func benchmarkUpload(b *testing.B, size int) {
	b.Helper()
	b.SetBytes(int64(size))
	client := &benchSSMClient{}
	for i := 0; i < b.N; i++ {
		localFile := writeTemp(b, size)
		dst := filepath.Join(b.TempDir(), "dst.bin")
		_, err := ssmlib.Upload(context.Background(), client, ssmlib.TargetInfo{InstanceID: "i-bench"}, localFile, dst, 30*time.Second)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUpload_1KB(b *testing.B)   { benchmarkUpload(b, 1*1024) }
func BenchmarkUpload_100KB(b *testing.B) { benchmarkUpload(b, 100*1024) }
func BenchmarkUpload_1MB(b *testing.B)   { benchmarkUpload(b, 1*1024*1024) }

// ─── Download benchmarks ──────────────────────────────────────────────────────

func benchmarkDownload(b *testing.B, size int) {
	b.Helper()
	b.SetBytes(int64(size))
	encoded := encodedPayload(size)
	client := &benchSSMClient{stdout: encoded}
	for i := 0; i < b.N; i++ {
		dst := filepath.Join(b.TempDir(), "download.bin")
		_, err := ssmlib.Download(context.Background(), client, ssmlib.TargetInfo{InstanceID: "i-bench"}, "/remote/file.bin", dst, 30*time.Second)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDownload_1KB(b *testing.B)   { benchmarkDownload(b, 1*1024) }
func BenchmarkDownload_100KB(b *testing.B) { benchmarkDownload(b, 100*1024) }
