package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCloneAllContinuesAfterFailureAndPreservesContent(t *testing.T) {
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

	out, _, err := execute(t, "clone", "--all", "--config", configPath, "--json")

	if err == nil || !json.Valid([]byte(out)) {
		t.Fatalf("clone over non-checkout directories = %q, %v, want failure with a report", out, err)
	}
	var report checkoutReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	if report.Complete || len(report.Results) != 2 || report.Results[0].Status != "failed" || report.Results[1].Status != "failed" {
		t.Fatalf("report = %+v, want two failed results", report)
	}
	for _, result := range report.Results {
		body, err := os.ReadFile(filepath.Join(result.Path, "keep"))
		if err != nil || string(body) != "unrelated" {
			t.Errorf("preserved file = %q, %v", body, err)
		}
	}
}
