# Automation contract

Fleet uses Grove and Gibson's sync script as the exit-code precedent. Success exits `0`. Any error exits `1`, including invalid arguments, configuration, selection, inaccessible `CODE_DIR`, repository failures, incomplete queries, cancellation, and output errors. An empty selection succeeds without requiring `CODE_DIR`. A dry run with unresolved history also succeeds; it has completed inspection but has not established whether an update can fast-forward.

Issue 1 leaves the initial JSON fields and exit-code categories open. This contract resolves them using the existing CLI conventions. JSON uses lowercase field names with underscores for Fleet-owned compound names, as Grove does for its stored records. GitHub timestamp and state-reason fields are mapped into those names.

The CLI supports macOS and Linux. Git and GitHub CLI (`gh`) must be available on `PATH` and authenticated without prompting. Git commands use a 120-second deadline and a 15-second SSH connection timeout. Cancellation terminates the subprocess group, including transport helpers. Fleet does not invoke `gh search` or read credentials from stdin.

## Streams and failures

`sync`, `issues`, and `prs` accept `--json`. Each writes one JSON object followed by a newline to stdout after repository work finishes. Diagnostics go to stderr. Output has no color or terminal control sequences; JSON encodes control characters inside strings. Human-readable titles, paths, and error details are quoted to prevent terminal control characters from being interpreted.

Preflight failures leave stdout empty and report `fleet: <message>` on stderr. These include configuration and selection failures, query usage errors, and invalid `CODE_DIR` for a nonempty sync selection. Scripts should check both the exit code and whether a report exists.

Once repository work begins, an individual failure does not prevent the rest of the selection from being attempted. The final JSON object includes failures and completed actions and remains valid when the command exits `1`. Output I/O errors or forced process termination can prevent delivery of a final report. Changes already completed remain in place.

Empty arrays are `[]`, never `null`. Optional error strings and action `from`/`to` fields are omitted when absent. Timestamps are RFC 3339 strings. Missing event timestamps are `null`. Consumers should tolerate added fields; the fields and meanings below are the initial contract.

## Sync report

```json
{
  "complete": true,
  "dry_run": false,
  "results": [
    {
      "repository": "github.com/example/service",
      "path": "/home/user/Code/github.com/example/service",
      "branch": "main",
      "commit": "0123456789abcdef0123456789abcdef01234567",
      "status": "updated",
      "actions": [
        {
          "kind": "switch_branch",
          "branch": "main",
          "from": "feature",
          "to": "main"
        }
      ],
      "planned_actions": [],
      "history_unresolved": false
    }
  ]
}
```

| Field | Meaning |
| --- | --- |
| `complete` | Every selected repository completed inspection or sync without error. |
| `dry_run` | This report describes a preview. |
| `results` | One result per selected repository, sorted by complete identity. |
| `repository`, `path` | Configured complete identity and primary checkout path. |
| `branch`, `commit` | Target branch and observed remote/fetched commit; empty strings if unknown. |
| `status` | `cloned`, `updated`, `current`, `planned`, or `failed`. A branch-only change is `updated`. |
| `actions` | Completed actions in execution order, retained after later failure. |
| `planned_actions` | Intended actions during a dry run. Empty during a real sync. |
| `history_unresolved` | A dry run lacked Git objects needed to compare target history. Planned updates remain conditional. |
| `error` | Failure reason, present only for failed repositories. |

Action `kind` is `clone`, `create_branch`, `switch_branch`, or `update`. Each action has `branch`. For a switch, `from` and `to` name branches. For an update, they contain commit IDs. A clone or branch creation has a `to` commit. A completed clone whose verification fails before its commit can be read can omit `to`.

A failed clone can leave a partial destination. Its error identifies the path for inspection. A failure creating parent directories can leave those directories. Fleet does not remove them. Fetching can change remote-tracking refs even if a later target-history check fails; dry runs never fetch or change refs.

## Issue and PR report

```json
{
  "query": {
    "resource": "issues",
    "state": "closed",
    "sort": "closed",
    "order": "desc",
    "limit": 15
  },
  "complete": false,
  "items": [],
  "repositories": [
    {
      "repository": "github.com/example/service",
      "complete": false,
      "error": "query could not be completed"
    }
  ]
}
```

`query` records the effective controls. `resource` is `issues` or `prs`. `state` is `open`, `closed`, or `all`; PRs also accept `merged`. `sort` is `created`, `updated`, or `closed`; PRs also accept `merged`. `order` is `asc` or `desc`, and `limit` is a positive global maximum.

`complete` means every repository established a sufficient candidate set for the requested global ordering. Each `repositories` entry identifies the repository, its `complete` status, and an optional `error`. Failed later retrievals can retain earlier candidates, but those candidates remain explicitly incomplete. Reports with `complete: false` must not be described as the newest or oldest across the whole selection.

Each item has these fields:

| Field | Meaning |
| --- | --- |
| `repository` | Configured complete identity. |
| `number` | Issue or PR number. |
| `title`, `url` | Item title and canonical URL. |
| `state` | Lowercase GitHub state, `open`, `closed`, or `merged`. |
| `state_reason` | GitHub's issue state reason, retained verbatim when provided, such as `COMPLETED`, `NOT_PLANNED`, or `DUPLICATE`. Omitted for PRs. |
| `created_at`, `updated_at` | Creation and update timestamps. |
| `closed_at`, `merged_at` | Event timestamps or `null`; `merged_at` is `null` for issues and unmerged PRs. |

Fleet combines candidates, deduplicates URLs, orders globally, and applies the limit. Equal timestamps sort by complete identity, then item number, ascending, for both timestamp directions.

## Retrieval completeness

Fleet runs at most four repository queries at a time. It uses `gh issue list` and `gh pr list`, which return GraphQL results. Created-descending queries use the ordinary repository connection. Other retrieval orders use the list command's `--search sort:...` option, which uses GraphQL search and its 1,000-result ceiling.

The list commands expose a total limit, without a cursor. Fleet increases that limit in 100-item steps, up to 1,000 candidates per repository. This repeats earlier pages; one repository can require up to 55 underlying 100-item page requests across ten list invocations. Fleet keeps only the best N candidates after each response. A response shorter than the requested prefix establishes exhaustion. The cap produces an incomplete result unless ordering was already proved.

For newest closure or merge queries, retrieval uses descending `updatedAt`. An unseen item's event cannot be later than its update. Once the Nth-best event is strictly later than the final retrieved `updatedAt`, unseen items cannot displace the current N. Strict comparison continues through equal timestamps so item-number ties are also resolved. Oldest closure/merge ordering requires exhaustion because an upper bound on event time cannot prove the oldest items.

This cap is explicit for every retrieval order. Reduce the requested limit or query the repository directly when Fleet reports incomplete results. Narrowing selection reduces queries but does not remove the per-repository cap. As with GitHub CLI itself, results reflect remote responses during the query, not a transactional snapshot across repositories.

## Configuration reporting

`fleet config` emits effective, redirectable TOML. `--provenance` adds TOML comment lines containing field path, effective value, and source. Sources come from go-config-loader, including absolute configuration paths, `<default>`, `<env>`, and `<pflag>`. It cannot reconstruct Overlay profile layers from a generated TOML file.

`--issues-limit` and `--prs-limit` are persistent root flags, so they are available before or after the command and appear in config provenance. Configuration inspection performs no Git or GitHub operations.
