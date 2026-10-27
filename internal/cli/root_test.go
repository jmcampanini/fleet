package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jmcampanini/fleet/internal/checkoutreport"
	"github.com/jmcampanini/fleet/internal/exitstatus"
	"github.com/spf13/cobra"
)

// unloadableDiscoveredConfig points discovery at a file that cannot parse,
// so any command that loads configuration fails with a "load config" error.
func unloadableDiscoveredConfig(t *testing.T) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "fleet")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("bad = [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", filepath.Dir(dir))
}

func TestHelpAndVersionNeedNoSetup(t *testing.T) {
	unloadableDiscoveredConfig(t)
	t.Setenv("CODE_DIR", "")
	t.Setenv("PATH", "")
	for _, args := range [][]string{{"--help"}, {"--version"}, {"clone", "--help"}, {"sync", "--help"}, {"repos", "--help"}, {"json-reports"}, {"issues", "--help"}, {"prs", "--help"}, {"config", "--help"}, {"groups", "--help"}, {"completion", "bash"}} {
		out, _, err := execute(t, args...)
		if err != nil || out == "" {
			t.Errorf("execute(%v) = %q, %v", args, out, err)
		}
	}
}

func TestExitCodesTopicListsEveryStatus(t *testing.T) {
	out, _, err := execute(t, "exit-codes")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []string{"0  Success", "1  Failure after", "2  Failure before"} {
		if !strings.Contains(out, row) {
			t.Errorf("exit-codes help lacks %q:\n%s", row, out)
		}
	}
}

// isHelpTopic reports whether command was built by newHelpTopic.
func isHelpTopic(command *cobra.Command) bool {
	return command.Annotations[helpTopicAnnotation] == "true"
}

// applicationCommands returns every command the repository defines, depth
// first from the root, skipping the help and completion subtrees cobra adds.
func applicationCommands(t *testing.T) []*cobra.Command {
	t.Helper()
	var commands []*cobra.Command
	var walk func(*cobra.Command)
	walk = func(command *cobra.Command) {
		if command.Name() == "help" || command.Name() == "completion" {
			return
		}
		commands = append(commands, command)
		for _, child := range command.Commands() {
			walk(child)
		}
	}
	walk(NewRoot())
	return commands
}

func TestEveryApplicationCommandDeclaresItsGrammar(t *testing.T) {
	for _, command := range applicationCommands(t) {
		if command.HasParent() && command.Args == nil {
			t.Errorf("%s declares no Args", command.CommandPath())
		}
		if command.HasParent() && (command.HasSubCommands() || isHelpTopic(command)) && !command.Runnable() {
			t.Errorf("%s is a command group or help topic but is not runnable", command.CommandPath())
		}
	}

	root := NewRoot()
	if root.Args != nil {
		t.Error("root declares Args; an unset root rejects unknown commands during discovery")
	}
	if !root.DisableSuggestions {
		t.Error("root does not set DisableSuggestions")
	}
	for _, name := range []string{"help", "version"} {
		if root.Flags().Lookup(name) == nil {
			t.Errorf("root lacks the --%s flag before execution", name)
		}
	}
}

func TestEveryApplicationCommandHasHelpWithin80Columns(t *testing.T) {
	for _, command := range applicationCommands(t) {
		t.Run(command.CommandPath(), func(t *testing.T) {
			if command.Short == "" {
				t.Error("lacks short help")
			}
			if command.Long == "" {
				t.Error("lacks long help")
			}

			path := strings.Fields(command.CommandPath())[1:]
			help, stderr, err := execute(t, append(path, "--help")...)
			if err != nil || stderr != "" || help == "" {
				t.Fatalf("--help = %q, %q, %v", help, stderr, err)
			}
			for _, line := range strings.Split(help, "\n") {
				if width := utf8.RuneCountInString(line); width > 80 {
					t.Errorf("help line has %d columns, want at most 80: %q", width, line)
				}
			}

			if !isHelpTopic(command) {
				return
			}
			for _, invocation := range [][]string{path, append([]string{"help"}, path...)} {
				out, stderr, err := execute(t, invocation...)
				if err != nil || stderr != "" || out != help {
					t.Errorf("execute(%v) = %q, %q, %v, want the command's help", invocation, out, stderr, err)
				}
			}
		})
	}
}

func TestUsageErrorsPrecedeConfigAndWork(t *testing.T) {
	unloadableDiscoveredConfig(t)
	t.Setenv("PATH", "")
	for _, args := range [][]string{{"unknown"}, {"config", "extra"}, {"groups", "extra"}, {"exit-codes", "extra"}, {"json-reports", "extra"}, {"clone"}, {"clone", "--all", "one"}, {"clone", "--all", "--group", "empty"}, {"sync", "--unknown"}, {"repos", "--unknown"}, {"issues", "--state", "merged"}, {"prs", "--sort", "closed"}, {"issues", "--sort", "merged"},
		{"issues", "--json", "--csv"}, {"prs", "--jsonl", "--json"}, {"issues", "--limit", "5", "--all"}, {"issues", "--limit", "0"}, {"prs", "--issues-limit", "5"},
		{"issues", "--overwrite"}, {"issues", "--output-file", "out.txt"}, {"issues", "--output-file", "", "--json"}, {"prs", "--no-header"}, {"repos", "--csv"},
		{"help", "unknown"}, {"help", "issues", "extra"}, {"completion", "unknown"}, {"completion", "bash", "extra"}} {
		out, _, err := execute(t, args...)
		if err == nil || out != "" || strings.Contains(err.Error(), "load config") || exitstatus.ExitCode(err) != exitstatus.ExitPreflight {
			t.Errorf("execute(%v) = %q, %v (exit %d), want a preflight error", args, out, err, exitstatus.ExitCode(err))
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
	for _, resource := range []string{"issues", "prs", "sync", "clone", "repos"} {
		out, stderr, err := execute(t, resource, "--config", path, "--group", "empty", "--json")
		if err != nil || !json.Valid([]byte(out)) || !strings.Contains(out, `"complete":true`) || !strings.Contains(stderr, "No repositories") {
			t.Errorf("empty %s = %q, %q, %v", resource, out, stderr, err)
		}
	}
	t.Setenv("FLEET_ISSUES_LIMIT", "31")
	out, _, err := execute(t, "config", "--config", path, "--provenance")
	if err != nil || !strings.Contains(out, "limit = 31") || !strings.Contains(out, "<env>") {
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
	if err == nil || !json.Valid([]byte(out)) || exitstatus.ExitCode(err) != exitstatus.ExitWork {
		t.Fatalf("partial sync = %q, %v (exit %d), want a work failure with a report", out, err, exitstatus.ExitCode(err))
	}
	var report checkoutreport.Report
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
