package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jmcampanini/fleet/internal/checkout"
	"github.com/jmcampanini/fleet/internal/inventory"
)

func TestRunCheckoutAlignsRowsBeforeTheNextStep(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), "fleet")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("[repos.\"github.com/a/one\"]\n[repos.\"github.com/a/two-longer\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", filepath.Dir(configDir))
	t.Setenv("CODE_DIR", t.TempDir())
	root := NewRoot()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	sync, _, err := root.Find([]string{"sync"})
	if err != nil {
		t.Fatal(err)
	}
	if err := sync.ParseFlags(nil); err != nil {
		t.Fatal(err)
	}
	sync.SetContext(context.Background())

	var seen []string
	step := func(_ context.Context, _ string, repo inventory.Repository, _ bool) checkout.Result {
		seen = append(seen, stdout.String())
		return checkout.Result{Repository: repo.ID, Branch: "main", Commit: "abc1234567", Status: checkout.StatusCurrent}
	}
	if err := runCheckout(sync, checkoutRun{step: step}); err != nil {
		t.Fatalf("runCheckout = %v, want success", err)
	}

	if len(seen) != 2 || seen[0] != "" || seen[1] != "\ueab2  one         current  abc1234 (main)\n" {
		t.Errorf("stdout before each step = %q, want the first line written before the second step", seen)
	}
	want := "\ueab2  one         current  abc1234 (main)\n" +
		"\ueab2  two-longer  current  abc1234 (main)\n\n2 repositories: 2 up to date\n"
	if stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
}
