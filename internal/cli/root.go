// Package cli connects Fleet's inventory and repository operations to Cobra.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jmcampanini/fleet/internal/exitstatus"
	"github.com/spf13/cobra"
)

// Version is injected by the versioned build target.
var Version = "dev"

// NewRoot constructs a fresh command tree without performing command work.
func NewRoot() *cobra.Command {
	var version bool
	root := &cobra.Command{
		Use: "fleet", Short: "Clone, sync, and describe repositories and list their issues and PRs",
		Long: `Fleet clones configured primary Git checkouts, keeps them on their intended
branches, and lists issues and pull requests across the same inventory.
clone and sync are separate commands: sync never clones, and clone never
updates an existing checkout. repos describes the inventory itself, with
each repository's path, groups, and GitHub topics.

Fleet runs two external programs, which must be on PATH and already
authenticated: Git 2.36 or newer for clone and sync, and gh 2.50 or newer
for repos, issues, and prs.
Every other command needs neither.

` + configHelp + "\n\n" + selectionHelp + "\n\n" + outputHelp + `

Common tasks:
  Clone every repository:           fleet clone --all
  Preview a sync without changes:   fleet sync jstack --group clis --dry-run
  Describe the inventory as JSON:   fleet repos --json
  List the 30 newest open issues:   fleet issues --limit 30
  List merged pull requests:        fleet prs --state merged
  List groups and their members:    fleet groups
  Show configuration with sources:  fleet config --provenance

Command groups:
  completion  Shell completion scripts:  fleet completion --help`,
		DisableSuggestions: true, SilenceErrors: true, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if version {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "fleet version %s\n", Version); err != nil {
					return exitstatus.WorkError{Err: err}
				}
				return nil
			}
			return writeHelp(cmd)
		},
	}
	root.InitDefaultHelpFlag()
	root.Flags().BoolVarP(&version, "version", "v", false, "Print the Fleet version")
	root.PersistentFlags().String("config", "", "Read this required TOML file instead of the default")
	root.AddGroup(&cobra.Group{ID: "commands", Title: "Commands:"}, &cobra.Group{ID: helpTopicGroupID, Title: "Help topics:"})
	help := newHelp()
	root.SetHelpCommand(help)
	for _, command := range []*cobra.Command{newConfig(), newClone(), newSync(), newRepos(), newIssues(), newPRs(), newGroups(), newCompletion(), help} {
		command.GroupID = "commands"
		root.AddCommand(command)
	}
	root.AddCommand(exitCodesTopic(), jsonReportsTopic())
	return root
}

// Execute runs the command tree with signal cancellation, prints a failure
// as one stderr line, and returns the process exit status.
func Execute() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err := runRoot(ctx, NewRoot())
	if err != nil {
		// A path or external message may carry line breaks; the diagnostic
		// stays one line so a script can read it whole.
		message := strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(err.Error())
		fmt.Fprintf(os.Stderr, "fleet: %s\n", message)
	}
	return exitstatus.ExitCode(err)
}

func runRoot(ctx context.Context, root *cobra.Command) error {
	var helpErr error
	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		helpErr = writeHelp(cmd)
	})
	if err := root.ExecuteContext(ctx); err != nil {
		return err
	}
	return helpErr
}
