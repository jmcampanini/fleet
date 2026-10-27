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
  List the 30 newest open issues:   fleet issues --limit 30
  List merged pull requests:        fleet prs --state merged
  List groups and their members:    fleet groups
  Show configuration with sources:  fleet config --provenance`,
		DisableSuggestions: true, SilenceErrors: true, SilenceUsage: true, Version: Version,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	root.InitDefaultHelpFlag()
	root.InitDefaultVersionFlag()
	root.PersistentFlags().String("config", "", "Read this required TOML file instead of the default")
	root.AddCommand(newConfig(), newClone(), newSync(), newRepos(), newIssues(), newPRs(), newGroups(), exitCodesTopic(), jsonReportsTopic())
	initUsageErrorCommands(root)
	return root
}

// initUsageErrorCommands builds cobra's help and completion commands during
// construction so an unknown topic or shell is a usage error. Cobra's own
// help prints usage and succeeds, and its completion parent is not runnable,
// so it prints help before checking its operands.
func initUsageErrorCommands(root *cobra.Command) {
	root.InitDefaultHelpCmd()
	// Cobra's completion scripts write to the writer it finds when building
	// them; resolving it at write time keeps a later SetOut in effect.
	root.SetOut(rootOut{root})
	root.InitDefaultCompletionCmd()
	root.SetOut(nil)
	for _, command := range root.Commands() {
		switch command.Name() {
		case "help":
			command.Run = nil
			command.RunE = func(cmd *cobra.Command, args []string) error {
				target, rest, err := cmd.Root().Find(args)
				if err != nil || len(rest) > 0 {
					return fmt.Errorf("unknown help topic %q", strings.Join(args, " "))
				}
				target.InitDefaultHelpFlag()
				target.InitDefaultVersionFlag()
				return target.Help()
			}
		case "completion":
			command.RunE = func(cmd *cobra.Command, _ []string) error { return cmd.Help() }
		}
	}
}

// rootOut writes to the root command's output as it is when written.
type rootOut struct{ root *cobra.Command }

func (w rootOut) Write(p []byte) (int, error) { return w.root.OutOrStdout().Write(p) }

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
