package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/fleet/internal/repos"
	"github.com/spf13/cobra"
)

func TestReposWithoutTopicsListsFromTOML(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "fleet.toml")
	config := "[repos.\"github.com/a/one\"]\nbranch = \"develop\"\n[repos.\"github.com/a/two\"]\n[groups]\nclis = [\"one\"]\nall = [\"one\", \"two\"]\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODE_DIR", root)
	t.Setenv("PATH", "")

	out, _, err := execute(t, "repos", "--config", configPath, "--no-topics", "--json")

	if err != nil {
		t.Fatalf("repos --no-topics = %q, %v, want success without gh", out, err)
	}
	var report repos.Report
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	want := repos.Report{Complete: true, Results: []repos.Entry{
		{Repository: "github.com/a/one", Path: filepath.Join(root, "github.com/a/one"), Branch: "develop", Groups: []string{"all", "clis"}, Topics: []string{}},
		{Repository: "github.com/a/two", Path: filepath.Join(root, "github.com/a/two"), Groups: []string{"all"}, Topics: []string{}},
	}}
	if !reflect.DeepEqual(report, want) {
		t.Errorf("report = %+v, want %+v", report, want)
	}
	if strings.Contains(out, "null") {
		t.Errorf("JSON report contains null arrays:\n%s", out)
	}
}

func TestReposTopicFailureExitsAfterWritingReport(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "fleet.toml")
	if err := os.WriteFile(configPath, []byte("[repos.\"github.com/a/one\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODE_DIR", root)
	t.Setenv("PATH", "")

	out, _, err := execute(t, "repos", "--config", configPath, "--json")

	if err == nil || ExitCode(err) != ExitWork || !json.Valid([]byte(out)) {
		t.Fatalf("repos without gh = %q, %v (exit %d), want a work failure with a report", out, err, ExitCode(err))
	}
	var report repos.Report
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	if report.Complete || !report.TopicsFetched || len(report.Results) != 1 || !strings.Contains(report.Results[0].Error, "--no-topics") {
		t.Fatalf("report = %+v, want one failed entry naming --no-topics", report)
	}
	if !strings.Contains(err.Error(), "--no-topics") {
		t.Errorf("error = %v, want a --no-topics hint", err)
	}
}

func TestReposRequiresCodeDirOnlyWhenRepositoriesAreSelected(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "fleet.toml")
	if err := os.WriteFile(configPath, []byte("[repos.\"github.com/a/one\"]\n[groups]\nempty = []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODE_DIR", "")
	t.Setenv("PATH", "")

	out, _, err := execute(t, "repos", "--config", configPath, "--no-topics")
	if err == nil || out != "" || ExitCode(err) != ExitPreflight || !strings.Contains(err.Error(), "CODE_DIR") {
		t.Errorf("repos without CODE_DIR = %q, %v (exit %d), want a CODE_DIR preflight error", out, err, ExitCode(err))
	}

	out, _, err = execute(t, "repos", "--config", configPath, "--group", "empty", "--json")
	if err != nil || !strings.Contains(out, `"complete":true`) {
		t.Errorf("empty repos selection = %q, %v, want success", out, err)
	}
}

func TestRenderReposShowsEachEntryAndError(t *testing.T) {
	report := repos.Report{TopicsFetched: true, Results: []repos.Entry{
		{Repository: "github.com/a/one", Path: "/code/github.com/a/one", Branch: "develop", Groups: []string{"clis", "tools"}, Topics: []string{"cli", "go"}},
		{Repository: "github.com/a/two", Path: "/code/github.com/a/two", Groups: []string{}, Topics: []string{}, Error: "gh: HTTP 401; pass --no-topics"},
	}}

	out := render(t, func(command *cobra.Command) error { return renderRepos(command, report, false) })

	for _, want := range []string{`github.com/a/one path="/code/github.com/a/one" branch="develop" groups=clis,tools topics=cli,go`, `github.com/a/two path="/code/github.com/a/two" branch="" groups= topics=`, `error: "gh: HTTP 401; pass --no-topics"`, "repositories=2 complete=false topics_fetched=true"} {
		if !strings.Contains(out, want) {
			t.Errorf("renderRepos output lacks %q:\n%s", want, out)
		}
	}
}
