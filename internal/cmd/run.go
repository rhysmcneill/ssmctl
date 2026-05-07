package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rhysmcneill/ssmctl/internal/app"
	ssmlib "github.com/rhysmcneill/ssmctl/internal/ssm"
)

type runOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

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
		Long: `Execute a command on a target instance via SSM.

The run command uses the AWS-RunShellScript document and is intended for
Linux/macOS targets. Windows targets require AWS-RunPowerShellScript, which
ssmctl does not currently select automatically.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := cmd.Context().Value(app.ContextKey{}).(*app.App)

			dashAt := cmd.ArgsLenAtDash()
			if dashAt < 0 {
				return fmt.Errorf("use -- to separate target from command, e.g.: ssmctl run <target> -- uname -a")
			}

			target := args[0]
			command := args[dashAt:]

			targetInfo, err := ssmlib.ResolveTargetInfo(cmd.Context(), a.EC2Client, target)
			if err != nil {
				return fmt.Errorf("resolve target: %w", err)
			}
			if targetInfo.IsWindows() {
				return fmt.Errorf("run does not currently support Windows targets; Windows targets require AWS-RunPowerShellScript, which ssmctl does not currently select automatically")
			}

			result, err := ssmlib.RunCommand(cmd.Context(), a.SSMClient, targetInfo.InstanceID, command, a.Config.Timeout)
			if err != nil {
				return fmt.Errorf("run command: %w", err)
			}

			if a.Config.Output == "json" {
				if err := a.Printer.Print(runOutput{
					Stdout:   result.Stdout,
					Stderr:   result.Stderr,
					ExitCode: result.ExitCode,
				}); err != nil {
					return fmt.Errorf("write output: %w", err)
				}
			} else {
				if result.Stdout != "" {
					if _, err := fmt.Fprint(cmd.OutOrStdout(), result.Stdout); err != nil {
						return fmt.Errorf("write stdout: %w", err)
					}
				}
				if result.Stderr != "" {
					if _, err := fmt.Fprint(cmd.ErrOrStderr(), result.Stderr); err != nil {
						return fmt.Errorf("write stderr: %w", err)
					}
				}
			}
			if result.ExitCode != 0 {
				return &ExitCodeError{ExitCode: result.ExitCode}
			}

			return nil
		},
	}
}
