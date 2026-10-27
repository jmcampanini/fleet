package cli

import (
	"fmt"
	"strings"

	"github.com/jmcampanini/fleet/internal/config"
	"github.com/jmcampanini/go-config-loader/configreporter"
	"github.com/spf13/cobra"
)

func newConfig() *cobra.Command {
	var provenance bool
	command := &cobra.Command{
		Use: "config", Short: "Print effective TOML and optional field provenance",
		Long: `Print the effective configuration as TOML on stdout after applying every
layer and validating the inventory, in a form --config can load again.
--provenance adds each field's source as TOML comments. The generated file
is the source; the Overlay layers that produced it cannot be recovered.
Diagnostics go to stderr. This command never prompts, needs no terminal,
and runs no Git or GitHub operations. Success exits 0; a configuration
error exits 2 with empty stdout.

Keys:
  issues.limit
    Default --limit of issues: the maximum total issues returned across
    the selection; a positive integer.
  prs.limit
    Default --limit of prs: the maximum total pull requests returned
    across the selection; a positive integer.
  repos."server/org/repo"
    One table per repository. The key is the repository identity:
    exactly server/org/repo with no URL scheme. Each component uses
    ASCII letters, digits, '.', '_', and '-' only, and is never . or ..;
    the server starts with a letter or digit. An empty table is valid
    and means the remote default branch.
  repos."server/org/repo".branch
    Optional branch override for clone and sync. Empty or absent means
    the remote default branch. A nonempty value must be a valid Git
    branch name.
  groups.<name>
    Array of repository references: repo, org/repo, or server/org/repo.
    Each reference must match exactly one configured repository; an
    ambiguous short name fails. Groups hold repositories only, never
    other groups. An empty array is valid. A group name is
    nonempty and contains no comma, tab, line break, or NUL.

Unknown keys fail the load. An empty inventory is valid.

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

` + configHelp,
		Example: `  fleet config
  FLEET_ISSUES_LIMIT=30 fleet config --config ./config.toml --provenance`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := configPath(cmd)
			if err != nil {
				return err
			}
			loaded, err := config.Load(path)
			if err != nil {
				return err
			}

			reporter := configreporter.New(loaded.Config, loaded.Report)
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
