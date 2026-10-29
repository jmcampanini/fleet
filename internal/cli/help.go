package cli

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/jmcampanini/fleet/internal/exitstatus"
	"github.com/spf13/cobra"
)

func newHelp() *cobra.Command {
	return &cobra.Command{
		Use: "help [command ...]", Short: "Show help for a command or topic",
		Long: `Print the named command or topic's help to stdout. With no command,
print root help. An unknown command or topic fails with a diagnostic on
stderr. Help has no machine-readable format, never prompts, and needs no
configuration, external programs, network, or interactive terminal.`,
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: completeHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			target, rest, err := cmd.Root().Find(args)
			if err != nil || len(rest) > 0 {
				return fmt.Errorf("unknown help topic %q", strings.Join(args, " "))
			}
			target.InitDefaultHelpFlag()
			return writeHelp(target)
		},
	}
}

func completeHelp(cmd *cobra.Command, args []string, prefix string) ([]cobra.Completion, cobra.ShellCompDirective) {
	target, rest, err := cmd.Root().Find(args)
	if err != nil || len(rest) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var completions []cobra.Completion
	for _, child := range target.Commands() {
		if !child.Hidden && strings.HasPrefix(child.Name(), prefix) {
			completions = append(completions, cobra.CompletionWithDesc(child.Name(), child.Short))
		}
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// writeHelp buffers Cobra's default help because Help discards writer errors.
// The final write lets Fleet report output failures with its exit status.
func writeHelp(cmd *cobra.Command) error {
	var help bytes.Buffer
	out := cmd.OutOrStdout()
	cmd.SetOut(&help)
	(&cobra.Command{}).HelpFunc()(cmd, nil)
	cmd.SetOut(out)

	if _, err := out.Write(help.Bytes()); err != nil {
		return exitstatus.WorkError{Err: err}
	}
	return nil
}
