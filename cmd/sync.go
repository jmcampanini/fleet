package cmd

import (
	"github.com/jmcampanini/fleet/internal/checkout"
	"github.com/jmcampanini/fleet/internal/process"
	"github.com/spf13/cobra"
)

func newSync() *cobra.Command {
	var selection selection
	var dryRun, jsonOutput bool
	command := &cobra.Command{
		Use: "sync [repository ...]", Short: "Fast-forward existing checkouts to their target branches",
		Long: `Sync primary checkouts at $CODE_DIR/server/org/repo sequentially in
complete-identity order. CODE_DIR must be an absolute, existing, accessible
directory. The shell must expand $HOME; Fleet does not infer a root.

Sync never clones. A missing or empty destination is reported with status
missing and a warning on stderr naming 'fleet clone'; it does not fail the
command or change the exit status. Rerunning sync is safe: a checkout
already on its target commit reports current with no actions.

Use a configured branch override or ask origin for its current default.
Require the target on origin before changing local state. Validate checkout
root and origin identity; SSH and HTTPS origins are accepted. Reject dirty
files, untracked files, dirty submodules, unfinished Git operations,
detached HEAD, and target branches occupied by another worktree. Check
target history before switching; reject ahead or diverged target commits.
Unpushed feature-branch commits remain on their branch and do not block a
clean switch. Create a missing target branch with origin tracking.

Directories that are not the configured checkout, symbolic links below
CODE_DIR, and additional worktrees are rejected. Fleet never stashes,
resets, cleans, rebases, pushes, creates merge commits, or overwrites
ignored files. Hooks, automatic maintenance, and recursive submodule updates
are disabled.

--dry-run queries remote refs and checks local state without fetching or
changing checkouts, refs, or configuration. Missing objects leave the history
check unresolved until sync. Planned actions are separate from completed
actions. A successful preview does not guarantee that sync will succeed.

Verify origin, branch, commit, and cleanliness after changes. Report branch
creation, switches, and updates separately. Continue after repository
failures; completed actions remain in place and appear in the final report.
The configuration and selection are loaded once, even if a synced repository
contains Overlay sources.

` + selectionHelp + "\n\n" + configHelp + "\n\n" + outputHelp,
		Example: `  fleet sync
  fleet sync gibson molly --group clis --group tools
  fleet sync --group personal-agent --dry-run --json`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, refs []string) error {
			client := checkout.Client{Run: process.Execute}
			return runCheckout(cmd, checkoutRun{dryRun: dryRun, jsonOutput: jsonOutput, refs: refs, selection: selection, step: client.Sync})
		},
	}
	selection.bind(command.Flags())
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Inspect and plan without changing local state")
	command.Flags().BoolVar(&jsonOutput, "json", false, "Emit a JSON report")
	return command
}
