package cmd

import "github.com/spf13/cobra"

func runCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <target> -- <command>",
		Short: "Execute a command on a target instance via SSM",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
}
