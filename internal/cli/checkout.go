package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/jmcampanini/fleet/internal/checkout"
	"github.com/jmcampanini/fleet/internal/checkoutreport"
	"github.com/jmcampanini/fleet/internal/exitstatus"
	"github.com/jmcampanini/fleet/internal/inventory"
	"github.com/spf13/cobra"
)

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

// runCheckout resolves the selection once and applies the step to every
// selected repository. Human output reports each repository as soon as its
// step returns, then a summary; --json emits one report after all work. The
// command fails when any result failed.
func runCheckout(cmd *cobra.Command, run checkoutRun) error {
	loaded, repos, err := run.selection.resolve(cmd, run.refs)
	if err != nil {
		return err
	}
	root := os.Getenv("CODE_DIR")
	if len(repos) > 0 {
		if err := checkout.ValidateRoot(root); err != nil {
			return err
		}
	}

	labels := make([]string, len(repos))
	labelWidth := 0
	if !run.jsonOutput {
		for i, repo := range repos {
			labels[i] = loaded.Inventory.Label(repo.ID)
			labelWidth = max(labelWidth, len(labels[i]))
		}
	}

	report := checkoutreport.Report{Complete: true, DryRun: run.dryRun, Results: []checkout.Result{}}
	for i, repo := range repos {
		result := run.step(cmd.Context(), root, repo, run.dryRun)
		// A checkout that sync finds missing is a warning, never a failure.
		if result.Status == checkout.StatusMissing {
			if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s is not cloned at %q; run 'fleet clone %s'\n", result.Repository, result.Path, result.Repository); err != nil {
				return exitstatus.WorkError{Err: err}
			}
		}
		report.Results = append(report.Results, result)
		report.Complete = report.Complete && result.Error == ""
		if !run.jsonOutput {
			if err := checkoutreport.WriteResult(cmd.OutOrStdout(), labels[i], labelWidth, result); err != nil {
				return exitstatus.WorkError{Err: err}
			}
		}
	}

	if run.jsonOutput {
		err = json.NewEncoder(cmd.OutOrStdout()).Encode(report)
	} else {
		err = checkoutreport.WriteSummary(cmd.OutOrStdout(), report)
	}
	if err != nil {
		return exitstatus.WorkError{Err: err}
	}
	if !report.Complete {
		return exitstatus.WorkError{Err: errors.New("one or more repositories failed; see the report")}
	}
	return nil
}
