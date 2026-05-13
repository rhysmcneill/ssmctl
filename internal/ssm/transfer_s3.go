package ssm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3API is the subset of the AWS S3 client used for staged file transfers.
// *s3.Client satisfies this interface automatically.
type S3API interface {
	PutObject(ctx context.Context, in *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, in *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, opts ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// S3Location describes an S3 staging area: a bucket and an optional key prefix.
type S3Location struct {
	Bucket string
	Prefix string
}

// URL returns the canonical s3:// URL for the staging location.
func (l S3Location) URL() string {
	if l.Prefix == "" {
		return "s3://" + l.Bucket
	}
	return "s3://" + l.Bucket + "/" + strings.TrimLeft(l.Prefix, "/")
}

// ParseS3URL parses an s3://bucket[/prefix] URL into an S3Location. The prefix
// may be empty (object is staged at the bucket root) and any leading slashes
// are stripped.
func ParseS3URL(s string) (S3Location, error) {
	const scheme = "s3://"
	if !strings.HasPrefix(s, scheme) {
		return S3Location{}, fmt.Errorf("invalid S3 URL %q: must start with s3://", s)
	}

	rest := strings.TrimPrefix(s, scheme)
	if rest == "" {
		return S3Location{}, fmt.Errorf("invalid S3 URL %q: missing bucket", s)
	}

	parts := strings.SplitN(rest, "/", 2)
	bucket := parts[0]
	if bucket == "" {
		return S3Location{}, fmt.Errorf("invalid S3 URL %q: missing bucket", s)
	}

	loc := S3Location{Bucket: bucket}
	if len(parts) == 2 {
		loc.Prefix = strings.TrimRight(parts[1], "/")
	}
	return loc, nil
}

// StagingKey generates a unique S3 staging object key for the given basename
// under the supplied prefix. It is exported for use by the benchmarks package.
func StagingKey(prefix, basename string) (string, error) {
	return defaultStagingKey(prefix, basename)
}

// stagingKeyFunc generates a unique staging object key for the given basename
// underneath the supplied prefix. It is overridable in tests for deterministic
// output.
var stagingKeyFunc = defaultStagingKey

func defaultStagingKey(prefix, basename string) (string, error) {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate staging key suffix: %w", err)
	}
	suffix := hex.EncodeToString(buf[:])

	name := fmt.Sprintf("ssmctl-%s-%s", suffix, sanitizeBasename(basename))
	if prefix == "" {
		return name, nil
	}
	return strings.TrimRight(prefix, "/") + "/" + name, nil
}

// sanitizeBasename strips path separators from a basename so that an attacker
// supplied path cannot escape the staging prefix when concatenated.
func sanitizeBasename(name string) string {
	name = filepath.Base(name)
	if name == "." || name == string(filepath.Separator) {
		return "file"
	}
	return name
}

// UploadViaS3 stages a local file in S3 then triggers an `aws s3 cp` on the
// remote instance to pull it down. It bypasses the SSM SendCommand payload
// limits that constrain the in-band Upload path.
//
// Staging objects are deleted after a successful transfer unless keepStaging
// is true.
func UploadViaS3(
	ctx context.Context,
	ssmClient RunAPI,
	s3Client S3API,
	instanceID string,
	localPath, remotePath string,
	staging S3Location,
	keepStaging bool,
	timeout time.Duration,
) (*TransferResult, error) {
	file, err := os.Open(localPath) // #nosec G304 -- localPath is user-supplied via CLI
	if err != nil {
		return nil, fmt.Errorf("failed to open local file: %w", err)
	}
	defer func() { _ = file.Close() }()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat local file: %w", err)
	}

	key, err := stagingKeyFunc(staging.Prefix, filepath.Base(localPath))
	if err != nil {
		return nil, err
	}

	if _, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(staging.Bucket),
		Key:    aws.String(key),
		Body:   file,
	}); err != nil {
		return nil, fmt.Errorf("failed to stage file in S3: %w", err)
	}

	stagedURL := "s3://" + staging.Bucket + "/" + key

	dir := path.Dir(remotePath)
	pullCmd := fmt.Sprintf(
		"mkdir -p %s && aws s3 cp %s %s",
		shellQuote(dir),
		shellQuote(stagedURL),
		shellQuote(remotePath),
	)

	result, runErr := RunCommand(ctx, ssmClient, instanceID, []string{pullCmd}, timeout)
	cleanupErr := maybeCleanupStagingObject(ctx, s3Client, staging.Bucket, key, keepStaging, runErr == nil && result != nil && result.ExitCode == 0)

	if runErr != nil {
		return nil, fmt.Errorf("failed to pull staged file on instance: %w", runErr)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("remote aws s3 cp failed: %s", strings.TrimSpace(result.Stderr))
	}
	if cleanupErr != nil {
		return nil, cleanupErr
	}

	return &TransferResult{
		Direction:   "upload",
		Source:      localPath,
		Destination: instanceID + ":" + remotePath,
		Bytes:       stat.Size(),
		Via:         "s3",
		StagingURL:  stagedURL,
		KeptStaging: keepStaging,
	}, nil
}

