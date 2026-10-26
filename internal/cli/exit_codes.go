package cli

import "github.com/spf13/cobra"

func exitCodesTopic() *cobra.Command {
	return newHelpTopic("exit-codes", "Exit codes and error categories", `Fleet reserves three exit statuses, split by whether repository work had
started:

  0  Success, including zero matching items, an empty selection, checkouts
     reported missing by sync or present by clone, a dry run with unresolved
     history, config output, help, version, and completion.
  1  Failure after repository work started: a clone, sync, or query report
     contains at least one failed result or incomplete repository, or the
     report itself could not be written. The report was emitted first.
  2  Failure before any repository work: usage, unknown command,
     configuration, selection, or CODE_DIR. Nothing was written to stdout.

Errors at the process boundary use 'fleet: <message>' on stderr. --json
preserves one parseable report on partial failure. Completed changes remain
in place after errors or cancellation; cancellation takes the status of the
phase it interrupted. Dry-run success means inspection completed;
unresolved history still needs checking during sync. 'fleet help <unknown>'
prints an unknown-topic line and the root usage on stderr, leaves stdout
empty, and exits 0.`)
}
