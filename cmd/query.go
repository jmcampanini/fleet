package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmcampanini/fleet/internal/process"
	"github.com/jmcampanini/fleet/internal/query"
	"github.com/spf13/cobra"
)

// querySpec carries what differs between the issues and prs commands.
type querySpec struct {
	example  string
	long     string
	resource query.Resource
	short    string
}

// newQuery builds a listing command; issues and prs share every flag,
// the selection and ordering contract, and the report shape.
func newQuery(spec querySpec) *cobra.Command {
	var selection selection
	var jsonOutput bool
	var state, sort, order string
	command := &cobra.Command{
		Use: string(spec.resource) + " [repository ...]", Short: spec.short,
		Long:    spec.long + "\n\n" + queryHelp + "\n\n" + selectionHelp + "\n\n" + configHelp + "\n\n" + outputHelp,
		Example: spec.example,
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, refs []string) error {
			options := query.Options{Resource: spec.resource, State: query.State(state), Sort: query.Sort(sort), Order: query.Order(order)}
			if err := options.Validate(); err != nil {
				return err
			}
			loaded, repos, err := selection.resolve(cmd, refs)
			if err != nil {
				return err
			}
			options.Limit = loaded.Config.Issues.Limit
			if spec.resource == query.ResourcePRs {
				options.Limit = loaded.Config.PRs.Limit
			}

			report := (query.Client{Run: process.Execute}).List(cmd.Context(), repos, options)

			if err := renderQuery(cmd, report, jsonOutput); err != nil {
				return workError{err}
			}
			if !report.Complete {
				return workError{errors.New("partial query results; see repository errors in the report")}
			}
			return nil
		},
	}
	selection.bind(command.Flags())
	command.Flags().StringVar(&state, "state", string(query.StateOpen), "State: open, closed, all; prs also accepts merged")
	command.Flags().StringVar(&sort, "sort", "", "Sort: created, updated, closed; prs also accepts merged")
	command.Flags().StringVar(&order, "order", string(query.OrderDesc), "Timestamp order: asc or desc")
	command.Flags().BoolVar(&jsonOutput, "json", false, "Emit a JSON report")
	return command
}

func renderQuery(cmd *cobra.Command, report query.Report, jsonOutput bool) error {
	if jsonOutput {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
	}

	var out strings.Builder
	if !report.Complete {
		fmt.Fprintln(&out, "PARTIAL RESULTS: ordering across the full selection is not established.")
	}
	fmt.Fprintf(&out, "%s state=%s sort=%s order=%s limit=%d complete=%t\n", report.Query.Resource, report.Query.State, report.Query.Sort, report.Query.Order, report.Query.Limit, report.Complete)
	for _, item := range report.Items {
		fmt.Fprintf(&out, "%s#%d %q state=%s state_reason=%q %s=%s\n  %q\n", item.Repository, item.Number, item.Title, item.State, item.StateReason, report.Query.Sort, item.Timestamp(report.Query.Sort).Format(time.RFC3339), item.URL)
	}
	for _, result := range report.Repositories {
		if result.Error != "" {
			fmt.Fprintf(&out, "%s error: %q\n", result.Repository, result.Error)
		}
	}
	fmt.Fprintf(&out, "%d items\n", len(report.Items))
	_, err := fmt.Fprint(cmd.OutOrStdout(), out.String())
	return err
}
