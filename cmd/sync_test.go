package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncWarnsAboutMissingCheckoutAndSucceeds(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "fleet.toml")
	if err := os.WriteFile(configPath, []byte("[repos.\"github.com/a/one\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODE_DIR", root)
	t.Setenv("PATH", "")

	out, stderr, err := execute(t, "sync", "--config", configPath, "--json")

	if err != nil {
		t.Fatalf("sync with missing checkout = %q, %v, want success", out, err)
	}
	var report checkoutReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	if !report.Complete || len(report.Results) != 1 || report.Results[0].Status != "missing" || report.Results[0].Error != "" {
		t.Errorf("report = %+v, want one complete missing result", report)
	}
	if !strings.Contains(stderr, "warning") || !strings.Contains(stderr, "fleet clone github.com/a/one") {
		t.Errorf("stderr = %q, want a warning naming fleet clone", stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "github.com")); !os.IsNotExist(err) {
		t.Errorf("sync created a destination for the missing checkout: %v", err)
	}
}
