package cli

import "github.com/spf13/cobra"

func jsonReportsTopic() *cobra.Command {
	return newHelpTopic("json-reports", "Fields of the --json reports and the JSONL and CSV records", `Every command that accepts --json writes one JSON object followed by a
newline to stdout, or to the --output-file file of issues and prs.
Configuration and selection errors leave stdout empty. Repository commands
report after work finishes, including on failure, except that issues and
prs write no --output-file file after a failed read. The repos, issues,
and prs reports carry no error text; their failures appear on stderr.
Field names are lowercase with underscores, except hasMore and nextCursor,
which keep the camelCase names every data workflow command shares. Empty
arrays are [], never null.
Optional strings are omitted when absent. Timestamps are RFC 3339 strings;
a missing event timestamp is null. Consumers should tolerate added fields.

Group report:
  groups             Selected groups, sorted alphabetically by name.

Each group:
  name               Configured group name.
  repositories       Unique server/org/repo strings, sorted by complete
                     identity. Shared repositories appear in every selected
                     group they belong to. Empty groups have [].

No configured groups produces {"groups":[]}.

Clone and sync report:
  complete           Every selected repository finished without error.
                     Checkouts that sync reports missing do not clear it.
  dry_run            The report describes a preview.
  results            One result per selected repository, sorted by
                     complete identity.

Each result carries repository, path, branch, commit, status, actions,
planned_actions, history_unresolved, and error. branch and commit are the
target branch and observed commit, empty if unknown. status is one of
current, updated, missing, planned, or failed for sync, and cloned,
present, planned, or failed for clone; a branch-only change is updated.
actions lists completed actions in execution order and is retained after
a later failure; planned_actions lists the intended actions of a dry run.
history_unresolved means a dry run lacked the Git objects needed to
compare target history, so planned updates remain conditional. error is
present only for failed repositories.

Each action has kind and branch. kind is clone, create_branch,
switch_branch, or update. A switch has from and to branch names. An update
has from and to commits. A clone or create_branch has a to commit, which a
clone whose verification failed early can omit.

Repos report:
  complete           Every selected repository was described without a
                     topic retrieval error.
  topics_fetched     Topics were requested from GitHub; false under
                     --no-topics, where every topic list is empty.
  results            One entry per selected repository, sorted by
                     complete identity.

Each entry carries repository, path, branch, groups, and topics. path is
the expected checkout under CODE_DIR whether or not it exists. branch is
the configured override, empty when the remote default applies. groups
lists the configured group names containing the repository, sorted.
topics lists the repository's GitHub topics, sorted, and is empty when they
were not fetched or the fetch failed. A failed fetch prints
'<repository> error: <message>' on stderr instead.

Issue and PR report:
  query              resource, state, sort, order, limit, and all in
                     effect. limit is an integer, 0 when all is true.
  complete           Every repository was read and established enough
                     candidates for the requested global ordering. false
                     marks a failed read: the failed repositories' items
                     may be missing, and the report is not the newest or
                     oldest across the selection. A limit cut leaves it
                     true.
  hasMore            true when the limit cut the items while more match.
  nextCursor         String to pass to --cursor for the next items. Present
                     when hasMore is true or complete is false.
  items              Matching items after deduplication, ordering, and the
                     limit.
  repositories       repository and complete for each queried repository.

Items:
  Each JSON item, JSONL line, and CSV row carries these fields, in this
  CSV column order. Fleet reads them from gh issue list --json or gh pr
  list --json, whose fields follow GitHub's GraphQL Issue and PullRequest
  objects. number, title, and url pass through unchanged; Fleet renames
  stateReason and the camelCase timestamps, lowercases state, and adds
  repository.
  repository         String: Fleet's server/org/repo identity.
  number             Integer: gh number.
  title              String: gh title.
  url                String: gh url.
  state              String: gh state lowercased, one of open, closed, or
                     merged.
  state_reason       String: gh stateReason verbatim, such as COMPLETED or
                     NOT_PLANNED. Omitted from JSON items and JSONL lines
                     when empty, which it always is for pull requests.
  created_at         Timestamp: gh createdAt.
  updated_at         Timestamp: gh updatedAt.
  closed_at          Timestamp or null: gh closedAt.
  merged_at          Timestamp or null: gh mergedAt; null for issues and
                     unmerged pull requests.

CSV output is RFC 4180 with a header row naming the columns above, unless
--no-header. Integers are decimal, timestamps are RFC 3339, and an empty
string or a null is an empty cell.`)
}
