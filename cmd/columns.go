package cmd

import (
	"fmt"
	"strings"
)

// writeColumns renders rows as space-separated columns padded to the widest
// cell in each column, so human reports line up. The last column is never
// padded and trailing whitespace is trimmed, so empty cells cost nothing.
func writeColumns(out *strings.Builder, rows [][]string) {
	var widths []int
	for _, row := range rows {
		for i, cell := range row {
			if i >= len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], len(cell))
		}
	}

	for _, row := range rows {
		var line strings.Builder
		for i, cell := range row {
			if i < len(row)-1 {
				fmt.Fprintf(&line, "%-*s  ", widths[i], cell)
			} else {
				line.WriteString(cell)
			}
		}
		out.WriteString(strings.TrimRight(line.String(), " "))
		out.WriteByte('\n')
	}
}

// shortCommit keeps the conventional seven-character abbreviation.
func shortCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

// firstLine keeps the summary line of a multi-line error for a table cell.
func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}

// countNoun pluralizes the repository count in summaries.
func countNoun(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}
