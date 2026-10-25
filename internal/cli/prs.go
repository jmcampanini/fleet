package cli

import (
	"github.com/jmcampanini/fleet/internal/query"
	"github.com/spf13/cobra"
)

func newPRs() *cobra.Command {
	var flags queryFlags
	command := newQuery(querySpec{
		resource: query.ResourcePRs,
		short:    "List pull requests across the selected repositories",
		long: `List pull requests across the configured selection. Issues are excluded.
Zero matching items succeeds.`,
		example: `  fleet prs gibson --group clis --prs-limit 30
  fleet prs --state merged
  fleet prs --state all --sort updated --order asc --json`,
	}, &flags)
	flags.selection.bind(command.Flags())
	command.Flags().StringVar(&flags.state, "state", string(query.StateOpen), "State: open, closed, all; prs also accepts merged")
	command.Flags().StringVar(&flags.sort, "sort", "", "Sort: created, updated, closed; prs also accepts merged")
	command.Flags().StringVar(&flags.order, "order", string(query.OrderDesc), "Timestamp order: asc or desc")
	command.Flags().BoolVar(&flags.jsonOutput, "json", false, "Emit a JSON report")
	return command
}
