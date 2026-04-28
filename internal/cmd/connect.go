// Package cmd provides CLI comamnds for ssmctl, including connect, run, and cp
// subcommands. Each command is implemented as a cobra.Command that operates on
// AWS EC2 and SSM resources.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rhysmcneill/ssmctl/internal/app"
	ssmlib "github.com/rhysmcneill/ssmctl/internal/ssm"
)

func connectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect <target>",
		Short: "Start an interactive SSM session with a target instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := cmd.Context().Value(app.ContextKey{}).(*app.App)

			// Keep connect available for Windows targets: unlike run/cp, an
			// interactive SSM session does not build POSIX shell commands locally.
			instanceID, err := ssmlib.ResolveTarget(cmd.Context(), a.EC2Client, args[0])
			if err != nil {
				return fmt.Errorf("resolve target: %w", err)
			}

			return ssmlib.StartSession(cmd.Context(), a.SSMClient, instanceID, a.Config.Region, a.Config.Profile)
		},
	}
}
