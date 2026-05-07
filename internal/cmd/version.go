package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rhysmcneill/ssmctl/internal/config"
	"github.com/rhysmcneill/ssmctl/internal/output"
	"github.com/rhysmcneill/ssmctl/internal/version"
)

type versionOutput struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"buildDate"`
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		// Override the root PersistentPreRunE: validate flags but skip AWS initialisation
		// so that `ssmctl version` works without any AWS credentials configured.
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error { // args was changed to _ to fix lint error.
			f := cmd.Root().PersistentFlags()
			outputFmt, _ := f.GetString("output")
			timeout, _ := f.GetDuration("timeout")
			return (&config.Config{Output: outputFmt, Timeout: timeout}).Validate()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			outputFmt, _ := cmd.Root().PersistentFlags().GetString("output")
			if outputFmt == "json" {
				p := &output.Printer{Format: outputFmt, Out: cmd.OutOrStdout()}
				return p.Print(versionOutput{
					Version:   version.Version,
					Commit:    version.Commit,
					BuildDate: version.BuildDate,
				})
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "version: %s\ncommit:  %s\nbuilt:   %s\n", version.Version, version.Commit, version.BuildDate); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}
}
