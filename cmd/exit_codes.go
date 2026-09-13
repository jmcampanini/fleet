package cmd

import "github.com/spf13/cobra"

func exitCodesTopic() *cobra.Command {
	return &cobra.Command{
		Use: "exit-codes", Short: "Exit codes and error categories", Args: cobra.NoArgs,
		Long: `Fleet follows Grove and Gibson's sync script with two exit codes:

  0  Success, including zero matching items, an empty selection, checkouts
     reported missing by sync or present by clone, a dry run with unresolved
     history, config output, help, version, and completion.
  1  Any error: usage, configuration, selection, CODE_DIR, Git, GitHub,
     incomplete queries, cancellation, or output I/O.

Errors at the process boundary use 'fleet: <message>' on stderr. Preflight
errors produce no stdout payload. After repository work starts, clone, sync,
and query reports include individual errors and return 1 if any result
failed or was incomplete. --json preserves one parseable report on partial
failure. Completed changes remain in place after errors or cancellation.
Dry-run success means inspection completed; unresolved history still needs
checking during sync. 'fleet help <unknown>' prints the root usage and exits
0. No numeric exit code is reserved for a particular error type.`,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
}
