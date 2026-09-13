package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jmcampanini/fleet/internal/checkout"
	"github.com/jmcampanini/fleet/internal/process"
	"github.com/jmcampanini/fleet/internal/repos"
	"github.com/spf13/cobra"
)

func newRepos() *cobra.Command {
	var selection selection
	var jsonOutput, noTopics bool
	command := &cobra.Command{
		Use: "repos [repository ...]", Short: "List the configured repositories with their GitHub topics",
		Long: `List the selected repositories in complete-identity order with their
branch override, group membership, and GitHub topics. The expected
checkout path appears in the --json report. Nothing is cloned, fetched,
or changed.

Topics:
  Ask authenticated gh for each repository's topics on its host, with at
  most four repositories in flight. GitHub.com and GitHub Enterprise are
  supported; other hosts fail explicitly. Topics are sorted. A repository
  whose topics cannot be retrieved reports an error naming --no-topics,
  keeps an empty topic list, and does not stop the others; the report is
  then partial and the command exits 1. --no-topics skips GitHub entirely
  and reports from TOML alone, with topics_fetched false in the report.

Paths:
  The path is $CODE_DIR/server/org/repo whether or not a checkout exists.
  CODE_DIR must be an absolute, existing, accessible directory whenever a
  repository is selected, as for clone and sync; it is not required for
  an empty selection.

` + selectionHelp + "\n\n" + configHelp + "\n\n" + outputHelp,
		Example: `  fleet repos
  fleet repos gibson --group clis --json
  fleet repos --no-topics`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, refs []string) error {
			_, selected, err := selection.resolve(cmd, refs)
			if err != nil {
				return err
			}
			root := os.Getenv("CODE_DIR")
			if len(selected) > 0 {
				if err := checkout.ValidateRoot(root); err != nil {
					return err
				}
			}

			report := (repos.Client{Run: process.Execute}).List(cmd.Context(), root, selected, !noTopics)

			if err := renderRepos(cmd, report, jsonOutput); err != nil {
				return workError{err}
			}
			if !report.Complete {
				return workError{errors.New("one or more repositories failed; see the report or pass --no-topics")}
			}
			return nil
		},
	}
	selection.bind(command.Flags())
	command.Flags().BoolVar(&jsonOutput, "json", false, "Emit a JSON report")
	command.Flags().BoolVar(&noTopics, "no-topics", false, "List from TOML alone without contacting GitHub")
	return command
}

func renderRepos(cmd *cobra.Command, report repos.Report, jsonOutput bool) error {
	if jsonOutput {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
	}

	var out strings.Builder
	rows := make([][]string, 0, len(report.Results))
	failed := 0
	for _, entry := range report.Results {
		rows = append(rows, []string{entry.Repository, "branch=" + orDefault(entry.Branch, "default"), "groups=" + orDefault(strings.Join(entry.Groups, ","), "-"), "topics=" + orDefault(strings.Join(entry.Topics, ","), "-"), firstLine(entry.Error)})
		if entry.Error != "" {
			failed++
		}
	}
	writeColumns(&out, rows)

	topics := "topics skipped"
	if report.TopicsFetched {
		topics = "topics fetched"
	}
	fmt.Fprintf(&out, "%s, %s", countNoun(len(report.Results), "repository", "repositories"), topics)
	if failed > 0 {
		fmt.Fprintf(&out, ", %d failed", failed)
	}
	fmt.Fprintln(&out)
	_, err := fmt.Fprint(cmd.OutOrStdout(), out.String())
	return err
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
