package cli

import (
	"github.com/jmcampanini/fleet/internal/exitstatus"
	"github.com/spf13/cobra"
)

func newCompletionBash() *cobra.Command {
	var noDescriptions bool
	command := &cobra.Command{
		Use: "bash", Short: "Generate a Bash completion script",
		Long: `Generate a completion script for Bash. Loading it requires the
bash-completion package in the shell.

` + completionHelp,
		Example:           `  source <(fleet completion bash)`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := cmd.Root().GenBashCompletionV2(cmd.OutOrStdout(), !noDescriptions); err != nil {
				return exitstatus.WorkError{Err: err}
			}
			return nil
		},
	}
	command.Flags().BoolVar(&noDescriptions, "no-descriptions", false, "Omit completion descriptions")
	return command
}
