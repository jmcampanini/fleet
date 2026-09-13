package cmd

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/jmcampanini/fleet/internal/checkout"
	"github.com/jmcampanini/fleet/internal/query"
	"github.com/spf13/cobra"
)

func jsonReportsTopic() *cobra.Command {
	return &cobra.Command{
		Use: "json-reports", Short: "Fields and values of the --json reports", Args: cobra.NoArgs,
		Long: `Every command that accepts --json writes one JSON object followed by a
newline to stdout after repository work finishes, including on failure.
Field names are lowercase with underscores. Empty arrays are [], never
null. Optional strings are omitted when absent. Timestamps are RFC 3339
strings; a missing event timestamp is null. Consumers should tolerate
added fields. The field tables below are generated from the report types.

Clone and sync report:
` + fieldTable(checkoutReport{}) + `

Each result:
` + fieldTable(checkout.Result{}) + `

Each action:
` + fieldTable(checkout.Action{}) + `

Issue and PR report:
` + fieldTable(query.Report{}) + `

The query object:
` + fieldTable(query.Options{}) + `

Each item:
` + fieldTable(query.Item{}) + `

Each repositories entry:
` + fieldTable(query.RepositoryResult{}),
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
}

// Help columns: two spaces, the JSON name padded to nameWidth, one space,
// then the description wrapped so no line exceeds helpWidth.
const (
	helpWidth = 80
	nameWidth = 18
)

// fieldTable renders one help line per exported field of a report struct,
// taking the name from the json tag and the description from the help tag.
// A field missing either tag is a programmer error caught by the help tests.
func fieldTable(report any) string {
	var lines []string
	kind := reflect.TypeOf(report)
	for index := range kind.NumField() {
		field := kind.Field(index)
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		help := field.Tag.Get("help")
		if name == "" || help == "" {
			panic(fmt.Sprintf("%s.%s needs json and help tags for the json-reports topic", kind.Name(), field.Name))
		}
		indent := strings.Repeat(" ", nameWidth+3)
		for row, text := range wrap(help, helpWidth-len(indent)) {
			if row == 0 {
				lines = append(lines, fmt.Sprintf("  %-*s %s", nameWidth, name, text))
				continue
			}
			lines = append(lines, indent+text)
		}
	}
	return strings.Join(lines, "\n")
}

// wrap breaks text at spaces into lines of at most width characters.
func wrap(text string, width int) []string {
	var lines []string
	var line string
	for _, word := range strings.Fields(text) {
		if line != "" && len(line)+1+len(word) > width {
			lines = append(lines, line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += word
	}
	return append(lines, line)
}
