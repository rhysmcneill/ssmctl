package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rhysmcneill/ssmctl/internal/app"
	ssmlib "github.com/rhysmcneill/ssmctl/internal/ssm"
)

func cpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cp <src> <dst>",
		Short: "Copy files to or from a target instance via SSM",
		Long: `Copy files to or from a target instance via SSM.

Remote paths are specified as <instance>:/path/to/file where <instance>
is either an instance ID (i-...) or a Name tag.

Upload:   ssmctl cp ./file.txt my-server:/tmp/file.txt
Download: ssmctl cp my-server:/var/log/app.log ./app.log

Note: uploads are limited to ~2MB; downloads to ~36KB.

Remote copy support is for Linux/macOS targets only. Uploads and downloads
construct remote shell commands that depend on POSIX utilities such as cat and
base64, which are not available by default on Windows targets.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := cmd.Context().Value(app.ContextKey{}).(*app.App)

			srcInstance, srcPath, srcRemote := ssmlib.ParseArg(args[0])
			dstInstance, dstPath, dstRemote := ssmlib.ParseArg(args[1])

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
				if target.IsWindows() {
					return fmt.Errorf("cp does not currently support Windows targets; remote copy relies on POSIX utilities such as cat and base64")
				}
				result, err = ssmlib.Download(cmd.Context(), a.SSMClient, target.InstanceID, srcPath, dstPath, a.Config.Timeout)

			case !srcRemote && dstRemote:
				target, resolveErr := ssmlib.ResolveTargetInfo(cmd.Context(), a.EC2Client, dstInstance)
				if resolveErr != nil {
					return fmt.Errorf("resolve destination instance: %w", resolveErr)
				}
				if target.IsWindows() {
					return fmt.Errorf("cp does not currently support Windows targets; remote copy relies on POSIX utilities such as cat and base64")
				}
				result, err = ssmlib.Upload(cmd.Context(), a.SSMClient, target.InstanceID, srcPath, dstPath, a.Config.Timeout)

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
}
