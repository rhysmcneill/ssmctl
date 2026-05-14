package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func completionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate a shell completion script for ssmctl and print it to stdout.

Source the script once (or add it to your shell config) to get tab completion
for all ssmctl subcommands and global flags. You do not need to run this command
again after the initial setup.

  Bash:
    source <(ssmctl completion bash)

    # To persist across sessions:
    echo 'source <(ssmctl completion bash)' >> ~/.bashrc

  Zsh:
    source <(ssmctl completion zsh)

    # To persist across sessions:
    echo 'source <(ssmctl completion zsh)' >> ~/.zshrc

  Fish:
    ssmctl completion fish | source

    # To persist across sessions:
    ssmctl completion fish > ~/.config/fish/completions/ssmctl.fish

  PowerShell:
    ssmctl completion powershell | Out-String | Invoke-Expression

    # To persist across sessions, add the above line to your PowerShell profile.`,
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		// Skip AWS initialisation — no credentials are needed to print a completion script.
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(cmd.OutOrStdout(), true)
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			default:
				return fmt.Errorf("unsupported shell %q; choose bash, zsh, fish, or powershell", args[0])
			}
		},
	}
}
