package cmd

import (
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

			instanceID, err := ssmlib.ResolveTarget(cmd.Context(), a.EC2Client, args[0])
			if err != nil {
				return err
			}

			return ssmlib.StartSession(cmd.Context(), a.SSMClient, instanceID, a.Config.Region, a.Config.Profile)
		},
	}
}
