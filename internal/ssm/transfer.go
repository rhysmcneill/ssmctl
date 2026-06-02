package ssm

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"strings"
	"time"
)

const (
	// These scratch paths are shared per-instance, so concurrent transfers to
	// the same target can race and clobber each other. Keep only one active
	// in-band transfer per instance until we move to unique temp paths.
	unixTempFile    = "/tmp/._ssmctl_transfer"
	windowsTempFile = `C:\Windows\Temp\._ssmctl_transfer`
	chunkSize       = 3072
)

// TransferResult contains the summary of a completed file transfer operation.
type TransferResult struct {
	Direction   string `json:"direction"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Bytes       int64  `json:"bytes"`
	Chunks      int    `json:"chunks,omitempty"`
	// Via reports the transport used for the transfer ("ssm" for the in-band
	// SSM Run Command path or "s3" for the staged S3-backed path). It is
	// omitted from JSON output when empty to preserve historical formatting.
	Via string `json:"via,omitempty"`
	// StagingURL is the s3:// URL of the staging object used during an
	// S3-backed transfer. It is empty for in-band SSM transfers.
	StagingURL string `json:"staging_url,omitempty"`
	// KeptStaging indicates that the staging object was left in place after
	// the transfer because --keep-staging was set.
	KeptStaging bool `json:"kept_staging,omitempty"`
}

// ParseArg parses a cp argument into instance, path, and whether it is remote.
// Remote format: <instance>:/path/to/file
func ParseArg(s string) (instance, path string, isRemote bool) {
	// Windows drive-letter paths (e.g. C:\folder\file or C:/folder/file) must
	// not be treated as remote even though they contain a colon at index 1.
	if len(s) >= 3 && s[1] == ':' && (s[2] == '\\' || s[2] == '/') &&
		(s[0] >= 'A' && s[0] <= 'Z' || s[0] >= 'a' && s[0] <= 'z') {
		return "", s, false
	}
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
func Upload(ctx context.Context, client RunAPI, target TargetInfo, localPath, remotePath string, timeout time.Duration) (*TransferResult, error) {
	data, err := os.ReadFile(localPath) // #nosec G304 -- localPath is a user-supplied CLI argument, path traversal is intentional
	if err != nil {
		return nil, fmt.Errorf("failed to read local file: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	tempFile := tempFileForTarget(target)

	initCmd := fmt.Sprintf("printf '' > %s", tempFile)
	if target.IsWindows() {
		remotePath = normalizeWindowsPath(remotePath)
		initCmd = fmt.Sprintf("[System.IO.File]::WriteAllBytes(%s, [byte[]]@())", powerShellQuote(tempFile))
	}

	init, err := RunCommandForTarget(ctx, client, target, []string{initCmd}, timeout)

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

		command := fmt.Sprintf(
			"cat << 'EOF' | base64 -d >> %s\n%s\nEOF",
			tempFile,
			chunk,
		)
		if target.IsWindows() {
			command = fmt.Sprintf(
				"$bytes = [System.Convert]::FromBase64String(%s); $stream = [System.IO.File]::Open(%s, [System.IO.FileMode]::Append, [System.IO.FileAccess]::Write, [System.IO.FileShare]::None); try { $stream.Write($bytes, 0, $bytes.Length) } finally { $stream.Dispose() }",
				powerShellQuote(chunk),
				powerShellQuote(tempFile),
			)
		}

		// Using heredoc with single-quoted EOF delimiter for safe base64
		// chunk embedding to prevent interpretation of special characters
		// such as: +, /, =.
		result, err := RunCommandForTarget(ctx, client, target, []string{command}, timeout)
		if err != nil {
			return nil, fmt.Errorf("failed to send chunk: %w", err)
		}
		if result.ExitCode != 0 {
			return nil, fmt.Errorf("chunk command failed: %s", result.Stderr)
		}
		chunks++
	}

	finaliseCmd := fmt.Sprintf("mkdir -p %s && mv %s %s", path.Dir(remotePath), tempFile, remotePath)
	if target.IsWindows() {
		finaliseCmd = fmt.Sprintf(
			"$destination = %s; $dir = [System.IO.Path]::GetDirectoryName($destination); if ($dir) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }; Move-Item -Force -LiteralPath %s -Destination $destination",
			powerShellQuote(remotePath),
			powerShellQuote(tempFile),
		)
	}

	result, err := RunCommandForTarget(ctx, client, target, []string{finaliseCmd}, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to move file to destination: %w", err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("move command failed: %s", result.Stderr)
	}

	return &TransferResult{
		Direction:   "upload",
		Source:      localPath,
		Destination: target.InstanceID + ":" + remotePath,
		Bytes:       int64(len(data)),
		Chunks:      chunks,
	}, nil
}

// Download copies a remote file to a local path via SSM.
// Practical limit: ~36KB due to SSM stdout truncation.
func Download(ctx context.Context, client RunAPI, target TargetInfo, remotePath, localPath string, timeout time.Duration) (*TransferResult, error) {
	command := fmt.Sprintf("cat %s | base64", remotePath)
	if target.IsWindows() {
		remotePath = normalizeWindowsPath(remotePath)
		command = fmt.Sprintf("[System.Convert]::ToBase64String([System.IO.File]::ReadAllBytes(%s))", powerShellQuote(remotePath))
	}

	result, err := RunCommandForTarget(ctx, client, target, []string{command}, timeout)
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
		Source:      target.InstanceID + ":" + remotePath,
		Destination: localPath,
		Bytes:       int64(len(data)),
	}, nil
}

func tempFileForTarget(target TargetInfo) string {
	if target.IsWindows() {
		return windowsTempFile
	}
	return unixTempFile
}

func normalizeWindowsPath(p string) string {
	return strings.ReplaceAll(p, "/", `\`)
}
