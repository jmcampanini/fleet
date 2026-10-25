package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmcampanini/fleet/internal/exitstatus"
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

// queryFlags holds the flag values a listing command reads; each command
// file binds them so its flags are declared beside its help.
type queryFlags struct {
	selection  selection
	jsonOutput bool
	state      string
	sort       string
	order      string
}

// newQuery builds a listing command; issues and prs share the selection and
// ordering contract and the report shape.
func newQuery(spec querySpec, flags *queryFlags) *cobra.Command {
	return &cobra.Command{
		Use: string(spec.resource) + " [repository ...]", Short: spec.short,
		Long:    spec.long + "\n\n" + queryHelp + "\n\n" + selectionHelp + "\n\n" + configHelp + "\n\n" + outputHelp,
		Example: spec.example,
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, refs []string) error {
			options := query.Options{Resource: spec.resource, State: query.State(flags.state), Sort: query.Sort(flags.sort), Order: query.Order(flags.order)}
			if err := options.Validate(); err != nil {
				return err
			}
			loaded, repos, err := flags.selection.resolve(cmd, refs)
			if err != nil {
				return err
			}
			options.Limit = loaded.Config.Issues.Limit
			if spec.resource == query.ResourcePRs {
				options.Limit = loaded.Config.PRs.Limit
			}

			report := (query.Client{Run: process.Execute}).List(cmd.Context(), repos, options)

			if err := renderQuery(cmd, report, flags.jsonOutput); err != nil {
				return exitstatus.WorkError{Err: err}
			}
			if !report.Complete {
				return exitstatus.WorkError{Err: errors.New("partial query results; see repository errors in the report")}
			}
			return nil
		},
	}
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
