package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/rhysmcneill/ssmctl/internal/app"
	"github.com/rhysmcneill/ssmctl/internal/config"
)

type rootOptions struct {
	Profile string
	Region  string
	Output  string
	Debug   bool
	Timeout time.Duration
}

func Run() error {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:   "ssmctl",
		Short: "A lightweight CLI for managing AWS SSM connections, remote command execution, and file transfers",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfg := &config.Config{
				Profile: opts.Profile,
				Region:  opts.Region,
				Output:  opts.Output,
				Debug:   opts.Debug,
				Timeout: opts.Timeout,
			}

			if err := cfg.Validate(); err != nil {
				return err
			}

			a, err := app.New(cfg)
			if err != nil {
				return err
			}

			ctx := context.WithValue(cmd.Context(), app.ContextKey{}, a)
			cmd.SetContext(ctx)

			return nil
		},
	}

	f := cmd.PersistentFlags()
	f.StringVarP(&opts.Profile, "profile", "p", "", "AWS profile")
	f.StringVarP(&opts.Region, "region", "r", "", "AWS region")
	f.StringVarP(&opts.Output, "output", "o", "text", "Output format (text|json)")
	f.BoolVarP(&opts.Debug, "debug", "d", false, "Enable debug logging")
	f.DurationVarP(&opts.Timeout, "timeout", "t", 60*time.Second, "Timeout for commands")

	cmd.AddCommand(connectCmd(), runCmd(), cpCmd(), versionCmd())

	return cmd.Execute()
}
