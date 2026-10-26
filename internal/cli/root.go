// Package cli connects Fleet's inventory and repository operations to Cobra.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jmcampanini/fleet/internal/config"
	"github.com/jmcampanini/fleet/internal/exitstatus"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/spf13/cobra"
)

// Version is injected by the versioned build target.
var Version = "dev"

// NewRoot constructs a fresh command tree without performing command work.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use: "fleet", Short: "Clone, sync, and describe repositories and list their issues and PRs",
		Long: `Fleet clones configured primary Git checkouts, keeps them on their intended
branches, and lists issues and pull requests across the same inventory.
clone and sync are separate commands: sync never clones, and clone never
updates an existing checkout. repos describes the inventory itself, with
each repository's path, groups, and GitHub topics.

Fleet runs two external programs, which must be on PATH and already
authenticated: git for clone and sync, and gh for repos, issues, and prs.
Every other command needs neither.

` + configHelp + "\n\n" + selectionHelp + "\n\n" + outputHelp + `

Common tasks:
  Clone every repository:           fleet clone --all
  Preview a sync without changes:   fleet sync gibson --group clis --dry-run
  Describe the inventory as JSON:   fleet repos --json
  List the 30 newest open issues:   fleet issues --issues-limit 30
  List merged pull requests:        fleet prs --state merged
  List groups and their members:    fleet groups
  Show configuration with sources:  fleet config --provenance`,
		DisableSuggestions: true, SilenceErrors: true, SilenceUsage: true, Version: Version,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	root.InitDefaultHelpFlag()
	root.InitDefaultVersionFlag()
	root.PersistentFlags().String("config", "", "Read this required TOML file instead of the default")
	if err := pflagloader.Register[config.Config](root.PersistentFlags()); err != nil {
		// Registration depends only on this package's static configuration type.
		panic(err)
	}
	root.AddCommand(newConfig(), newClone(), newSync(), newRepos(), newIssues(), newPRs(), newGroups(), exitCodesTopic(), jsonReportsTopic())
	return root
}

// Execute runs the command tree with signal cancellation, prints a failure
// as one stderr line, and returns the process exit status.
func Execute() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err := NewRoot().ExecuteContext(ctx)
	if err != nil {
		// A path or external message may carry line breaks; the diagnostic
		// stays one line so a script can read it whole.
		message := strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(err.Error())
		fmt.Fprintf(os.Stderr, "fleet: %s\n", message)
	}
	return exitstatus.ExitCode(err)
}
