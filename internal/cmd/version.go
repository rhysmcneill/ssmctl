package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rhysmcneill/ssmctl/internal/config"
	"github.com/rhysmcneill/ssmctl/internal/version"
)

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		// Override the root PersistentPreRunE: validate flags but skip AWS initialisation
		// so that `ssmctl version` works without any AWS credentials configured.
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			f := cmd.Root().PersistentFlags()
			output, _ := f.GetString("output")
			timeout, _ := f.GetDuration("timeout")
			return (&config.Config{Output: output, Timeout: timeout}).Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("version: %s\ncommit:  %s\nbuilt:   %s\n", version.Version, version.Commit, version.BuildDate)
			return nil
		},
	}
}
