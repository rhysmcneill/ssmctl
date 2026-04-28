package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/rhysmcneill/ssmctl/internal/app"
	ssmlib "github.com/rhysmcneill/ssmctl/internal/ssm"
)

// ExitCodeError is returned by RunE when a remote command exits with a
// non-zero status. main inspects this type to forward the exact exit code.
type ExitCodeError struct {
	ExitCode int
}

func (e *ExitCodeError) Error() string {
	return fmt.Sprintf("command exited with code %d", e.ExitCode)
}

func runCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <target> -- <command>",
		Short: "Execute a command on a target instance via SSM",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := cmd.Context().Value(app.ContextKey{}).(*app.App)

			dashAt := cmd.ArgsLenAtDash()
			if dashAt < 0 {
				return fmt.Errorf("use -- to separate target from command, e.g.: ssmctl run <target> -- uname -a")
			}

			target := args[0]
			command := args[dashAt:]

			instanceID, err := ssmlib.ResolveTarget(cmd.Context(), a.EC2Client, target)
			if err != nil {
				return fmt.Errorf("resolve target: %w", err)
			}

			result, err := ssmlib.RunCommand(cmd.Context(), a.SSMClient, instanceID, command, a.Config.Timeout)
			if err != nil {
				return fmt.Errorf("run command: %w", err)
			}

			if result.Stdout != "" {
				fmt.Print(result.Stdout)
			}
			if result.Stderr != "" {
				fmt.Fprint(os.Stderr, result.Stderr)
			}
			if result.ExitCode != 0 {
				return &ExitCodeError{ExitCode: result.ExitCode}
			}

			return nil
		},
	}
}
