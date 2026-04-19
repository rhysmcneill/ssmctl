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

Note: uploads are limited to ~2MB; downloads to ~36KB.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := cmd.Context().Value(app.ContextKey{}).(*app.App)

			srcInstance, srcPath, srcRemote := ssmlib.ParseArg(args[0])
			dstInstance, dstPath, dstRemote := ssmlib.ParseArg(args[1])

			switch {
			case srcRemote && !dstRemote:
				instanceID, err := ssmlib.ResolveTarget(cmd.Context(), a.EC2Client, srcInstance)
				if err != nil {
					return err
				}
				return ssmlib.Download(cmd.Context(), a.SSMClient, instanceID, srcPath, dstPath, a.Config.Timeout)

			case !srcRemote && dstRemote:
				instanceID, err := ssmlib.ResolveTarget(cmd.Context(), a.EC2Client, dstInstance)
				if err != nil {
					return err
				}
				return ssmlib.Upload(cmd.Context(), a.SSMClient, instanceID, srcPath, dstPath, a.Config.Timeout)

			default:
				return fmt.Errorf("exactly one of src or dst must be a remote path (e.g. my-server:/tmp/file.txt)")
			}
		},
	}
}
