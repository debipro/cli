package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

func (a *App) completionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: "Generate a shell completion script for debi. For example:\n\n" +
			"  debi completion bash > /etc/bash_completion.d/debi\n" +
			"  debi completion zsh > \"${fpath[1]}/_debi\"\n" +
			"  debi completion fish > ~/.config/fish/completions/debi.fish",
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := a.rootCmd()
			if err != nil {
				return err
			}
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(os.Stdout)
			case "zsh":
				return root.GenZshCompletion(os.Stdout)
			case "fish":
				return root.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return cmd.Help()
			}
		},
	}
}
