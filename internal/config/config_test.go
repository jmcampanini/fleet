package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmcampanini/go-config-loader/configreporter"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/spf13/pflag"
)

func flags(t *testing.T, args ...string) *pflag.FlagSet {
	t.Helper()
	f := pflag.NewFlagSet("test", pflag.ContinueOnError)
	if err := pflagloader.Register[Config](f); err != nil {
		t.Fatal(err)
	}
	if err := f.Parse(args); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestLayersAndProvenance(t *testing.T) {
	t.Setenv("FLEET_ISSUES_LIMIT", "25")
	t.Setenv("FLEET_PRS_LIMIT", "26")
	path := filepath.Join(t.TempDir(), "fleet.toml")
	if err := os.WriteFile(path, []byte("[issues]\nlimit = 20\n[repos.\"github.com/a/one\"]\nbranch = \"develop\"\n[repos.\"github.com/a/two\"]\n[groups]\nall = [\"one\", \"two\", \"a/one\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, report, inv, err := Load(path, flags(t, "--issues-limit", "30"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Issues.Limit != 30 || cfg.PRs.Limit != 26 {
		t.Fatalf("limits = %d, %d, want 30, 26", cfg.Issues.Limit, cfg.PRs.Limit)
	}
	repos, err := inv.Select(nil, []string{"all"})
	if err != nil || len(repos) != 2 || repos[0].Branch != "develop" {
		t.Fatalf("composed inventory = %v, %v", repos, err)
	}
	rows := configreporter.New(cfg, report).ProvenanceRows()
	joined := ""
	for _, row := range rows {
		joined += strings.Join(row, " ") + "\n"
	}
	for _, want := range []string{"issues.limit", "prs.limit", path, "<env>", "<pflag>"} {
		if !strings.Contains(joined, want) {
			t.Errorf("provenance %q missing %q", joined, want)
		}
	}
	body, err := configreporter.New(cfg, report).TOML()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FLEET_ISSUES_LIMIT", "")
	t.Setenv("FLEET_PRS_LIMIT", "")
	// Remove empty overrides: empty scalar environment values are invalid.
	if err := os.Unsetenv("FLEET_ISSUES_LIMIT"); err != nil {
		t.Fatal(err)
	}
	if err := os.Unsetenv("FLEET_PRS_LIMIT"); err != nil {
		t.Fatal(err)
	}
	roundtrip, _, _, err := Load(path, flags(t))
	if err != nil || roundtrip.Issues.Limit != 30 || roundtrip.PRs.Limit != 26 {
		t.Fatalf("roundtrip = %+v, %v", roundtrip, err)
	}
}

func TestDiscoveryAndStrictValidation(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	path := filepath.Join(root, "fleet", "fleet.toml")
	if _, _, _, err := Load("", flags(t)); err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("missing default error = %v, want expected path", err)
	}
	if err := os.Mkdir(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"unknown = true", "[issues]\nlimit = 0", "[prs]\nlimit = -1", "[repos.\"github.com/a/b\"]\nunknown = 1", "[groups]\nx = [\"undefined\"]"} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := Load("", flags(t)); err == nil {
			t.Errorf("Load(%q) succeeded", body)
		}
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, _, inv, err := Load("", flags(t))
	if err != nil {
		t.Fatal(err)
	}
	repos, err := inv.Select(nil, nil)
	if err != nil || len(repos) != 0 || cfg.Issues.Limit != 15 || cfg.PRs.Limit != 15 {
		t.Fatalf("empty inventory/defaults = %+v, %v, %v", cfg, repos, err)
	}
}
