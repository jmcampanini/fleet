package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jmcampanini/fleet/internal/checkout"
	"github.com/jmcampanini/fleet/internal/inventory"
	"github.com/spf13/cobra"
)

// checkoutReport is the payload that clone and sync share.
type checkoutReport struct {
	Complete bool              `json:"complete" help:"Every selected repository finished without error. Checkouts that sync reports missing do not clear it."`
	DryRun   bool              `json:"dry_run" help:"The report describes a preview."`
	Results  []checkout.Result `json:"results" help:"One result per selected repository, sorted by complete identity."`
}

// checkoutStep processes one selected repository; Client.Clone and
// Client.Sync both have this shape.
type checkoutStep func(ctx context.Context, root string, repo inventory.Repository, dryRun bool) checkout.Result

type checkoutRun struct {
	dryRun     bool
	jsonOutput bool
	refs       []string
	selection  selection
	step       checkoutStep
}

// runCheckout resolves the selection once, applies the step to every selected
// repository, renders one report, and fails when any result failed.
func runCheckout(cmd *cobra.Command, run checkoutRun) error {
	_, repos, err := run.selection.resolve(cmd, run.refs)
	if err != nil {
		return err
	}
	root := os.Getenv("CODE_DIR")
	if len(repos) > 0 {
		if err := checkout.ValidateRoot(root); err != nil {
			return err
		}
	}

	report := checkoutReport{Complete: true, DryRun: run.dryRun, Results: []checkout.Result{}}
	for _, repo := range repos {
		result := run.step(cmd.Context(), root, repo, run.dryRun)
		// A checkout that sync finds missing is a warning, never a failure.
		if result.Status == checkout.StatusMissing {
			if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s is not cloned at %q; run 'fleet clone %s'\n", result.Repository, result.Path, result.Repository); err != nil {
				return workError{err}
			}
		}
		report.Results = append(report.Results, result)
		report.Complete = report.Complete && result.Error == ""
	}

	if err := renderCheckout(cmd, report, run.jsonOutput); err != nil {
		return workError{err}
	}
	if !report.Complete {
		return workError{errors.New("one or more repositories failed; see the report")}
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
