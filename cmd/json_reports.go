package cmd

import "github.com/spf13/cobra"

func jsonReportsTopic() *cobra.Command {
	return &cobra.Command{
		Use: "json-reports", Short: "Fields and values of the --json reports", Args: cobra.NoArgs,
		Long: `Every command that accepts --json writes one JSON object followed by a
newline to stdout after repository work finishes, including on failure.
Field names are lowercase with underscores. Empty arrays are [], never
null. Optional strings are omitted when absent. Timestamps are RFC 3339
strings; a missing event timestamp is null. Consumers should tolerate
added fields.

Clone and sync report:
  complete           Every selected repository finished without error.
                     Checkouts that sync reports missing do not clear it.
  dry_run            The report describes a preview.
  results            One result per selected repository, sorted by
                     complete identity.

Each result:
  repository, path   Configured complete identity and primary checkout path.
  branch, commit     Target branch and observed commit; empty if unknown.
  status             sync: current, updated, missing, planned, or failed.
                     clone: cloned, present, planned, or failed. A
                     branch-only change is updated.
  actions            Completed actions in execution order, retained after
                     a later failure.
  planned_actions    Intended actions of a dry run; empty otherwise.
  history_unresolved A dry run lacked the Git objects needed to compare
                     target history; planned updates remain conditional.
  error              Failure reason, present only for failed repositories.

Each action has kind and branch. kind is clone, create_branch,
switch_branch, or update. A switch has from and to branch names. An update
has from and to commits. A clone or create_branch has a to commit, which a
clone whose verification failed early can omit.

Issue and PR report:
  query              resource, state, sort, order, and limit in effect.
  complete           Every repository established enough candidates for
                     the requested global ordering. An incomplete report is
                     not the newest or oldest across the selection.
  items              Matching items after deduplication, ordering, and the
                     limit.
  repositories       repository, complete, and optional error for each
                     queried repository.

Each item:
  repository         Configured complete identity.
  number, title, url Item number, title, and canonical URL.
  state              open, closed, or merged.
  state_reason       GitHub's issue state reason verbatim, such as
                     COMPLETED or NOT_PLANNED. Omitted for pull requests.
  created_at,        Creation and update timestamps.
  updated_at
  closed_at,         Event timestamps or null; merged_at is null for
  merged_at          issues and unmerged pull requests.`,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
}
