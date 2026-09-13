package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/fleet/internal/query"
	"github.com/spf13/cobra"
)

func render(t *testing.T, write func(*cobra.Command) error) string {
	t.Helper()
	var out bytes.Buffer
	command := &cobra.Command{}
	command.SetOut(&out)
	if err := write(command); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestQueryFailureExitsAfterWritingReport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fleet.toml")
	if err := os.WriteFile(path, []byte("[repos.\"github.com/a/one\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "")

	out, _, err := execute(t, "issues", "--config", path, "--json")

	if err == nil || ExitCode(err) != ExitWork || !json.Valid([]byte(out)) {
		t.Fatalf("issues without gh = %q, %v (exit %d), want a work failure with a report", out, err, ExitCode(err))
	}
	var report query.Report
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	if report.Complete || len(report.Repositories) != 1 || report.Repositories[0].Error == "" {
		t.Fatalf("report = %+v, want one incomplete repository with an error", report)
	}
}

func TestRenderQueryShowsEachItemAndFailure(t *testing.T) {
	closed := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	report := query.Report{
		Query: query.Options{Resource: query.ResourceIssues, State: query.StateClosed, Sort: query.SortClosed, Order: query.OrderDesc, Limit: 15},
		Items: []query.Item{{Repository: "github.com/a/one", Number: 7, Title: "Broken build", URL: "https://github.com/a/one/issues/7", State: "closed", StateReason: "COMPLETED", ClosedAt: &closed}},
		Repositories: []query.RepositoryResult{
			{Repository: "github.com/a/one", Complete: true},
			{Repository: "github.com/a/two", Error: "authentication failed"},
		},
	}

	out := render(t, func(command *cobra.Command) error { return renderQuery(command, report, false) })

	for _, want := range []string{"PARTIAL RESULTS", "issues state=closed sort=closed order=desc limit=15", "github.com/a/one#7", "Broken build", "https://github.com/a/one/issues/7", "state_reason=\"COMPLETED\"", "closed=2026-09-01T12:00:00Z", "github.com/a/two error: \"authentication failed\"", "1 items"} {
		if !strings.Contains(out, want) {
			t.Errorf("renderQuery output lacks %q:\n%s", want, out)
		}
	}
}
