// Package cmd connects Fleet's inventory and repository operations to Cobra.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jmcampanini/fleet/internal/config"
	"github.com/jmcampanini/fleet/internal/process"
	"github.com/jmcampanini/fleet/internal/query"
	reposync "github.com/jmcampanini/fleet/internal/sync"
	"github.com/jmcampanini/go-config-loader/configreporter"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/spf13/cobra"
)

// Version is injected by the versioned build target.
var Version = "dev"

// NewRoot constructs a fresh command tree without performing command work.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use: "fleet", Short: "Sync repositories and list their issues and pull requests",
		Long: `Fleet keeps configured primary Git checkouts on their intended branches
and lists issues and pull requests across the same inventory.

` + configHelp + "\n\n" + selectionHelp + "\n\n" + outputHelp,
		Example: `  fleet sync gibson molly --group clis --dry-run
  fleet issues --issues-limit 30
  fleet prs --state merged
  fleet config --provenance`,
		Args: cobra.NoArgs, SilenceErrors: true, SilenceUsage: true, Version: Version,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	root.PersistentFlags().String("config", "", "Read this required TOML file instead of the default")
	if err := pflagloader.Register[config.Config](root.PersistentFlags()); err != nil {
		// Registration depends only on this package's static configuration type.
		panic(err)
	}
	root.AddCommand(newConfig(), newSync(), newQuery("issues"), newQuery("prs"), exitCodesTopic())
	return root
}

func configPath(cmd *cobra.Command) (string, error) {
	path, err := cmd.Flags().GetString("config")
	if err != nil {
		return "", err
	}
	if cmd.Flags().Changed("config") && path == "" {
		return "", fmt.Errorf("--config requires a nonempty file path")
	}
	return path, nil
}

