package cmd

import (
	"fmt"
	"strings"
	"unicode"

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

The run command uses AWS-RunShellScript for Linux/macOS targets and
AWS-RunPowerShellScript for Windows targets.`,
		Args:               cobra.MinimumNArgs(1),
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE: func(cmd *cobra.Command, args []string) error {
			a := cmd.Context().Value(app.ContextKey{}).(*app.App)

			dashAt := cmd.ArgsLenAtDash()
			if dashAt < 0 {
				return fmt.Errorf("use -- to separate target from command, e.g.: ssmctl run <target> -- uname -a")
			}

			target := args[0]

			targetInfo, err := ssmlib.ResolveTargetInfo(cmd.Context(), a.EC2Client, target)
			if err != nil {
				return fmt.Errorf("resolve target: %w", err)
			}
			command := []string{joinShellArgs(args[dashAt:])}
			if targetInfo.IsWindows() {
				command = []string{joinPowerShellArgs(args[dashAt:])}
			}

			result, err := ssmlib.RunCommandForTarget(cmd.Context(), a.SSMClient, targetInfo, command, a.Config.Timeout)
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

func joinShellArgs(args []string) string {
	return joinArgs(args, shellArg)
}

func joinPowerShellArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}

	quoted := make([]string, 0, len(args))
	quoted = append(quoted, powerShellCommandName(args[0]))
	for _, arg := range args[1:] {
		quoted = append(quoted, powerShellArg(arg))
	}
	return strings.Join(quoted, " ")
}

func joinArgs(args []string, quote func(string) string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, quote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellArg(arg string) string {
	if arg == "" {
		return ssmlib.ShellQuote(arg)
	}
	for _, r := range arg {
		if !isSafeShellRune(r) {
			return ssmlib.ShellQuote(arg)
		}
	}
	return arg
}

func powerShellArg(arg string) string {
	if arg == "" {
		return ssmlib.PowerShellQuote(arg)
	}
	for _, r := range arg {
		if !isSafePowerShellRune(r) {
			return ssmlib.PowerShellQuote(arg)
		}
	}
	return arg
}

func powerShellCommandName(arg string) string {
	if arg == "" {
		return "& " + ssmlib.PowerShellQuote(arg)
	}
	for _, r := range arg {
		if !isSafePowerShellRune(r) {
			return "& " + ssmlib.PowerShellQuote(arg)
		}
	}
	return arg
}

func isSafeShellRune(r rune) bool {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return true
	}

	switch r {
	case '_', '@', '%', '+', '=', ':', ',', '.', '/', '-', '~':
		return true
	default:
		return false
	}
}

func isSafePowerShellRune(r rune) bool {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return true
	}

	switch r {
	case '_', '%', '+', '=', ':', ',', '.', '/', '\\', '-', '~':
		return true
	default:
		return false
	}
}
