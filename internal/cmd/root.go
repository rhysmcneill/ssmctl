// Package cmd defines the CLI command structure and root command for ssmctl.
// It handles flag parsing, AWS configuration initialization, and delegates to
// subcommands (connect, run, cp, version).
package cmd

import (
	"context"
	"fmt"
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

// Run initializes and executes the root ssmctl command with all subcommands.
// It sets up global flags (profile, region, output, debug, timeout), validates
// configuration, and creates the App instance before delegating to subcommands.
func Run() error {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:   "ssmctl",
		Short: "A lightweight CLI for managing AWS SSM connections, remote command execution, and file transfers",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cfg := &config.Config{
				Profile: opts.Profile,
				Region:  opts.Region,
				Output:  opts.Output,
				Debug:   opts.Debug,
				Timeout: opts.Timeout,
			}

			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("validate config: %w", err)
			}

			a, err := app.New(cfg)
			if err != nil {
				return fmt.Errorf("initialize app: %w", err)
			}

			a.Printer.Out = cmd.OutOrStdout()

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

	cmd.AddCommand(connectCmd(), runCmd(), cpCmd(), versionCmd(), listCmd(), forwardCmd(), completionCmd())

	if err := cmd.Execute(); err != nil {
		return fmt.Errorf("execute command: %w", err)
	}
	return nil
}
