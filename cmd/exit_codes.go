package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

// Exit statuses by phase. Preflight covers everything before repository
// work starts; work covers failures whose report has already been written.
const (
	ExitSuccess   = 0
	ExitWork      = 1
	ExitPreflight = 2
)

// workError marks a failure raised after repository work started, so the
// process exits ExitWork rather than ExitPreflight.
type workError struct {
	err error
}

func (e workError) Error() string { return e.err.Error() }

func (e workError) Unwrap() error { return e.err }

// ExitCode maps an error returned by the root command to a process exit
// status.
func ExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}
	var afterWork workError
	if errors.As(err, &afterWork) {
		return ExitWork
	}
	return ExitPreflight
}

func exitCodesTopic() *cobra.Command {
	return &cobra.Command{
		Use: "exit-codes", Short: "Exit codes and error categories", Args: cobra.NoArgs,
		Long: `Fleet reserves three exit statuses, split by whether repository work had
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
prints the root usage and exits 0.`,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
}
