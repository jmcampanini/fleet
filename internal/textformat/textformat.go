// Package textformat formats text shared by human-readable reports.
package textformat

import (
	"fmt"
	"strings"
)

// FirstLine keeps the summary line of a multi-line error for a table cell.
func FirstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}

// CountNoun pluralizes the repository count in summaries.
func CountNoun(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}
