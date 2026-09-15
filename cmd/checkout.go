package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
		if !run.jsonOutput {
			if err := writeResult(cmd.OutOrStdout(), loaded.Inventory.Label(repo.ID), result); err != nil {
				return workError{err}
			}
		}
	}

	if run.jsonOutput {
		err = json.NewEncoder(cmd.OutOrStdout()).Encode(report)
	} else {
		err = writeSummary(cmd.OutOrStdout(), report)
	}
	if err != nil {
		return workError{err}
	}
	if !report.Complete {
		return workError{errors.New("one or more repositories failed; see the report")}
	}
	return nil
}

// writeResult prints one repository as a marked line: a check for finished
// work, an arrow for a dry-run plan, and a cross for a missing or failed
// checkout. Extra lines of an error follow, indented.
func writeResult(out io.Writer, label string, result checkout.Result) error {
	marker, detail := "✓", ""
	name := label
	if result.Branch != "" {
		name += " (" + result.Branch + ")"
	}
	completed := describeActions(result.Actions, false)

	switch result.Status {
	case checkout.StatusCurrent:
		detail = "up to date at " + shortCommit(result.Commit)
	case checkout.StatusPresent:
		detail = "already present at " + shortCommit(result.Commit)
	case checkout.StatusCloned, checkout.StatusUpdated:
		detail = strings.Join(completed, ", ")
	case checkout.StatusPlanned:
		marker = "→"
		detail = "up to date at " + shortCommit(result.Commit)
		if len(result.PlannedActions) > 0 {
			detail = "would " + strings.Join(describeActions(result.PlannedActions, true), ", ")
		}
		if result.HistoryUnresolved {
			detail += "; history unresolved"
		}
	case checkout.StatusMissing:
		marker = "✗"
		detail = fmt.Sprintf("not cloned; run 'fleet clone %s'", label)
	case checkout.StatusFailed:
		marker = "✗"
		detail = firstLine(result.Error)
		if len(completed) > 0 {
			detail = strings.Join(completed, ", ") + "; " + detail
		}
	}

	var line strings.Builder
	fmt.Fprintf(&line, "%s %s: %s\n", marker, name, detail)
	if result.Status == checkout.StatusFailed {
		for _, extra := range strings.Split(result.Error, "\n")[1:] {
			fmt.Fprintf(&line, "    %s\n", extra)
		}
	}
	_, err := io.WriteString(out, line.String())
	return err
}

// describeActions puts actions into words in the order they run, as a plan
// ("switch from main") or as completed work ("switched from main").
func describeActions(actions []checkout.Action, planned bool) []string {
	var words []string
	for _, action := range actions {
		var plan, done, object string
		switch action.Kind {
		case checkout.KindClone:
			plan, done, object = "clone at", "cloned at", shortCommit(action.To)
		case checkout.KindCreateBranch:
			plan, done, object = "create branch", "created branch", action.Branch
		case checkout.KindSwitchBranch:
			plan, done, object = "switch from", "switched from", action.From
		case checkout.KindUpdate:
			plan, done, object = "update", "updated", shortCommit(action.From)+" -> "+shortCommit(action.To)
		default:
			plan, done = string(action.Kind), string(action.Kind)
		}
		verb := done
		if planned {
			verb = plan
		}
		words = append(words, strings.TrimSpace(verb+" "+object))
	}
	return words
}

// writeSummary counts the results by status in words.
func writeSummary(out io.Writer, report checkoutReport) error {
	counts := make(map[checkout.Status]int)
	for _, result := range report.Results {
		counts[result.Status]++
	}
	var summary []string
	for _, entry := range []struct {
		status checkout.Status
		word   string
	}{
		{checkout.StatusCurrent, "up to date"}, {checkout.StatusUpdated, "updated"}, {checkout.StatusCloned, "cloned"}, {checkout.StatusPresent, "present"},
		{checkout.StatusPlanned, "planned"}, {checkout.StatusMissing, "missing"}, {checkout.StatusFailed, "failed"},
	} {
		if counts[entry.status] > 0 {
			summary = append(summary, fmt.Sprintf("%d %s", counts[entry.status], entry.word))
		}
	}

	var line strings.Builder
	line.WriteString(countNoun(len(report.Results), "repository", "repositories"))
	if len(summary) > 0 {
		fmt.Fprintf(&line, ": %s", strings.Join(summary, ", "))
	}
	if report.DryRun {
		line.WriteString(" (dry run)")
	}
	line.WriteByte('\n')
	_, err := io.WriteString(out, line.String())
	return err
}
