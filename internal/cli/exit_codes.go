package cli

import "github.com/spf13/cobra"

func exitCodesTopic() *cobra.Command {
	return newHelpTopic("exit-codes", "Exit codes and error categories", `Fleet uses three exit statuses:

  0  Success, including zero matching items, an empty selection, checkouts
     reported missing by sync or present by clone, a dry run with unresolved
     history, config output, help, version, and completion.
  1  Failure after work started: a clone, sync, repos, or query
     report contains at least one failed result or incomplete repository,
     or output could not be written. Failed repository reads retain their
     human or JSON report on stdout; issues and prs write no JSONL or CSV
     records and leave an --output-file file unchanged. Output failures
     can leave stdout empty or partially written.
  2  Failure before any repository work: usage, unknown command,
     configuration, selection, CODE_DIR, an invalid --cursor, or an existing
     --output-file file without --overwrite. Nothing was written to stdout.

Errors at the process boundary use one 'fleet: <message>' line on stderr,
without usage text. repos, issues, and prs first print one
'<repository> error: <message>' line per failed repository. --json on
stdout preserves one parseable report on partial failure. Completed
changes remain in place after errors or cancellation; cancellation takes
the status of the phase it interrupted. Dry-run success means inspection
completed; unresolved history still needs checking during sync.
'fleet help <unknown>' and 'fleet completion <unknown>' are usage errors
and exit 2.`)
}
