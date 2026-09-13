package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jmcampanini/fleet/internal/config"
	"github.com/jmcampanini/fleet/internal/inventory"
	reposync "github.com/jmcampanini/fleet/internal/sync"
	"github.com/spf13/cobra"
)

// checkoutReport is the payload that clone and sync share.
type checkoutReport struct {
	Complete bool              `json:"complete"`
	DryRun   bool              `json:"dry_run"`
	Results  []reposync.Result `json:"results"`
}

// checkoutStep processes one selected repository; Client.Clone and
// Client.Sync both have this shape.
type checkoutStep func(ctx context.Context, root string, repo inventory.Repository, dryRun bool) reposync.Result

type checkoutRun struct {
	dryRun     bool
	groups     []string
	jsonOutput bool
	refs       []string
	step       checkoutStep
}

// runCheckout loads the selection once, applies the step to every selected
// repository, renders one report, and fails when any result failed.
func runCheckout(cmd *cobra.Command, run checkoutRun) error {
	path, err := configPath(cmd)
	if err != nil {
		return err
	}
	_, _, inv, err := config.Load(path, cmd.Root().PersistentFlags())
	if err != nil {
		return err
	}
	repos, err := inv.Select(run.refs, run.groups)
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

	report := checkoutReport{Complete: true, DryRun: run.dryRun, Results: []reposync.Result{}}
	for _, repo := range repos {
		result := run.step(cmd.Context(), root, repo, run.dryRun)
		// A checkout that sync finds missing is a warning, never a failure.
		if result.Status == "missing" {
			if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s is not cloned at %q; run 'fleet clone %s'\n", result.Repository, result.Path, result.Repository); err != nil {
				return err
			}
		}
		report.Results = append(report.Results, result)
		report.Complete = report.Complete && result.Error == ""
	}

	if err := renderCheckout(cmd, report, run.jsonOutput); err != nil {
		return err
	}
	if !report.Complete {
		return fmt.Errorf("one or more repositories failed; see the report")
	}
	return nil
}

func renderCheckout(cmd *cobra.Command, report checkoutReport, jsonOutput bool) error {
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
