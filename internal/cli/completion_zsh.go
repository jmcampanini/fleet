package cli

import (
	"github.com/jmcampanini/fleet/internal/exitstatus"
	"github.com/spf13/cobra"
)

func newCompletionZsh() *cobra.Command {
	var noDescriptions bool
	command := &cobra.Command{
		Use: "zsh", Short: "Generate a Zsh completion script",
		Long: `Generate a completion script for Zsh. Enable compinit in the shell before
loading the script.

` + completionHelp,
		Example: `  autoload -Uz compinit && compinit
  source <(fleet completion zsh)`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, _ []string) error {
			generate := cmd.Root().GenZshCompletion
			if noDescriptions {
				generate = cmd.Root().GenZshCompletionNoDesc
			}
			if err := generate(cmd.OutOrStdout()); err != nil {
				return exitstatus.WorkError{Err: err}
			}
			return nil
		},
	}
	command.Flags().BoolVar(&noDescriptions, "no-descriptions", false, "Omit completion descriptions")
	return command
}
