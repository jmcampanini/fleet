package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
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
	all       bool
	cursor    string
	limit     int
	order     string
	output    dataOutput
	selection selection
	sort      string
	state     string
}

// bindPaging registers the flags that bound and continue a listing. The
// configured limit is not known until the command loads configuration, so
// --limit has no flag default.
func (f *queryFlags) bindPaging(command *cobra.Command, resource query.Resource) {
	command.Flags().IntVar(&f.limit, "limit", 0, fmt.Sprintf("Maximum items to return (default %s.limit)", resource))
	command.Flags().BoolVar(&f.all, "all", false, "Return every matching item")
	command.Flags().StringVar(&f.cursor, "cursor", "", "Continue after the page that printed this cursor")
	command.MarkFlagsMutuallyExclusive("limit", "all")
}

// csvColumns names the CSV columns in order; they match the JSON item
// fields.
var csvColumns = []string{"repository", "number", "title", "url", "state", "state_reason", "created_at", "updated_at", "closed_at", "merged_at"}

// newQuery builds a listing command; issues and prs share the selection and
// ordering contract and the report shape.
func newQuery(spec querySpec, flags *queryFlags) *cobra.Command {
	return &cobra.Command{
		Use: string(spec.resource) + " [repository ...]", Short: spec.short,
		Long:    spec.long + "\n\n" + fmt.Sprintf(dataHelp, spec.resource) + "\n\n" + queryHelp + "\n\n" + selectionHelp + "\n\n" + configHelp + "\n\n" + outputHelp,
		Example: spec.example,
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, refs []string) error {
			if err := flags.output.validate(cmd); err != nil {
				return err
			}
			if cmd.Flags().Changed("limit") && flags.limit <= 0 {
				return errors.New("--limit must be a positive integer")
			}
			options := query.Options{Resource: spec.resource, State: query.State(flags.state), Sort: query.Sort(flags.sort), Order: query.Order(flags.order), All: flags.all}
			if err := options.Validate(); err != nil {
				return err
			}
			loaded, repos, err := flags.selection.resolve(cmd, refs)
			if err != nil {
				return err
			}
			switch {
			case options.All:
			case cmd.Flags().Changed("limit"):
				options.Limit = flags.limit
			case spec.resource == query.ResourcePRs:
				options.Limit = loaded.Config.PRs.Limit
			default:
				options.Limit = loaded.Config.Issues.Limit
			}
			var cursor query.Cursor
			if cmd.Flags().Changed("cursor") {
				if cursor, err = query.ParseCursor(flags.cursor, options, repos); err != nil {
					return fmt.Errorf("invalid --cursor: %w", err)
				}
			}

			report := (query.Client{Run: process.Execute}).List(cmd.Context(), repos, options, cursor)

			return writeQuery(cmd, report, flags.output)
		},
	}
}

// writeQuery writes the diagnostics on stderr, then the payload, and
// returns the failure an incomplete read causes. Records and files receive
// only a complete read: JSONL and CSV cannot say they are incomplete, and a
// file the caller names never holds a failed run's output. The payload goes
// last so a failed diagnostic cannot follow a replaced file.
func writeQuery(cmd *cobra.Command, report query.Report, output dataOutput) error {
	var payload bytes.Buffer
	var err error
	switch {
	case output.json:
		err = json.NewEncoder(&payload).Encode(report)
	case output.jsonl:
		err = writeJSONL(&payload, report.Items)
	case output.csv:
		err = writeCSV(&payload, report.Items, !output.noHeader)
	default:
		err = writeHuman(&payload, report)
	}
	if err != nil {
		return exitstatus.WorkError{Err: err}
	}

	stderr := cmd.ErrOrStderr()
	failed := 0
	for _, result := range report.Repositories {
		if result.Err != nil {
			failed++
			if err := writeRepositoryError(stderr, result.Repository, result.Err); err != nil {
				return exitstatus.WorkError{Err: err}
			}
		}
	}
	human := !output.json && !output.records()
	var notes []string
	if human && !report.Complete {
		notes = append(notes, "PARTIAL RESULTS: ordering across the full selection is not established.")
	}
	// After a failed read the final fleet: line names the cursor instead.
	if report.HasMore && report.Complete && !output.json {
		notes = append(notes, fmt.Sprintf("More %s exist; continue with --cursor %s", report.Query.Resource, report.NextCursor))
	}
	for _, note := range notes {
		if _, err := fmt.Fprintln(stderr, note); err != nil {
			return exitstatus.WorkError{Err: err}
		}
	}

	if report.Complete || (output.file == "" && !output.records()) {
		if err := output.write(cmd, payload.Bytes()); err != nil {
			return exitstatus.WorkError{Err: err}
		}
	}
	if report.Complete {
		return nil
	}

	message := fmt.Sprintf("partial query results: %d of %d repositories failed", failed, len(report.Repositories))
	switch {
	case output.file != "":
		message += fmt.Sprintf("; left %q unchanged", output.file)
	case output.records():
		message += "; wrote no records"
	case output.json:
		message += "; the report's nextCursor retries them"
	default:
		message += "; --cursor " + report.NextCursor + " retries them"
	}
	return exitstatus.WorkError{Err: errors.New(message)}
}

// writeRepositoryError prints one repository's failure as a single
// diagnostic line; quoting keeps a multi-line message on that line.
func writeRepositoryError(w io.Writer, repository string, err error) error {
	_, werr := fmt.Fprintf(w, "%s error: %q\n", repository, err.Error())
	return werr
}

func writeHuman(w io.Writer, report query.Report) error {
	limit := strconv.Itoa(report.Query.Limit)
	if report.Query.All {
		limit = "all"
	}
	if _, err := fmt.Fprintf(w, "%s state=%s sort=%s order=%s limit=%s complete=%t\n", report.Query.Resource, report.Query.State, report.Query.Sort, report.Query.Order, limit, report.Complete); err != nil {
		return err
	}
	for _, item := range report.Items {
		if _, err := fmt.Fprintf(w, "%s#%d %q state=%s state_reason=%q %s=%s\n  %q\n", item.Repository, item.Number, item.Title, item.State, item.StateReason, report.Query.Sort, item.Timestamp(report.Query.Sort).Format(time.RFC3339), item.URL); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "%d items\n", len(report.Items))
	return err
}

func writeJSONL(w io.Writer, items []query.Item) error {
	encoder := json.NewEncoder(w)
	for _, item := range items {
		if err := encoder.Encode(item); err != nil {
			return err
		}
	}
	return nil
}

func writeCSV(w io.Writer, items []query.Item, header bool) error {
	out := csv.NewWriter(w)
	if header {
		if err := out.Write(csvColumns); err != nil {
			return err
		}
	}
	for _, item := range items {
		record := []string{item.Repository, strconv.Itoa(item.Number), item.Title, item.URL, item.State, item.StateReason, csvTime(&item.CreatedAt), csvTime(&item.UpdatedAt), csvTime(item.ClosedAt), csvTime(item.MergedAt)}
		if err := out.Write(record); err != nil {
			return err
		}
	}
	out.Flush()
	return out.Error()
}

// csvTime formats a timestamp as JSON does, and a missing one as an empty
// cell.
func csvTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339Nano)
}
