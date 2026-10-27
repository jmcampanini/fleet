package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/fleet/internal/exitstatus"
	"github.com/jmcampanini/fleet/internal/query"
	"github.com/spf13/cobra"
)

// oneRepositoryConfig writes a configuration with one repository, which a
// query without gh on PATH fails to read.
func oneRepositoryConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fleet.toml")
	if err := os.WriteFile(path, []byte("[repos.\"github.com/a/one\"]\n[groups]\nempty = []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "")
	return path
}

func TestQueryFailureExitsAfterWritingReport(t *testing.T) {
	path := oneRepositoryConfig(t)

	out, stderr, err := execute(t, "issues", "--config", path, "--json")

	if err == nil || exitstatus.ExitCode(err) != exitstatus.ExitWork || !json.Valid([]byte(out)) {
		t.Fatalf("issues without gh = %q, %v (exit %d), want a work failure with a report", out, err, exitstatus.ExitCode(err))
	}
	var report query.Report
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	if report.Complete || report.NextCursor == "" || len(report.Repositories) != 1 || report.Repositories[0].Complete {
		t.Fatalf("report = %+v, want one incomplete repository and a cursor", report)
	}
	if strings.Contains(out, `"error"`) {
		t.Errorf("report carries error text:\n%s", out)
	}
	if !strings.HasPrefix(stderr, "github.com/a/one error: ") || strings.Count(stderr, "\n") != 1 {
		t.Errorf("stderr = %q, want one repository error line", stderr)
	}
}

func TestIncompleteRecordsAndFilesHoldNothing(t *testing.T) {
	path := oneRepositoryConfig(t)
	for _, format := range []string{"--jsonl", "--csv"} {
		out, stderr, err := execute(t, "prs", "--config", path, format)
		if exitstatus.ExitCode(err) != exitstatus.ExitWork || out != "" || !strings.Contains(stderr, "github.com/a/one error: ") || !strings.Contains(err.Error(), "wrote no records") {
			t.Errorf("prs %s without gh = %q, %q, %v, want no records and a work failure", format, out, stderr, err)
		}
	}

	file := filepath.Join(t.TempDir(), "prs.json")
	if err := os.WriteFile(file, []byte("earlier"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, _, err := execute(t, "prs", "--config", path, "--json", "--output-file", file, "--overwrite")
	body, readErr := os.ReadFile(file)
	if exitstatus.ExitCode(err) != exitstatus.ExitWork || out != "" || readErr != nil || string(body) != "earlier" {
		t.Errorf("failed prs --output-file = %q, %v, file %q, %v, want the file unchanged", out, err, body, readErr)
	}
	if entries, _ := os.ReadDir(filepath.Dir(file)); len(entries) != 1 {
		t.Errorf("output directory holds %d entries, want only the original file", len(entries))
	}
}

func TestOutputFileCreatesRefusesAndOverwrites(t *testing.T) {
	path := oneRepositoryConfig(t)
	file := filepath.Join(t.TempDir(), "issues.csv")

	if _, _, err := execute(t, "issues", "--config", path, "--group", "empty", "--csv", "--output-file", file); err != nil {
		t.Fatalf("create = %v", err)
	}
	if body, err := os.ReadFile(file); err != nil || string(body) != strings.Join(csvColumns, ",")+"\n" {
		t.Fatalf("created file = %q, %v, want the CSV header", body, err)
	}

	out, _, err := execute(t, "issues", "--config", path, "--group", "empty", "--jsonl", "--output-file", file)
	if exitstatus.ExitCode(err) != exitstatus.ExitPreflight || out != "" || !strings.Contains(err.Error(), "--overwrite") {
		t.Errorf("existing file = %q, %v, want a preflight refusal naming --overwrite", out, err)
	}

	if _, _, err := execute(t, "issues", "--config", path, "--group", "empty", "--json", "--output-file", file, "--overwrite"); err != nil {
		t.Fatalf("overwrite = %v", err)
	}
	body, err := os.ReadFile(file)
	if err != nil || !json.Valid(body) || !strings.Contains(string(body), `"items":[]`) {
		t.Errorf("overwritten file = %q, %v, want an empty JSON report", body, err)
	}
}

func TestEmptyResultsPerFormat(t *testing.T) {
	path := oneRepositoryConfig(t)
	for format, want := range map[string]string{
		"":            "issues state=open sort=created order=desc limit=15 complete=true\n0 items\n",
		"--jsonl":     "",
		"--csv":       strings.Join(csvColumns, ",") + "\n",
		"--no-header": "",
	} {
		args := []string{"issues", "--config", path, "--group", "empty"}
		switch format {
		case "--no-header":
			args = append(args, "--csv", format)
		case "":
		default:
			args = append(args, format)
		}
		out, _, err := execute(t, args...)
		if err != nil || out != want {
			t.Errorf("empty issues %q = %q, %v, want %q", format, out, err, want)
		}
	}
}

func queryReport() query.Report {
	closed := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	return query.Report{
		Query:    query.Options{Resource: query.ResourceIssues, State: query.StateClosed, Sort: query.SortClosed, Order: query.OrderDesc, Limit: 1},
		Complete: true,
		Items: []query.Item{{
			Repository: "github.com/a/one", Number: 7, Title: "Broken, \"quoted\"\nbuild", URL: "https://github.com/a/one/issues/7", State: "closed", StateReason: "COMPLETED",
			CreatedAt: closed.Add(-time.Hour), UpdatedAt: closed, ClosedAt: &closed,
		}},
		Repositories: []query.RepositoryResult{{Repository: "github.com/a/one", Complete: true}},
	}
}

// writeReport runs writeQuery and returns stdout, stderr, and its error.
func writeReport(t *testing.T, report query.Report, output dataOutput) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	command := &cobra.Command{}
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	err := writeQuery(command, report, output)
	return stdout.String(), stderr.String(), err
}

func TestWriteQueryHumanSendsFailuresToStderr(t *testing.T) {
	report := queryReport()
	report.Complete, report.NextCursor = false, "CURSOR"
	report.Repositories = append(report.Repositories, query.RepositoryResult{Repository: "github.com/a/two", Err: errors.New("authentication\nfailed")})

	out, stderr, err := writeReport(t, report, dataOutput{})

	for _, want := range []string{"issues state=closed sort=closed order=desc limit=1 complete=false", "github.com/a/one#7", "Broken", "https://github.com/a/one/issues/7", "state_reason=\"COMPLETED\"", "closed=2026-09-01T12:00:00Z", "1 items"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "PARTIAL") || strings.Contains(out, "error") {
		t.Errorf("stdout carries diagnostics:\n%s", out)
	}
	if want := "github.com/a/two error: \"authentication\\nfailed\"\nPARTIAL RESULTS: ordering across the full selection is not established.\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	if exitstatus.ExitCode(err) != exitstatus.ExitWork || !strings.Contains(err.Error(), "1 of 2 repositories failed") || !strings.Contains(err.Error(), "--cursor CURSOR") {
		t.Errorf("error = %v, want a work failure naming the resume cursor", err)
	}
}

func TestWriteQueryReportsMoreRecords(t *testing.T) {
	report := queryReport()
	report.HasMore, report.NextCursor = true, "CURSOR"
	note := "More issues exist; continue with --cursor CURSOR\n"

	out, stderr, err := writeReport(t, report, dataOutput{json: true})
	if err != nil || stderr != "" || !strings.Contains(out, `"hasMore":true,"nextCursor":"CURSOR"`) {
		t.Errorf("JSON = %q, %q, %v, want hasMore and nextCursor in the report", out, stderr, err)
	}
	for _, output := range []dataOutput{{}, {jsonl: true}, {csv: true}} {
		out, stderr, err := writeReport(t, report, output)
		if err != nil || out == "" || stderr != note {
			t.Errorf("writeQuery(%+v) = %q, %q, %v, want records and the cursor note", output, out, stderr, err)
		}
	}
}

func TestWriteQueryRecordFormats(t *testing.T) {
	report := queryReport()

	out, _, err := writeReport(t, report, dataOutput{jsonl: true})
	if err != nil || strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("JSONL = %q, %v, want one line", out, err)
	}
	var fields []string
	decoder := json.NewDecoder(strings.NewReader(out))
	if _, err := decoder.Token(); err != nil {
		t.Fatal(err)
	}
	for decoder.More() {
		name, err := decoder.Token()
		if err != nil {
			t.Fatal(err)
		}
		fields = append(fields, name.(string))
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Join(fields, ",") != strings.Join(csvColumns, ",") {
		t.Errorf("JSONL fields = %v, want the CSV columns %v", fields, csvColumns)
	}

	out, _, err = writeReport(t, report, dataOutput{csv: true})
	want := strings.Join(csvColumns, ",") + "\n" +
		"github.com/a/one,7,\"Broken, \"\"quoted\"\"\nbuild\",https://github.com/a/one/issues/7,closed,COMPLETED,2026-09-01T11:00:00Z,2026-09-01T12:00:00Z,2026-09-01T12:00:00Z,\n"
	if err != nil || out != want {
		t.Errorf("CSV = %q, %v, want %q", out, err, want)
	}
	out, _, err = writeReport(t, report, dataOutput{csv: true, noHeader: true})
	if err != nil || strings.HasPrefix(out, "repository,") {
		t.Errorf("CSV --no-header = %q, %v, want no header", out, err)
	}
}

func TestCursorFromAnotherQueryIsAPreflightError(t *testing.T) {
	path := oneRepositoryConfig(t)

	out, _, err := execute(t, "issues", "--config", path, "--cursor", "e30")

	if exitstatus.ExitCode(err) != exitstatus.ExitPreflight || out != "" || !strings.Contains(err.Error(), "invalid --cursor") {
		t.Errorf("issues --cursor e30 = %q, %v, want a preflight error", out, err)
	}
}