func newConfig() *cobra.Command {
	var provenance bool
	command := &cobra.Command{
		Use: "config", Short: "Print effective TOML and optional field provenance",
		Long: `Print redirectable TOML after applying all configuration layers and
validating the inventory. --provenance adds field sources as TOML comments.
The generated file is the source; individual Overlay layers cannot be
recovered. This command performs no Git or GitHub operations.

Example configuration:
  [issues]
  limit = 15
  [prs]
  limit = 15
  [repos."github.com/example/service"]
  branch = "develop"
  [repos."github.com/example/helper"]
  [groups]
  tools = ["service", "helper"]

Identities contain exactly server/org/repo without schemes or traversal.
Unknown fields and unresolved group members fail. Empty repository tables
use the remote default branch. Empty inventories are valid. Groups contain
repository references, never other groups. Overlay owns additive profile
composition; a later file can override branch and limit settings.

` + configHelp,
		Example: `  fleet config
  fleet config --config ./fleet.toml --issues-limit 30 --provenance`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := configPath(cmd)
			if err != nil {
				return err
			}
			cfg, report, _, err := config.Load(path, cmd.Root().PersistentFlags())
			if err != nil {
				return err
			}
			reporter := configreporter.New(cfg, report)
			if err := reporter.WriteTOML(cmd.OutOrStdout()); err != nil {
				return err
			}
			if provenance {
				for _, row := range reporter.ProvenanceRows() {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "# %s\n", strings.Join(quoted(row), " | ")); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
	command.Flags().BoolVar(&provenance, "provenance", false, "Include field sources as TOML comments")
	return command
}

func quoted(values []string) []string {
	var fields []string
	for _, value := range values {
		fields = append(fields, fmt.Sprintf("%q", value))
	}
	return fields
}

func newSync() *cobra.Command {
	var groups []string
	var dryRun, jsonOutput bool
	command := &cobra.Command{
		Use: "sync [repository ...]", Short: "Clone missing checkouts and fast-forward target branches",
		Long: `Sync primary checkouts at $CODE_DIR/server/org/repo sequentially in
complete-identity order. CODE_DIR must be an absolute, existing, accessible
directory. The shell must expand $HOME; Fleet does not infer a root.

Use a configured branch override or ask origin for its current default.
Require the target on origin before changing local state. Validate checkout
root and origin identity; SSH and HTTPS origins are accepted. Reject dirty
files, untracked files, dirty submodules, unfinished Git operations,
detached HEAD, and target branches occupied by another worktree. Check
target history before switching; reject ahead or diverged target commits.
Unpushed feature-branch commits remain on their branch and do not block a
clean switch. Create a missing target branch with origin tracking.

Fleet can create parent directories and clone into a missing or empty
destination. Other contents, symbolic links below CODE_DIR, and additional
worktrees are rejected. Failed clones may leave a partial destination;
inspect it before rerunning. Fleet never stashes, resets, cleans, rebases,
pushes, creates merge commits, or overwrites ignored files. Hooks, automatic
maintenance, and recursive submodule updates are disabled.

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
			path, err := configPath(cmd)
			if err != nil {
				return err
			}
			_, _, inv, err := config.Load(path, cmd.Root().PersistentFlags())
			if err != nil {
				return err
			}
			repos, err := inv.Select(refs, groups)
			if err != nil {
				return err
			}
			root := os.Getenv("CODE_DIR")
			if len(repos) == 0 {
				if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "No repositories selected."); err != nil {
					return err
				}
			} else if err := reposync.ValidateRoot(root); err != nil {
				return err
			}
			report := syncReport{Complete: true, DryRun: dryRun, Results: []reposync.Result{}}
			client := reposync.Client{Run: process.Execute}
			for _, repo := range repos {
				result := client.Sync(cmd.Context(), root, repo, dryRun)
				report.Results = append(report.Results, result)
				report.Complete = report.Complete && result.Error == ""
			}
			if err := renderSync(cmd, report, jsonOutput); err != nil {
				return err
			}
			if !report.Complete {
				return fmt.Errorf("one or more repositories failed; see the sync report")
			}
			return nil
		},
	}
	command.Flags().StringArrayVar(&groups, "group", nil, "Select one group; repeat for several groups")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Inspect and plan without changing local state")
	command.Flags().BoolVar(&jsonOutput, "json", false, "Emit a JSON report")
	return command
}

type syncReport struct {
	Complete bool              `json:"complete"`
	DryRun   bool              `json:"dry_run"`
	Results  []reposync.Result `json:"results"`
}

func renderSync(cmd *cobra.Command, report syncReport, jsonOutput bool) error {
	if jsonOutput {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
	}
	var out strings.Builder
	for _, result := range report.Results {
		fmt.Fprintf(&out, "%s %s path=%q branch=%q commit=%s\n", result.Status, result.Repository, result.Path, result.Branch, result.Commit)
		for _, action := range result.Actions {
			fmt.Fprintf(&out, "  completed %s branch=%q from=%q to=%q\n", action.Kind, action.Branch, action.From, action.To)
		}
		for _, action := range result.PlannedActions {
			fmt.Fprintf(&out, "  planned %s branch=%q from=%q to=%q\n", action.Kind, action.Branch, action.From, action.To)
		}
		if result.HistoryUnresolved {
			fmt.Fprintln(&out, "  history check unresolved; sync must fetch objects before checking")
		}
		if result.Error != "" {
			fmt.Fprintf(&out, "  error: %q\n", result.Error)
		}
	}
	fmt.Fprintf(&out, "repositories=%d complete=%t dry_run=%t\n", len(report.Results), report.Complete, report.DryRun)
	_, err := fmt.Fprint(cmd.OutOrStdout(), out.String())
	return err
}

func newQuery(resource string) *cobra.Command {
	var groups []string
	var jsonOutput bool
	options := query.Options{Resource: resource, State: "open", Order: "desc", Limit: 15}
	command := &cobra.Command{
		Use: resource + " [repository ...]", Short: "List " + resource + " across the selected repositories",
		Long: `List remote items across the configured selection. Issues exclude pull
requests; prs includes only pull requests. Zero matching items succeeds.

` + queryHelp + "\n\n" + selectionHelp + "\n\n" + configHelp + "\n\n" + outputHelp,
		Example: `  fleet issues gibson --group clis --issues-limit 30
  fleet issues --state closed
  fleet prs --state merged --prs-limit 30
  fleet prs --state all --sort updated --order asc --json`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, refs []string) error {
			if err := options.Validate(); err != nil {
				return err
			}
			path, err := configPath(cmd)
			if err != nil {
				return err
			}
			cfg, _, inv, err := config.Load(path, cmd.Root().PersistentFlags())
			if err != nil {
				return err
			}
			repos, err := inv.Select(refs, groups)
			if err != nil {
				return err
			}
			options.Limit = cfg.Issues.Limit
			if resource == "prs" {
				options.Limit = cfg.PRs.Limit
			}
			if len(repos) == 0 {
				if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "No repositories selected."); err != nil {
					return err
				}
			}
			report := (query.Client{Run: process.Execute}).List(cmd.Context(), repos, options)
			if err := renderQuery(cmd, report, jsonOutput); err != nil {
				return err
			}
			if !report.Complete {
				return fmt.Errorf("partial query results; see repository errors in the report")
			}
			return nil
		},
	}
	command.Flags().StringArrayVar(&groups, "group", nil, "Select one group; repeat for several groups")
	command.Flags().StringVar(&options.State, "state", "open", "State: open, closed, all; prs also accepts merged")
	command.Flags().StringVar(&options.Sort, "sort", "", "Sort: created, updated, closed; prs also accepts merged")
	command.Flags().StringVar(&options.Order, "order", "desc", "Timestamp order: asc or desc")
	command.Flags().BoolVar(&jsonOutput, "json", false, "Emit a JSON report")
	return command
}

func renderQuery(cmd *cobra.Command, report query.Report, jsonOutput bool) error {
	if jsonOutput {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
	}
	var out strings.Builder
	if !report.Complete {
		fmt.Fprintln(&out, "PARTIAL RESULTS: ordering across the full selection is not established.")
	}
	fmt.Fprintf(&out, "%s state=%s sort=%s order=%s limit=%d complete=%t\n", report.Query.Resource, report.Query.State, report.Query.Sort, report.Query.Order, report.Query.Limit, report.Complete)
	for _, item := range report.Items {
		fmt.Fprintf(&out, "%s#%d %q state=%s state_reason=%q %s=%s\n  %q\n", item.Repository, item.Number, item.Title, item.State, item.StateReason, report.Query.Sort, item.Timestamp(report.Query.Sort).Format(time.RFC3339), item.URL)
	}
	for _, result := range report.Repositories {
		if result.Error != "" {
			fmt.Fprintf(&out, "%s error: %q\n", result.Repository, result.Error)
		}
	}
	fmt.Fprintf(&out, "%d items\n", len(report.Items))
	_, err := fmt.Fprint(cmd.OutOrStdout(), out.String())
	return err
}
