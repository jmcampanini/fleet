package cmd

import (
	"github.com/jmcampanini/fleet/internal/query"
	"github.com/spf13/cobra"
)

func newIssues() *cobra.Command {
	return newQuery(querySpec{
		resource: query.ResourceIssues,
		short:    "List issues across the selected repositories",
		long: `List issues across the configured selection. Pull requests are excluded.
Zero matching items succeeds.`,
		example: `  fleet issues gibson --group clis --issues-limit 30
  fleet issues --state closed
  fleet issues --state all --sort updated --order asc --json`,
	})
}
