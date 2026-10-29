package cli

// Shared fragments compose command help so repeated contracts cannot drift.
const selectionHelp = `Selection:
  Repository arguments accept repo, org/repo, or server/org/repo. Short names
  must be unique across the full inventory. Each --group takes one exact
  group name; repeat it for several groups. Arguments and groups form a
  deduplicated union. No selection means all repositories. An explicitly
  empty group selects none. Comma-separated selections are not supported.
  Invalid configuration or selection stops before repository work.`

const configHelp = `Configuration:
  Read $XDG_CONFIG_HOME/fleet/config.toml, or ~/.config/fleet/config.toml
  when XDG_CONFIG_HOME is empty or relative.
  A missing discovered file is not an error; Fleet then runs with defaults
  and an empty inventory. --config PATH replaces discovery and must name a
  file that exists. No ancestor search runs.
  Precedence is defaults, TOML, environment, then command flags.
  issues.limit and prs.limit set the default --limit of issues and prs;
  they default to 15. FLEET_ISSUES_LIMIT and FLEET_PRS_LIMIT override
  TOML, and each command's --limit overrides both for one run.
  The effective limits must be positive after applying all layers.
  Inventory and groups come only from TOML. Overlay may compose the file;
  Fleet does not run Overlay or select layers.`

const outputHelp = `Output and dependencies:
  Results go to stdout; diagnostics go to stderr. clone and sync print every
  repository as soon as it finishes, in aligned rows without headers or color:
  Nerd Font icon, repository, result, details, then the branch in parentheses.
  Repository and result columns align; long branches remain in full at the end.
  Codicons mark current/present with a check, updated/cloned with circular
  arrows, planned with an eye, missing with a warning triangle, and failed
  with a circled cross. Result words also identify each outcome.
  Commit hashes use seven characters. Failures retain completed actions and
  full error details, with additional error lines indented. A blank line
  separates rows from the summary, which lists failures and missing checkouts
  first, changes and plans next, and unchanged repositories last.
  --json instead emits one JSON report after all work, including failures
  and completed actions. Success exits 0.
  Partial or incomplete results exit 1 after the report is written.
  Output failures also exit 1 and can leave stdout empty or partial.
  Preflight errors exit 2 and leave stdout empty.
  Commands never prompt, read credentials from stdin, or need an interactive
  terminal. Git 2.36 or newer and gh 2.50 or newer must already be
  authenticated. Each subprocess has a 120-second timeout; Git SSH uses
  batch mode and a 15-second connection timeout.
  Help, version, and shell completion need no configuration or network.
  See 'fleet help exit-codes' for exit status and 'fleet help json-reports'
  for the fields of every --json report and the JSONL and CSV records.`

// dataHelp is a format string; %[1]s is the command's resource.
const dataHelp = `Formats:
  Output is a human-readable list by default, on a terminal and through a
  pipe alike. --json writes one JSON report object. --jsonl writes one JSON
  object per item, each followed by a newline. --csv writes a header row,
  omitted under --no-header, then one row per item. Choose at most one.
  'fleet help json-reports' names every report field, record field, and
  CSV column. Zero matching items print the human summary line and
  "0 items", a JSON report whose items are [], no JSONL output, and only
  the CSV header row, or nothing under --no-header.

Limits and cursors:
  --limit N returns at most N items across the selection. It defaults to
  %[1]s.limit, which is 15 unless configured. --all returns
  every matching item and cannot combine with --limit. When the limit cuts
  the result while more items match, the JSON report sets hasMore and
  nextCursor; human, JSONL, and CSV output print the cursor on stderr. The
  command still exits 0. --cursor VALUE, given a nextCursor, returns the
  items after the ones already returned. It must come from the same command
  with the same --state, --sort, --order, and selection; --limit and --all
  may change between pages.

Failed reads:
  Each repository that cannot be read prints '<repository> error: <message>'
  on stderr, and the command exits 1. Human output lists the items read,
  including those a failed repository returned before failing, and prints
  PARTIAL RESULTS on stderr. The JSON report sets complete to false and
  nextCursor, which reads the failed repositories again from where this
  run started them, without repeating the other repositories' items.
  JSONL and CSV output write no records.

Output file:
  --output-file PATH writes the payload to PATH instead of stdout and
  requires --json, --jsonl, or --csv. The command creates PATH and refuses
  an existing file unless --overwrite, which replaces it; --overwrite
  requires --output-file. The payload is written to a temporary file beside
  PATH and moved into place only after a complete read, so a failed run
  leaves PATH as it was.`

const queryHelp = `Ordering:
  Open items default to created descending; closed items to closed
  descending; merged PRs to merged descending; all states to updated
  descending. --sort closed requires --state closed. --sort merged requires
  prs --state merged. --order accepts asc or desc. Equal timestamps sort
  by complete repository identity, then item number, ascending.
  The limit applies globally after URL deduplication. Closed PRs include
  merged and unmerged outcomes. Closed issues retain GitHub's stateReason.
  Closure and merge recency use event timestamps, with no age window.

Retrieval:
  Query authenticated gh issue list or gh pr list on each repository's
  host, with at most four repositories in flight. GitHub.com and GitHub
  Enterprise are supported; other hosts fail explicitly. No CODE_DIR or
  local checkout is needed. Created descending uses ordinary GraphQL
  listing; other retrieval orders use gh list's GraphQL search path.
  Expand 100-item prefixes until enough candidates establish the requested
  ordering. For newest closure or merge ordering, updatedAt bounds unseen
  event times. Continue through ties. Oldest event ordering requires
  exhausting the candidates. Ordinary GraphQL listing has no 1000-item
  cap. At 1000 search candidates, switch to ordinary created-descending
  listing and read the entire repository before sorting. This fallback
  can take longer, even with a small --limit. --all reads every matching
  item; --cursor resumes after the previous page.
  Failed repositories do not stop other queries. Partial output does not
  claim to be the newest or oldest across the selection.`
