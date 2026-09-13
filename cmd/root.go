// Package cmd connects Fleet's inventory and repository operations to Cobra.
package cmd

import (
	"fmt"

	"github.com/jmcampanini/fleet/internal/config"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/spf13/cobra"
)

// Version is injected by the versioned build target.
var Version = "dev"

// NewRoot constructs a fresh command tree without performing command work.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use: "fleet", Short: "Clone and sync repositories and list their issues and pull requests",
		Long: `Fleet clones configured primary Git checkouts, keeps them on their intended
branches, and lists issues and pull requests across the same inventory.
clone and sync are separate commands: sync never clones, and clone never
updates an existing checkout.

` + configHelp + "\n\n" + selectionHelp + "\n\n" + outputHelp,
		Example: `  fleet clone --all
  fleet sync gibson molly --group clis --dry-run
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
	root.AddCommand(newConfig(), newClone(), newSync(), newIssues(), newPRs(), exitCodesTopic(), jsonReportsTopic())
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
