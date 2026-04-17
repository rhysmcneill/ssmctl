package cmd

import "github.com/spf13/cobra"

func connectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect <target>",
		Short: "Start an interactive SSM session with a target instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
}
