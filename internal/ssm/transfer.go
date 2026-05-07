package ssm

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	tempFile  = "/tmp/._ssmctl_transfer"
	chunkSize = 3072
)

// TransferResult contains the summary of a completed file transfer operation.
type TransferResult struct {
	Direction   string `json:"direction"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Bytes       int64  `json:"bytes"`
	Chunks      int    `json:"chunks,omitempty"`
}

// ParseArg parses a cp argument into instance, path, and whether it is remote.
// Remote format: <instance>:/path/to/file
func ParseArg(s string) (instance, path string, isRemote bool) {
	if idx := strings.Index(s, ":"); idx > 0 {
		return s[:idx], s[idx+1:], true
	}
	return "", s, false
}

// Upload copies a local file to a remote instance via SSM.
// Practical limit: ~2MB.
//
// Safety note: base64-encoded chunks are passed to the remote shell with
// heredoc. The 'EOF' delimiter is single-quoted, which prevents shell
// interpretation of any characters in the chunk (i.e., +, /, =).
func Upload(ctx context.Context, client RunAPI, instanceID, localPath, remotePath string, timeout time.Duration) (*TransferResult, error) {
	data, err := os.ReadFile(localPath) // #nosec G304 -- localPath is a user-supplied CLI argument, path traversal is intentional
	if err != nil {
		return nil, fmt.Errorf("failed to read local file: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(data)

	init, err := RunCommand(ctx, client, instanceID, []string{fmt.Sprintf("printf '' > %s", tempFile)}, timeout)

	if err != nil {
		return nil, fmt.Errorf("failed to initialise transfer file: %w", err)
	}
	if init.ExitCode != 0 {
		return nil, fmt.Errorf("failed to initialise transfer file: %s", init.Stderr)
	}

	chunks := 0
	for i := 0; i < len(encoded); i += chunkSize {
		end := i + chunkSize
		if end > len(encoded) {
			end = len(encoded)
		}
		chunk := encoded[i:end]

		// Using heredoc with single-quoted EOF delimiter for safe base64
		// chunk embedding to prevent interpretation of special characters
		// such as: +, /, =.
		result, err := RunCommand(ctx, client, instanceID,
			[]string{fmt.Sprintf(
				"cat << 'EOF' | base64 -d >> %s\n%s\nEOF",
				tempFile,
				chunk,
			)},
			timeout,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to send chunk: %w", err)
		}
		if result.ExitCode != 0 {
			return nil, fmt.Errorf("chunk command failed: %s", result.Stderr)
		}
		chunks++
	}

	dir := filepath.Dir(remotePath)
	result, err := RunCommand(ctx, client, instanceID,
		[]string{fmt.Sprintf("mkdir -p %s && mv %s %s", dir, tempFile, remotePath)},
		timeout,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to move file to destination: %w", err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("move command failed: %s", result.Stderr)
	}

	return &TransferResult{
		Direction:   "upload",
		Source:      localPath,
		Destination: instanceID + ":" + remotePath,
		Bytes:       int64(len(data)),
		Chunks:      chunks,
	}, nil
}

// Download copies a remote file to a local path via SSM.
// Practical limit: ~36KB due to SSM stdout truncation.
func Download(ctx context.Context, client RunAPI, instanceID, remotePath, localPath string, timeout time.Duration) (*TransferResult, error) {
	result, err := RunCommand(ctx, client, instanceID,
		[]string{fmt.Sprintf("cat %s | base64", remotePath)},
		timeout,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read remote file: %w", err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("remote read failed: %s", result.Stderr)
	}

	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(result.Stdout))
	if err != nil {
		return nil, fmt.Errorf("failed to decode file content: %w", err)
	}

	if err := os.WriteFile(localPath, data, 0o600); err != nil {
		return nil, fmt.Errorf("failed to write local file: %w", err)
	}

	return &TransferResult{
		Direction:   "download",
		Source:      instanceID + ":" + remotePath,
		Destination: localPath,
		Bytes:       int64(len(data)),
	}, nil
}
