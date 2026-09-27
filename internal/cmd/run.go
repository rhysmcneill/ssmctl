package cmd

import (
	"fmt"
	"io"
	"strings"
	"unicode"

	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/spf13/cobra"

	"github.com/rhysmcneill/ssmctl/internal/app"
	ssmlib "github.com/rhysmcneill/ssmctl/internal/ssm"
)

type runOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

// multiRunOutput is one element of the JSON array printed when run targets
// multiple instances via --filter or --platform.
type multiRunOutput struct {
	InstanceID string `json:"instance_id"`
	Name       string `json:"name"`
	runOutput
	Error string `json:"error,omitempty"`
}

type runOptions struct {
	filter      string
	platform    string
	concurrency int
}

// ExitCodeError is returned by RunE when a remote command exits with a
// non-zero status. main inspects this type to forward the exact exit code.
type ExitCodeError struct {
	ExitCode int
	Message  string
}

func (e *ExitCodeError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("command exited with code %d", e.ExitCode)
}

func runCmd() *cobra.Command {
	opts := &runOptions{}

	cmd := &cobra.Command{
		Use:   "run [<target>] -- <command>",
		Short: "Execute a command on one or more instances via SSM",
		Long: `Execute a command on one or more instances via SSM.

The run command uses AWS-RunShellScript for Linux/macOS targets and
AWS-RunPowerShellScript for Windows targets.

Pass a single <target> (instance ID or Name tag), or use --filter and/or
--platform instead to run the command in parallel on every online instance
that matches (same matching as "ssmctl list"). Each instance's output is
printed as soon as it finishes, with every line prefixed by the instance
name, or ID when it has no Name tag:

  ssmctl run web-1 -- uname -a
  ssmctl run --filter api -- systemctl status nginx
  ssmctl run --platform linux --concurrency 10 -- df -h /

When targeting multiple instances the command exits non-zero if any instance
fails, and --output json prints an array of per-instance results.`,
		Args:               cobra.MinimumNArgs(1),
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE: func(cmd *cobra.Command, args []string) error {
			a := cmd.Context().Value(app.ContextKey{}).(*app.App)

			dashAt := cmd.ArgsLenAtDash()
			if dashAt < 0 {
				return fmt.Errorf("use -- to separate target from command, e.g.: ssmctl run <target> -- uname -a")
			}

			if cmd.Flags().Changed("filter") || cmd.Flags().Changed("platform") {
				if dashAt > 0 {
					return fmt.Errorf("<target> cannot be combined with --filter or --platform")
				}
				return runOnMatchingInstances(cmd, a, opts, args[dashAt:])
			}

			if dashAt == 0 {
				return fmt.Errorf("missing <target>: use ssmctl run <target> -- <command>, or --filter/--platform to target multiple instances")
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

	f := cmd.Flags()
	f.StringVar(&opts.filter, "filter", "", "Run on all online instances whose Name tag or ID contains this substring (case-insensitive)")
	f.StringVar(&opts.platform, "platform", "", "Run on all online instances of this platform: linux or windows")
	f.IntVar(&opts.concurrency, "concurrency", 0, "Maximum number of instances to run on in parallel with --filter/--platform (0 = unlimited)")

	return cmd
}

func runOnMatchingInstances(cmd *cobra.Command, a *app.App, opts *runOptions, commandArgs []string) error {
	if cmd.Flags().Changed("filter") && opts.filter == "" {
		return fmt.Errorf("--filter must not be empty")
	}
	if opts.concurrency < 0 {
		return fmt.Errorf("--concurrency must be 0 (unlimited) or greater")
	}

	matched, err := ssmlib.ListInstances(cmd.Context(), a.ListClient, a.EC2Client, opts.filter, opts.platform)
	if err != nil {
		return fmt.Errorf("list instances: %w", err)
	}

	online := make([]ssmlib.InstanceInfo, 0, len(matched))
	for _, inst := range matched {
		if inst.Status == string(ssmtypes.PingStatusOnline) {
			online = append(online, inst)
		}
	}
	if skipped := len(matched) - len(online); skipped > 0 {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "skipping %d matching instance(s) that are not online\n", skipped)
	}
	if len(online) == 0 {
		return fmt.Errorf("no online instances matched --filter %q --platform %q", opts.filter, opts.platform)
	}

	commandFor := func(t ssmlib.TargetInfo) []string {
		if t.IsWindows() {
			return []string{joinPowerShellArgs(commandArgs)}
		}
		return []string{joinShellArgs(commandArgs)}
	}

	jsonOutput := a.Config.Output == "json"

	var onDone func(ssmlib.TargetResult)
	var writeErr error
	if !jsonOutput {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "running on %d instance(s)...\n", len(online))
		onDone = func(r ssmlib.TargetResult) {
			if writeErr == nil {
				writeErr = printPrefixedResult(cmd.OutOrStdout(), cmd.ErrOrStderr(), r)
			}
		}
	}

	results := ssmlib.RunCommandOnInstances(cmd.Context(), a.SSMClient, online, commandFor, a.Config.Timeout, opts.concurrency, onDone)

	if writeErr != nil {
		return writeErr
	}
	if jsonOutput {
		if err := a.Printer.Print(toMultiRunOutput(results)); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
	}

	failed := 0
	for _, r := range results {
		if r.Err != nil || r.Result.ExitCode != 0 {
			failed++
		}
	}
	if failed > 0 {
		return &ExitCodeError{
			ExitCode: 1,
			Message:  fmt.Sprintf("command failed on %d of %d instances", failed, len(results)),
		}
	}
	return nil
}

func toMultiRunOutput(results []ssmlib.TargetResult) []multiRunOutput {
	out := make([]multiRunOutput, 0, len(results))
	for _, r := range results {
		item := multiRunOutput{InstanceID: r.Instance.InstanceID, Name: r.Instance.Name}
		if r.Err != nil {
			item.Error = r.Err.Error()
		} else {
			item.runOutput = runOutput{Stdout: r.Result.Stdout, Stderr: r.Result.Stderr, ExitCode: r.Result.ExitCode}
		}
		out = append(out, item)
	}
	return out
}

func printPrefixedResult(stdout, stderr io.Writer, r ssmlib.TargetResult) error {
	label := r.Instance.Label()
	if r.Err != nil {
		if _, err := fmt.Fprintf(stderr, "[%s] error: %v\n", label, r.Err); err != nil {
			return fmt.Errorf("write stderr: %w", err)
		}
		return nil
	}
	if _, err := io.WriteString(stdout, prefixLines(label, r.Result.Stdout)); err != nil {
		return fmt.Errorf("write stdout: %w", err)
	}
	errText := prefixLines(label, r.Result.Stderr)
	if r.Result.ExitCode != 0 {
		errText += fmt.Sprintf("[%s] command exited with code %d\n", label, r.Result.ExitCode)
	}
	if _, err := io.WriteString(stderr, errText); err != nil {
		return fmt.Errorf("write stderr: %w", err)
	}
	return nil
}

// prefixLines prefixes every line of s with "[label] ", terminating the last
// line with a newline if it was missing.
func prefixLines(label, s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
		b.WriteString("[" + label + "] " + line + "\n")
	}
	return b.String()
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
