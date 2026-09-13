package cmd

import (
	"fmt"

	"github.com/jmcampanini/fleet/internal/process"
	reposync "github.com/jmcampanini/fleet/internal/sync"
	"github.com/spf13/cobra"
)

func newClone() *cobra.Command {
	var groups []string
	var all, dryRun, jsonOutput bool
	command := &cobra.Command{
		Use: "clone [repository ...]", Short: "Clone missing checkouts into CODE_DIR",
		Long: `Clone primary checkouts into $CODE_DIR/server/org/repo sequentially in
complete-identity order. CODE_DIR must be an absolute, existing, accessible
directory. The shell must expand $HOME; Fleet does not infer a root.

Selection is explicit: pass --all, repository arguments, or --group. Unlike
sync, issues, and prs, an empty selection is a usage error rather than the
whole inventory, and --all cannot be combined with arguments or groups. The
other selection rules below still apply.

Clone uses SSH (git@server:org/repo.git) and checks out the configured
branch override or the remote default branch, which must exist on origin.
Fleet creates parent directories and clones into a missing or empty
destination. Rerunning clone is safe: a destination that already holds a
primary checkout whose origin identifies the configured repository reports
present with no actions and is never modified, even if it is dirty or on
another branch. Any other content at the destination fails that repository.
Failed clones may leave a partial destination; inspect it before rerunning.
Hooks and recursive submodule updates are disabled. Clone never updates an
existing checkout; use 'fleet sync' for that.

--dry-run queries remote refs and checks the destination without creating
directories or cloning. Continue after repository failures; completed
clones remain in place and appear in the final report.

` + selectionHelp + "\n\n" + configHelp + "\n\n" + outputHelp,
		Example: `  fleet clone --all
  fleet clone gibson molly --group clis
  fleet clone --group personal-agent --dry-run --json`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, refs []string) error {
			if all && (len(refs) > 0 || len(groups) > 0) {
				return fmt.Errorf("--all cannot be combined with repository arguments or --group")
			}
			if !all && len(refs) == 0 && len(groups) == 0 {
				return fmt.Errorf("clone requires --all, repository arguments, or --group")
			}

			client := reposync.Client{Run: process.Execute}
			return runCheckout(cmd, checkoutRun{dryRun: dryRun, groups: groups, jsonOutput: jsonOutput, refs: refs, step: client.Clone})
		},
	}
	command.Flags().BoolVar(&all, "all", false, "Select every configured repository")
	command.Flags().StringArrayVar(&groups, "group", nil, "Select one group; repeat for several groups")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Inspect and plan without creating anything")
	command.Flags().BoolVar(&jsonOutput, "json", false, "Emit a JSON report")
	return command
}
