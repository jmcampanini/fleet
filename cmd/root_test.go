package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func execute(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := NewRoot()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	err := root.ExecuteContext(t.Context())
	return stdout.String(), stderr.String(), err
}

func TestHelpAndVersionNeedNoSetup(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("CODE_DIR", "")
	t.Setenv("PATH", "")
	for _, args := range [][]string{{"--help"}, {"--version"}, {"clone", "--help"}, {"sync", "--help"}, {"json-reports"}, {"issues", "--help"}, {"prs", "--help"}, {"config", "--help"}, {"completion", "bash"}} {
		out, _, err := execute(t, args...)
		if err != nil || out == "" {
			t.Errorf("execute(%v) = %q, %v", args, out, err)
		}
	}
}

func TestExitCodesTopicPrintsSameHelpFromBothEntryPoints(t *testing.T) {
	a, _, err := execute(t, "exit-codes")
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := execute(t, "help", "exit-codes")
	if err != nil {
		t.Fatal(err)
	}
	if a != b || !strings.Contains(a, "0  Success") || !strings.Contains(a, "1  Any error") {
		t.Errorf("exit code help differs or lacks status rows:\n%s\n%s", a, b)
	}
}

func TestEveryApplicationCommandHasWrappedLongHelp(t *testing.T) {
	var walk func(*cobra.Command)
	walk = func(command *cobra.Command) {
		if command.Name() == "help" || command.Name() == "completion" {
			return
		}
		if command.Long == "" {
			t.Errorf("%s lacks long help", command.CommandPath())
		}
		for _, text := range []string{command.Long, command.Example} {
			for _, line := range strings.Split(text, "\n") {
				if len(line) > 80 {
					t.Errorf("%s line exceeds 80 columns: %q", command.CommandPath(), line)
				}
			}
		}
		for _, child := range command.Commands() {
			walk(child)
		}
	}
	walk(NewRoot())
}

func TestUsageErrorsPrecedeConfigAndWork(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("PATH", "")
	for _, args := range [][]string{{"unknown"}, {"config", "extra"}, {"exit-codes", "extra"}, {"json-reports", "extra"}, {"clone"}, {"clone", "--all", "one"}, {"clone", "--all", "--group", "empty"}, {"sync", "--unknown"}, {"issues", "--state", "merged"}, {"prs", "--sort", "closed"}, {"issues", "--sort", "merged"}} {
		out, _, err := execute(t, args...)
		if err == nil || out != "" || strings.Contains(err.Error(), "load config") {
			t.Errorf("execute(%v) = %q, %v", args, out, err)
		}
	}
}

func TestEmptyQueriesAndConfigProvenance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fleet.toml")
	if err := os.WriteFile(path, []byte("[groups]\nempty = []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODE_DIR", "")
	t.Setenv("PATH", "")
	for _, resource := range []string{"issues", "prs", "sync", "clone"} {
		out, stderr, err := execute(t, resource, "--config", path, "--group", "empty", "--json")
		if err != nil || !json.Valid([]byte(out)) || !strings.Contains(out, `"complete":true`) || !strings.Contains(stderr, "No repositories") {
			t.Errorf("empty %s = %q, %q, %v", resource, out, stderr, err)
		}
	}
	out, _, err := execute(t, "config", "--config", path, "--issues-limit", "31", "--provenance")
	if err != nil || !strings.Contains(out, "limit = 31") || !strings.Contains(out, "<pflag>") {
		t.Errorf("config = %q, %v", out, err)
	}
}

func TestSyncBatchJSONContinuesAfterFailure(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "fleet.toml")
	if err := os.WriteFile(configPath, []byte("[repos.\"github.com/a/one\"]\n[repos.\"github.com/a/two\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"one", "two"} {
		path := filepath.Join(root, "github.com", "a", name)
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "keep"), []byte("unrelated"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CODE_DIR", root)
	out, _, err := execute(t, "sync", "--config", configPath, "--json")
	if err == nil || !json.Valid([]byte(out)) {
		t.Fatalf("partial sync = %q, %v", out, err)
	}
	var report checkoutReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	if report.Complete || len(report.Results) != 2 || report.Results[0].Error == "" || report.Results[1].Error == "" {
		t.Fatalf("partial report = %+v", report)
	}
	for _, result := range report.Results {
		body, err := os.ReadFile(filepath.Join(result.Path, "keep"))
		if err != nil || string(body) != "unrelated" {
			t.Errorf("preserved file = %q, %v", body, err)
		}
	}
}