// DownloadViaS3 instructs the remote instance to push a file to a staging S3
// object then downloads that object locally. It bypasses the SSM stdout
// truncation that constrains the in-band Download path.
//
// Staging objects are deleted after a successful transfer unless keepStaging
// is true.
func DownloadViaS3(
	ctx context.Context,
	ssmClient RunAPI,
	s3Client S3API,
	instanceID string,
	remotePath, localPath string,
	staging S3Location,
	keepStaging bool,
	timeout time.Duration,
) (*TransferResult, error) {
	key, err := stagingKeyFunc(staging.Prefix, path.Base(remotePath))
	if err != nil {
		return nil, err
	}
	stagedURL := "s3://" + staging.Bucket + "/" + key

	pushCmd := fmt.Sprintf(
		"aws s3 cp %s %s",
		shellQuote(remotePath),
		shellQuote(stagedURL),
	)

	pushed, runErr := RunCommand(ctx, ssmClient, instanceID, []string{pushCmd}, timeout)
	if runErr != nil {
		return nil, fmt.Errorf("failed to push remote file to S3: %w", runErr)
	}
	if pushed.ExitCode != 0 {
		// Best-effort cleanup if a partial object somehow exists.
		_ = maybeCleanupStagingObject(ctx, s3Client, staging.Bucket, key, keepStaging, false)
		return nil, fmt.Errorf("remote aws s3 cp failed: %s", strings.TrimSpace(pushed.Stderr))
	}

	bytes, getErr := fetchStagedObject(ctx, s3Client, staging.Bucket, key, localPath)
	cleanupErr := maybeCleanupStagingObject(ctx, s3Client, staging.Bucket, key, keepStaging, getErr == nil)
	if getErr != nil {
		return nil, getErr
	}
	if cleanupErr != nil {
		return nil, cleanupErr
	}

	return &TransferResult{
		Direction:   "download",
		Source:      instanceID + ":" + remotePath,
		Destination: localPath,
		Bytes:       bytes,
		Via:         "s3",
		StagingURL:  stagedURL,
		KeptStaging: keepStaging,
	}, nil
}

func fetchStagedObject(ctx context.Context, s3Client S3API, bucket, key, localPath string) (int64, error) {
	out, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to fetch staged object: %w", err)
	}
	defer func() { _ = out.Body.Close() }()

	dst, err := os.Create(localPath) // #nosec G304 -- localPath is user-supplied via CLI
	if err != nil {
		return 0, fmt.Errorf("failed to create local file: %w", err)
	}
	defer func() { _ = dst.Close() }()

	n, err := io.Copy(dst, out.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to write local file: %w", err)
	}
	return n, nil
}

// maybeCleanupStagingObject deletes the staging object unless keepStaging is
// true. It is only invoked after the caller knows whether the prior step
// succeeded so that failed transfers can choose to leave staging artefacts in
// place for debugging.
func maybeCleanupStagingObject(ctx context.Context, s3Client S3API, bucket, key string, keepStaging, prevSucceeded bool) error {
	if keepStaging {
		return nil
	}
	if !prevSucceeded {
		return nil
	}
	if _, err := s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("failed to delete staging object s3://%s/%s: %w", bucket, key, err)
	}
	return nil
}

// shellQuote wraps s in single quotes for safe inclusion inside a POSIX shell
// command, escaping any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ShellQuote is the exported form of shellQuote, provided for use by the
// benchmarks package.
func ShellQuote(s string) string {
	return shellQuote(s)
}
