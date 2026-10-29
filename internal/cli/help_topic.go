package cli

import "github.com/spf13/cobra"

const helpTopicGroupID = "help-topics"

// topicHelp is the contract every help topic shares.
const topicHelp = `This topic prints its text to stdout and nothing to stderr. It never
prompts and needs no configuration, terminal, CODE_DIR, Git, gh, or network.`

// newHelpTopic builds a command that does no work and prints long, followed
// by the shared topic contract, as its help, whether invoked directly or
// through 'fleet help <use>'.
func newHelpTopic(use, short, long string) *cobra.Command {
	return &cobra.Command{
		Use: use, Short: short, Long: long + "\n\n" + topicHelp, Args: cobra.NoArgs,
		GroupID: helpTopicGroupID,
		RunE:    func(cmd *cobra.Command, _ []string) error { return writeHelp(cmd) },
	}
}
