package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rhysmcneill/ssmctl/internal/app"
	ssmlib "github.com/rhysmcneill/ssmctl/internal/ssm"
)

type forwardOptions struct {
	localPort  int
	remoteFlag string
}

// forwardBanner is the JSON-serialisable summary printed before the tunnel opens.
type forwardBanner struct {
	InstanceID string `json:"instance_id"`
	LocalPort  int    `json:"local_port"`
	RemoteHost string `json:"remote_host,omitempty"`
	RemotePort int    `json:"remote_port"`
	Document   string `json:"document"`
}

func forwardCmd() *cobra.Command {
	opts := &forwardOptions{}

	cmd := &cobra.Command{
		Use:   "forward <target>",
		Short: "Forward a local port to a remote port through an SSM tunnel",
		Long: `Forward a local port to a remote port through an SSM tunnel.

<target> can be an EC2 instance ID (i-...) or a Name tag.

Local port forwarding (instance's own localhost):

  ssmctl forward web-1 --local 5432 --remote 5432

Remote-host forwarding (a host reachable from the instance):

  ssmctl forward web-1 --local 5432 --remote rds.internal.example.com:5432

The command blocks until you press Ctrl-C. The SSM session is terminated
cleanly when the process exits.

Requires the Session Manager plugin (session-manager-plugin) to be installed
and available on your PATH.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := cmd.Context().Value(app.ContextKey{}).(*app.App)

			if opts.remoteFlag == "" {
				return fmt.Errorf("--remote is required")
			}

			remoteHost, remotePort, err := ssmlib.ParseRemoteFlag(opts.remoteFlag)
			if err != nil {
				return fmt.Errorf("--remote: %w", err)
			}

			if opts.localPort < 1 || opts.localPort > 65535 {
				return fmt.Errorf("--local %d: port must be between 1 and 65535", opts.localPort)
			}

			instanceID, err := ssmlib.ResolveTarget(cmd.Context(), a.EC2Client, args[0])
			if err != nil {
				return fmt.Errorf("resolve target: %w", err)
			}

			fwd := ssmlib.PortForwardingTarget{
				LocalPort:  opts.localPort,
				RemoteHost: remoteHost,
				RemotePort: remotePort,
			}

			if a.Config.Output == "json" {
				doc := "AWS-StartPortForwardingSession"
				if remoteHost != "" {
					doc = "AWS-StartPortForwardingSessionToRemoteHost"
				}
				banner := forwardBanner{
					InstanceID: instanceID,
					LocalPort:  opts.localPort,
					RemoteHost: remoteHost,
					RemotePort: remotePort,
					Document:   doc,
				}
				if err := a.Printer.Print(banner); err != nil {
					return fmt.Errorf("print banner: %w", err)
				}
			} else {
				remote := fmt.Sprintf("%s:%d", instanceID, remotePort)
				if remoteHost != "" {
					remote = fmt.Sprintf("%s (%s:%d)", instanceID, remoteHost, remotePort)
				}
				_, _ = fmt.Fprintf(cmd.OutOrStderr(), "Forwarding localhost:%d -> %s (Ctrl-C to stop)\n", opts.localPort, remote)
			}

			return ssmlib.StartPortForwardingSession(cmd.Context(), a.SSMClient, instanceID, a.Config.Region, a.Config.Profile, fwd)
		},
	}

	cmd.Flags().IntVar(&opts.localPort, "local", 0, "Local port to listen on")
	cmd.Flags().StringVar(&opts.remoteFlag, "remote", "", "Remote port (e.g. 5432) or host:port (e.g. rds.internal:5432)")
	_ = cmd.MarkFlagRequired("local")
	_ = cmd.MarkFlagRequired("remote")

	return cmd
}
