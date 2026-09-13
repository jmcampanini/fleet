package cmd

// Shared fragments compose command help so repeated contracts cannot drift.
const selectionHelp = `Selection:
  Repository arguments accept repo, org/repo, or server/org/repo. Short names
  must be unique across the full inventory. Each --group takes one exact
  group name; repeat it for several groups. Arguments and groups form a
  deduplicated union. No selection means all repositories. An explicitly
  empty group selects none. Comma-separated selections are not supported.
  Invalid configuration or selection stops before repository work.`

const configHelp = `Configuration:
  Read $XDG_CONFIG_HOME/fleet/fleet.toml, or ~/.config/fleet/fleet.toml when
  XDG_CONFIG_HOME is empty. Relative XDG_CONFIG_HOME follows go-config-loader
  and currently resolves against the working directory. --config PATH
  replaces discovery; the selected file must exist. No ancestor search runs.
  Precedence is defaults, TOML, environment, then flags. Limits default to
  15 and must be positive. FLEET_ISSUES_LIMIT and FLEET_PRS_LIMIT override
  TOML; --issues-limit and --prs-limit override the environment.
  Inventory and groups come only from TOML. Overlay may compose the file;
  Fleet does not run Overlay or discover profiles.`

const outputHelp = `Output and dependencies:
  Results go to stdout; diagnostics go to stderr. --json emits one JSON
  report, including failures and completed actions. Partial or incomplete
  results exit 1; success exits 0. Preflight errors leave stdout empty.
  Commands do not prompt or read credentials from stdin. Git and gh must
  already be authenticated. Each subprocess has a 120-second timeout;
  Git SSH uses batch mode and a 15-second connection timeout.
  Help, version, and shell completion need no configuration or network.
  See 'fleet exit-codes' and docs/automation.md for the automation contract.`

const queryHelp = `Ordering:
  Open items default to created descending; closed items to closed
  descending; merged PRs to merged descending; all states to updated
  descending. --sort closed requires --state closed. --sort merged requires
  prs --state merged. --order accepts asc or desc. Equal timestamps sort
  by complete repository identity, then item number, ascending.
  Limits apply globally after URL deduplication. Closed PRs include merged
  and unmerged outcomes. Closed issues retain GitHub's stateReason.
  Closure and merge recency use event timestamps, with no age window.

Retrieval:
  Query authenticated gh issue list or gh pr list on each repository's
  host, with at most four repositories in flight. GitHub.com and GitHub
  Enterprise are supported; other hosts fail explicitly. No CODE_DIR or
  local checkout is needed. Created descending uses ordinary GraphQL
  listing; other retrieval orders use gh list's GraphQL search path.
  Expand 100-item prefixes up to 1000 candidates per repository. For newest
  closure or merge ordering, updatedAt bounds unseen event times. Continue
  through ties. Oldest event ordering requires exhausting the candidates.
  Reaching the cap without proving completeness produces a partial report
  and exit 1. Failed repositories do not stop other queries. Partial output
  does not claim to be the newest or oldest across the selection.`
