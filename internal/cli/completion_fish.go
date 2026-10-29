package cli

import (
	"github.com/jmcampanini/fleet/internal/exitstatus"
	"github.com/spf13/cobra"
)

func newCompletionFish() *cobra.Command {
	var noDescriptions bool
	command := &cobra.Command{
		Use: "fish", Short: "Generate a Fish completion script",
		Long: `Generate a completion script for Fish.

` + completionHelp,
		Example:           `  fleet completion fish | source`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := cmd.Root().GenFishCompletion(cmd.OutOrStdout(), !noDescriptions); err != nil {
				return exitstatus.WorkError{Err: err}
			}
			return nil
		},
	}
	command.Flags().BoolVar(&noDescriptions, "no-descriptions", false, "Omit completion descriptions")
	return command
}
