package cmd

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/rhysmcneill/ssmctl/internal/app"
	ssmlib "github.com/rhysmcneill/ssmctl/internal/ssm"
)

type listOptions struct {
	Filter   string
	Platform string
	Tags     []string
}

func listCmd() *cobra.Command {
	opts := &listOptions{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List SSM-managed EC2 instances",
		Long: `List all EC2 instances that are managed by AWS Systems Manager.

Instances are sourced from SSM DescribeInstanceInformation and enriched with
EC2 Name tags. Use --filter, --platform and --tag to narrow the results
client-side. --tag key=value may be repeated (all must match, values are
case-sensitive); --tag key matches any instance that has the key. If no
instance has the requested tags, an error is returned.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a := cmd.Context().Value(app.ContextKey{}).(*app.App)

			tags, err := ssmlib.ParseTagFilters(opts.Tags)
			if err != nil {
				return fmt.Errorf("parse --tag: %w", err)
			}

			instances, err := ssmlib.ListInstances(cmd.Context(), a.ListClient, a.EC2Client, opts.Filter, opts.Platform, tags)
			if err != nil {
				return fmt.Errorf("list instances: %w", err)
			}

			if a.Config.Output == "json" {
				return a.Printer.Print(instances)
			}

			return printTable(cmd.OutOrStdout(), instances)
		},
	}

	f := cmd.Flags()
	f.StringVar(&opts.Filter, "filter", "", "Filter by Name tag substring (case-insensitive)")
	f.StringVar(&opts.Platform, "platform", "", "Filter by platform: linux or windows")
	f.StringArrayVar(&opts.Tags, "tag", nil, "Filter by EC2 tag key=value, or key to match any value (repeatable, ANDed)")

	return cmd
}

func printTable(out io.Writer, instances []ssmlib.InstanceInfo) error {
	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)

	fmt.Fprintln(w, "INSTANCE ID\tNAME\tPLATFORM\tAGENT VERSION\tSTATUS") //nolint:errcheck // tabwriter buffers; errors surface on Flush
	for _, inst := range instances {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", //nolint:errcheck // tabwriter buffers; errors surface on Flush
			inst.InstanceID,
			inst.Name,
			inst.Platform,
			inst.AgentVersion,
			inst.Status,
		)
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("flush table output: %w", err)
	}
	return nil
}
