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
	Complete bool              `json:"complete"`
	DryRun   bool              `json:"dry_run"`
	Results  []checkout.Result `json:"results"`
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
	rows := make([][]string, 0, len(report.Results))
	counts := make(map[checkout.Status]int)
	for _, result := range report.Results {
		rows = append(rows, []string{string(result.Status), result.Repository, result.Branch, resultCommit(result), resultDetail(result)})
		counts[result.Status]++
	}
	writeColumns(&out, rows)

	var summary []string
	for _, status := range []checkout.Status{checkout.StatusCurrent, checkout.StatusUpdated, checkout.StatusCloned, checkout.StatusPresent, checkout.StatusPlanned, checkout.StatusMissing, checkout.StatusFailed} {
		if counts[status] > 0 {
			summary = append(summary, fmt.Sprintf("%d %s", counts[status], status))
		}
	}
	fmt.Fprint(&out, countNoun(len(report.Results), "repository", "repositories"))
	if len(summary) > 0 {
		fmt.Fprintf(&out, ": %s", strings.Join(summary, ", "))
	}
	if report.DryRun {
		fmt.Fprint(&out, " (dry run)")
	}
	fmt.Fprintln(&out)
	_, err := fmt.Fprint(cmd.OutOrStdout(), out.String())
	return err
}

// resultCommit shows an update as its commit range and anything else as the
// observed commit, abbreviated.
func resultCommit(result checkout.Result) string {
	for _, action := range append(append([]checkout.Action{}, result.Actions...), result.PlannedActions...) {
		if action.Kind == checkout.KindUpdate {
			return shortCommit(action.From) + " -> " + shortCommit(action.To)
		}
	}
	return shortCommit(result.Commit)
}

// resultDetail describes completed branch changes, planned work, the clone
// hint for a missing checkout, and the first line of any error.
func resultDetail(result checkout.Result) string {
	var completed, planned, parts []string
	for _, action := range result.Actions {
		switch action.Kind {
		case checkout.KindSwitchBranch:
			completed = append(completed, "switched from "+action.From)
		case checkout.KindCreateBranch:
			completed = append(completed, "created branch "+action.Branch)
		}
	}
	for _, action := range result.PlannedActions {
		switch action.Kind {
		case checkout.KindSwitchBranch:
			planned = append(planned, "switch from "+action.From)
		case checkout.KindCreateBranch:
			planned = append(planned, "create branch "+action.Branch)
		default:
			planned = append(planned, string(action.Kind))
		}
	}

	if len(completed) > 0 {
		parts = append(parts, strings.Join(completed, ", "))
	}
	if len(planned) > 0 {
		parts = append(parts, strings.Join(planned, ", "))
	}
	if result.HistoryUnresolved {
		parts = append(parts, "history unresolved")
	}
	if result.Status == checkout.StatusMissing {
		parts = append(parts, fmt.Sprintf("run 'fleet clone %s'", result.Repository))
	}
	if result.Error != "" {
		parts = append(parts, firstLine(result.Error))
	}
	return strings.Join(parts, "; ")
}
