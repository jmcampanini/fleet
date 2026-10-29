package cli

import "github.com/spf13/cobra"

const completionHelp = `The script goes to stdout; failures are diagnostics on stderr. The command
has no JSON format and creates or replaces no files. It never prompts and
needs no configuration, external programs, network, or interactive terminal.
The script is meant to be loaded by the named shell.`

func newCompletion() *cobra.Command {
	command := &cobra.Command{
		Use: "completion", Short: "Generate shell completion scripts",
		Long: `Print help when invoked without a shell. Choose a shell subcommand to
generate its completion script; see that subcommand's --help for usage.

` + completionHelp,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE:              func(cmd *cobra.Command, _ []string) error { return writeHelp(cmd) },
	}
	command.AddCommand(newCompletionBash(), newCompletionZsh(), newCompletionFish(), newCompletionPowerShell())
	return command
}
