package cli

import (
	"github.com/jmcampanini/fleet/internal/exitstatus"
	"github.com/spf13/cobra"
)

func newCompletionPowerShell() *cobra.Command {
	var noDescriptions bool
	command := &cobra.Command{
		Use: "powershell", Short: "Generate a PowerShell completion script",
		Long: `Generate a completion script for PowerShell.

` + completionHelp,
		Example:           `  fleet completion powershell | Out-String | Invoke-Expression`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, _ []string) error {
			generate := cmd.Root().GenPowerShellCompletionWithDesc
			if noDescriptions {
				generate = cmd.Root().GenPowerShellCompletion
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
