package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rhysmcneill/ssmctl/internal/app"
	ssmlib "github.com/rhysmcneill/ssmctl/internal/ssm"
)

type cpOptions struct {
	via         string
	keepStaging bool
}

func cpCmd() *cobra.Command {
	opts := &cpOptions{}

	cmd := &cobra.Command{
		Use:   "cp <src> <dst>",
		Short: "Copy files to or from a target instance via SSM",
		Long: `Copy files to or from a target instance via SSM.

Remote paths are specified as <instance>:/path/to/file where <instance>
is either an instance ID (i-...) or a Name tag.

Upload:   ssmctl cp ./file.txt my-server:/tmp/file.txt
Download: ssmctl cp my-server:/var/log/app.log ./app.log

Note: in-band SSM transfers are limited to ~2MB uploads and ~36KB downloads.
Use --via s3://bucket/prefix to stage the file in S3 and lift those limits.

Linux/macOS targets use POSIX shell utilities under the hood. Windows targets
use PowerShell for in-band transfers and require the AWS CLI on the instance
for the S3-backed path.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := cmd.Context().Value(app.ContextKey{}).(*app.App)

			srcInstance, srcPath, srcRemote := ssmlib.ParseArg(args[0])
			dstInstance, dstPath, dstRemote := ssmlib.ParseArg(args[1])

			var staging ssmlib.S3Location
			useS3 := strings.TrimSpace(opts.via) != ""
			if useS3 {
				parsed, err := ssmlib.ParseS3URL(opts.via)
				if err != nil {
					return fmt.Errorf("--via: %w", err)
				}
				staging = parsed
			} else if opts.keepStaging {
				return fmt.Errorf("--keep-staging requires --via s3://bucket/prefix")
			}

			var (
				result *ssmlib.TransferResult
				err    error
			)
			switch {
			case srcRemote && !dstRemote:
				target, resolveErr := ssmlib.ResolveTargetInfo(cmd.Context(), a.EC2Client, srcInstance)
				if resolveErr != nil {
					return fmt.Errorf("resolve source instance: %w", resolveErr)
				}
				if useS3 {
					result, err = ssmlib.DownloadViaS3(cmd.Context(), a.SSMClient, a.S3Client, target, srcPath, dstPath, staging, opts.keepStaging, a.Config.Timeout)
				} else {
					result, err = ssmlib.Download(cmd.Context(), a.SSMClient, target, srcPath, dstPath, a.Config.Timeout)
				}

			case !srcRemote && dstRemote:
				target, resolveErr := ssmlib.ResolveTargetInfo(cmd.Context(), a.EC2Client, dstInstance)
				if resolveErr != nil {
					return fmt.Errorf("resolve destination instance: %w", resolveErr)
				}
				if useS3 {
					result, err = ssmlib.UploadViaS3(cmd.Context(), a.SSMClient, a.S3Client, target, srcPath, dstPath, staging, opts.keepStaging, a.Config.Timeout)
				} else {
					result, err = ssmlib.Upload(cmd.Context(), a.SSMClient, target, srcPath, dstPath, a.Config.Timeout)
				}

			default:
				return fmt.Errorf("exactly one of src or dst must be a remote path (e.g. my-server:/tmp/file.txt)")
			}
			if err != nil {
				return fmt.Errorf("transfer: %w", err)
			}
			if a.Config.Output == "json" {
				return a.Printer.Print(result)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&opts.via, "via", "", "Stage the transfer through S3 (e.g. s3://my-bucket/tmp) to lift SSM payload size limits")
	cmd.Flags().BoolVar(&opts.keepStaging, "keep-staging", false, "Keep the S3 staging object after a successful transfer (only valid with --via)")

	return cmd
}
