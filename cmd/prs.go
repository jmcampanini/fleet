package cmd

import (
	"github.com/jmcampanini/fleet/internal/query"
	"github.com/spf13/cobra"
)

func newPRs() *cobra.Command {
	return newQuery(querySpec{
		resource: query.ResourcePRs,
		short:    "List pull requests across the selected repositories",
		long: `List pull requests across the configured selection. Issues are excluded.
Zero matching items succeeds.`,
		example: `  fleet prs gibson --group clis --prs-limit 30
  fleet prs --state merged
  fleet prs --state all --sort updated --order asc --json`,
	})
}
