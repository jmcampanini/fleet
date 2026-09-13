package cmd

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
			loaded, err := config.Load(path, cmd.Root().PersistentFlags())
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
