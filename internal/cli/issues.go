package cli

import (
	"github.com/jmcampanini/fleet/internal/query"
	"github.com/spf13/cobra"
)

func newIssues() *cobra.Command {
	var flags queryFlags
	command := newQuery(querySpec{
		resource: query.ResourceIssues,
		short:    "List issues across the selected repositories",
		long: `List issues across the configured selection. Pull requests are excluded.
Zero matching items succeeds.`,
		example: `  fleet issues gibson --group clis --issues-limit 30
  fleet issues --state closed
  fleet issues --state all --sort updated --order asc --json`,
	}, &flags)
	flags.selection.bind(command.Flags())
	command.Flags().StringVar(&flags.state, "state", string(query.StateOpen), "State: open, closed, all")
	command.Flags().StringVar(&flags.sort, "sort", "", "Sort: created, updated, closed")
	command.Flags().StringVar(&flags.order, "order", string(query.OrderDesc), "Timestamp order: asc or desc")
	command.Flags().BoolVar(&flags.jsonOutput, "json", false, "Emit a JSON report")
	return command
}
