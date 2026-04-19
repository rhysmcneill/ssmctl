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
func Upload(ctx context.Context, client SSMRunAPI, instanceID, localPath, remotePath string, timeout time.Duration) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local file: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(data)

	init, err := RunCommand(ctx, client, instanceID, []string{fmt.Sprintf("printf '' > %s", tempFile)}, timeout)
	if err != nil {
		return fmt.Errorf("failed to initialise transfer file: %w", err)
	}
	if init.ExitCode != 0 {
		return fmt.Errorf("failed to initialise transfer file: %s", init.Stderr)
	}

	for i := 0; i < len(encoded); i += chunkSize {
		end := i + chunkSize
		if end > len(encoded) {
			end = len(encoded)
		}
		chunk := encoded[i:end]

		result, err := RunCommand(ctx, client, instanceID,
			[]string{fmt.Sprintf("printf '%%s' '%s' | base64 -d >> %s", chunk, tempFile)},
			timeout,
		)
		if err != nil {
			return fmt.Errorf("failed to send chunk: %w", err)
		}
		if result.ExitCode != 0 {
			return fmt.Errorf("chunk command failed: %s", result.Stderr)
		}
	}

	dir := filepath.Dir(remotePath)
	result, err := RunCommand(ctx, client, instanceID,
		[]string{fmt.Sprintf("mkdir -p %s && mv %s %s", dir, tempFile, remotePath)},
		timeout,
	)
	if err != nil {
		return fmt.Errorf("failed to move file to destination: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("move command failed: %s", result.Stderr)
	}

	return nil
}

// Download copies a remote file to a local path via SSM.
// Practical limit: ~36KB due to SSM stdout truncation.
func Download(ctx context.Context, client SSMRunAPI, instanceID, remotePath, localPath string, timeout time.Duration) error {
	result, err := RunCommand(ctx, client, instanceID,
		[]string{fmt.Sprintf("cat %s | base64", remotePath)},
		timeout,
	)
	if err != nil {
		return fmt.Errorf("failed to read remote file: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("remote read failed: %s", result.Stderr)
	}

	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(result.Stdout))
	if err != nil {
		return fmt.Errorf("failed to decode file content: %w", err)
	}

	if err := os.WriteFile(localPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write local file: %w", err)
	}

	return nil
}
