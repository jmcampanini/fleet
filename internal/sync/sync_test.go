package sync

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/fleet/internal/inventory"
)

type fixture struct {
	client Client
	path   string
	remote string
	repo   inventory.Repository
	root   string
	seed   string
}

func gitRun(ctx context.Context, dir, program string, args ...string) (string, error) {
	args = append([]string{"-c", "core.hooksPath=/dev/null", "-c", "submodule.recurse=false", "-c", "user.name=Fleet Test", "-c", "user.email=fleet@example.invalid", "-c", "commit.gpgSign=false"}, args...)
	command := exec.CommandContext(ctx, program, args...)
	command.Dir = dir
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0")
	out, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v: %w: %s", args, err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitRun(t.Context(), dir, "git", args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func write(t *testing.T, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func setup(t *testing.T, checkout bool) fixture {
	t.Helper()
	root := t.TempDir()
	f := fixture{root: filepath.Join(root, "code with spaces"), seed: filepath.Join(root, "seed"), remote: filepath.Join(root, "remote.git"), repo: inventory.Repository{ID: "github.com/example/project"}}
	f.path = filepath.Join(f.root, filepath.FromSlash(f.repo.ID))
	if err := os.MkdirAll(f.root, 0o700); err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "-b", "trunk", f.seed)
	write(t, filepath.Join(f.seed, "tracked"), "initial\n")
	git(t, f.seed, "add", "tracked")
	git(t, f.seed, "commit", "-m", "initial")
	git(t, root, "clone", "--bare", f.seed, f.remote)
	if checkout {
		git(t, root, "clone", f.remote, f.path)
		git(t, f.path, "remote", "set-url", "origin", "https://github.com/example/project.git")
	}
	// Only transport is replaced. Real Git owns every checkout, ref, status,
	// branch, and merge operation, including the failure cases under test.
	f.client = Client{Run: func(ctx context.Context, dir, program string, args ...string) (string, error) {
		args = append([]string(nil), args...)
		if args[0] == "ls-remote" || args[0] == "fetch" || args[0] == "clone" {
			for i, arg := range args {
				if arg == "origin" && args[0] != "clone" || strings.HasPrefix(arg, "git@github.com:") {
					args[i] = f.remote
				}
			}
		}
		out, err := gitRun(ctx, dir, program, args...)
		if err == nil && args[0] == "clone" {
			_, err = gitRun(ctx, f.path, "git", "remote", "set-url", "origin", "https://github.com/example/project.git")
		}
		return out, err
	}}
	return f
}

func advance(t *testing.T, f fixture) string {
	t.Helper()
	write(t, filepath.Join(f.seed, "tracked"), "remote update\n")
	git(t, f.seed, "add", "tracked")
	git(t, f.seed, "commit", "-m", "remote update")
	git(t, f.seed, "push", f.remote, "trunk")
	return git(t, f.seed, "rev-parse", "HEAD")
}

func TestCloneDefaultAndOverride(t *testing.T) {
	for _, branch := range []string{"", "develop"} {
		t.Run("override="+branch, func(t *testing.T) {
			f := setup(t, false)
			if branch != "" {
				git(t, f.seed, "push", f.remote, "HEAD:refs/heads/develop")
				f.repo.Branch = branch
			}
			result := f.client.Sync(t.Context(), f.root, f.repo, false)
			wantBranch := branch
			if wantBranch == "" {
				wantBranch = "trunk"
			}
			if result.Error != "" || result.Status != "cloned" || result.Branch != wantBranch {
				t.Fatalf("Sync() = %+v", result)
			}
			if got := git(t, f.path, "branch", "--show-current"); got != wantBranch {
				t.Errorf("branch = %q, want %q", got, wantBranch)
			}
		})
	}
}

func TestFastForwardAndCurrent(t *testing.T) {
	f := setup(t, true)
	want := advance(t, f)
	result := f.client.Sync(t.Context(), f.root, f.repo, false)
	if result.Error != "" || result.Status != "updated" || len(result.Actions) != 1 || result.Actions[0].Kind != "update" {
		t.Fatalf("Sync() = %+v", result)
	}
	if got := git(t, f.path, "rev-parse", "HEAD"); got != want {
		t.Errorf("HEAD = %s, want %s", got, want)
	}
	result = f.client.Sync(t.Context(), f.root, f.repo, false)
	if result.Error != "" || result.Status != "current" || len(result.Actions) != 0 {
		t.Fatalf("second Sync() = %+v", result)
	}
}

func TestFeatureCommitsSurviveBranchCreation(t *testing.T) {
	f := setup(t, true)
	git(t, f.path, "switch", "-c", "feature")
	write(t, filepath.Join(f.path, "feature"), "local work")
	git(t, f.path, "add", "feature")
	git(t, f.path, "commit", "-m", "feature work")
	feature := git(t, f.path, "rev-parse", "HEAD")
	git(t, f.path, "branch", "-D", "trunk")
	result := f.client.Sync(t.Context(), f.root, f.repo, false)
	if result.Error != "" || len(result.Actions) != 2 || result.Actions[0].Kind != "create_branch" || result.Actions[1].Kind != "switch_branch" {
		t.Fatalf("Sync() = %+v", result)
	}
	if got := git(t, f.path, "rev-parse", "feature"); got != feature {
		t.Error("feature commits changed")
	}
	if got := git(t, f.path, "rev-parse", "--abbrev-ref", "trunk@{upstream}"); got != "origin/trunk" {
		t.Errorf("upstream = %q", got)
	}
}

func TestLocalWorkRejectedWithoutSwitch(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(*testing.T, fixture)
		reason  string
	}{
		{"unstaged", func(t *testing.T, f fixture) { write(t, filepath.Join(f.path, "tracked"), "local") }, "dirty"},
		{"staged", func(t *testing.T, f fixture) {
			write(t, filepath.Join(f.path, "tracked"), "local")
			git(t, f.path, "add", "tracked")
		}, "dirty"},
		{"untracked", func(t *testing.T, f fixture) { write(t, filepath.Join(f.path, "new"), "local") }, "dirty"},
		{"dirty submodule", func(t *testing.T, f fixture) {
			git(t, f.path, "-c", "protocol.file.allow=always", "submodule", "add", f.seed, "sub")
			git(t, f.path, "commit", "-m", "add submodule")
			write(t, filepath.Join(f.path, "sub", "untracked"), "local submodule work")
		}, "dirty"},
		{"unfinished", func(t *testing.T, f fixture) {
			write(t, filepath.Join(f.path, ".git", "MERGE_HEAD"), git(t, f.path, "rev-parse", "HEAD"))
		}, "unfinished"},
		{"detached", func(t *testing.T, f fixture) { git(t, f.path, "checkout", "--detach") }, "detached HEAD"},
		{"ahead target", func(t *testing.T, f fixture) {
			git(t, f.path, "commit", "--allow-empty", "-m", "local target")
			git(t, f.path, "switch", "-c", "feature")
		}, "local commits"},
		{"diverged target", func(t *testing.T, f fixture) {
			git(t, f.path, "commit", "--allow-empty", "-m", "local target")
			advance(t, f)
			git(t, f.path, "switch", "-c", "feature")
		}, "local commits"},
		{"occupied target", func(t *testing.T, f fixture) {
			git(t, f.path, "switch", "-c", "feature")
			git(t, f.path, "worktree", "add", filepath.Join(f.root, "other"), "trunk")
		}, "occupied"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			f := setup(t, true)
			tt.prepare(t, f)
			before := git(t, f.path, "rev-parse", "HEAD")
			status := git(t, f.path, "status", "--porcelain=v1")
			result := f.client.Sync(t.Context(), f.root, f.repo, false)
			if !strings.Contains(result.Error, tt.reason) || len(result.Actions) != 0 {
				t.Fatalf("Sync() = %+v, want %q with no actions", result, tt.reason)
			}
			if git(t, f.path, "rev-parse", "HEAD") != before || git(t, f.path, "status", "--porcelain=v1") != status {
				t.Error("rejection changed local work")
			}
		})
	}
}

func TestDryRunDoesNotFetchOrChangeRefs(t *testing.T) {
	f := setup(t, true)
	advance(t, f)
	git(t, f.path, "switch", "-c", "feature")
	before := git(t, f.path, "show-ref")
	result := f.client.Sync(t.Context(), f.root, f.repo, true)
	if result.Error != "" || !result.HistoryUnresolved || len(result.Actions) != 0 || len(result.PlannedActions) != 2 {
		t.Fatalf("Sync(dryRun) = %+v", result)
	}
	if git(t, f.path, "show-ref") != before || git(t, f.path, "branch", "--show-current") != "feature" {
		t.Error("dry run changed refs or branch")
	}
	if got := git(t, f.path, "status", "--porcelain=v1"); got != "" {
		t.Errorf("dry-run status = %q", got)
	}
}

func TestFailureReportsCompletedSwitchAndPreservesIgnoredFile(t *testing.T) {
	f := setup(t, true)
	git(t, f.path, "switch", "-c", "feature")
	write(t, filepath.Join(f.seed, "ignored"), "remote")
	git(t, f.seed, "add", "ignored")
	git(t, f.seed, "commit", "-m", "add tracked file")
	git(t, f.seed, "push", f.remote, "trunk")
	write(t, filepath.Join(f.path, ".git", "info", "exclude"), "ignored\n")
	write(t, filepath.Join(f.path, "ignored"), "precious local file")
	result := f.client.Sync(t.Context(), f.root, f.repo, false)
	if result.Error == "" || len(result.Actions) != 1 || result.Actions[0].Kind != "switch_branch" {
		t.Fatalf("Sync() = %+v", result)
	}
	body, err := os.ReadFile(filepath.Join(f.path, "ignored"))
	if err != nil || string(body) != "precious local file" {
		t.Fatalf("ignored file = %q, %v", body, err)
	}
}

func TestPathsAndRemoteValidation(t *testing.T) {
	for _, root := range []string{"", "relative", filepath.Join(t.TempDir(), "missing")} {
		if ValidateRoot(root) == nil {
			t.Errorf("ValidateRoot(%q) succeeded", root)
		}
	}
	f := setup(t, false)
	if err := os.MkdirAll(f.path, 0o700); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(f.path, "unrelated"), "keep")
	result := f.client.Sync(t.Context(), f.root, f.repo, false)
	if result.Error == "" {
		t.Fatal("non-checkout directory was accepted")
	}
	body, err := os.ReadFile(filepath.Join(f.path, "unrelated"))
	if err != nil || string(body) != "keep" {
		t.Fatalf("unrelated data = %q, %v", body, err)
	}
	for _, origin := range []string{"git@github.com:example/project.git", "ssh://git@github.com/example/project.git", "https://github.com/example/project/"} {
		if got := originIdentity(origin); got != f.repo.ID {
			t.Errorf("originIdentity(%q) = %q", origin, got)
		}
	}
}

func TestMissingRemoteBranchDoesNotFetch(t *testing.T) {
	f := setup(t, true)
	f.repo.Branch = "missing"
	before := git(t, f.path, "show-ref")
	result := f.client.Sync(t.Context(), f.root, f.repo, false)
	if result.Error == "" || !reflect.DeepEqual(result.Actions, []Action{}) {
		t.Fatalf("Sync() = %+v", result)
	}
	if git(t, f.path, "show-ref") != before {
		t.Error("missing branch changed refs")
	}
}

func TestEmptyDestinationAndFailedClone(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprintf("fail=%t", fail), func(t *testing.T) {
			f := setup(t, false)
			if err := os.MkdirAll(f.path, 0o700); err != nil {
				t.Fatal(err)
			}
			if fail {
				run := f.client.Run
				f.client.Run = func(ctx context.Context, dir, program string, args ...string) (string, error) {
					if args[0] == "clone" {
						write(t, filepath.Join(f.path, "partial"), "partial clone")
						return "", fmt.Errorf("transport interrupted")
					}
					return run(ctx, dir, program, args...)
				}
			}
			result := f.client.Sync(t.Context(), f.root, f.repo, false)
			if !fail && result.Status != "cloned" {
				t.Fatalf("empty destination = %+v", result)
			}
			if fail && (!strings.Contains(result.Error, "partial destination") || !strings.Contains(result.Error, f.path)) {
				t.Fatalf("failed clone = %+v", result)
			}
			if fail {
				if _, err := os.Stat(filepath.Join(f.path, "partial")); err != nil {
					t.Fatal("partial clone was removed")
				}
			}
		})
	}
}

func TestWrongOriginAndSymlinkArePreserved(t *testing.T) {
	t.Run("origin", func(t *testing.T) {
		f := setup(t, true)
		git(t, f.path, "remote", "set-url", "origin", "git@github.com:other/repository.git")
		before := git(t, f.path, "show-ref")
		result := f.client.Sync(t.Context(), f.root, f.repo, false)
		if !strings.Contains(result.Error, "origin does not identify") || git(t, f.path, "show-ref") != before {
			t.Fatalf("wrong origin = %+v", result)
		}
	})
	t.Run("symlink", func(t *testing.T) {
		f := setup(t, false)
		if err := os.Symlink(t.TempDir(), filepath.Join(f.root, "github.com")); err != nil {
			t.Fatal(err)
		}
		result := f.client.Sync(t.Context(), f.root, f.repo, false)
		if !strings.Contains(result.Error, "symbolic link") {
			t.Fatalf("symlink destination = %+v", result)
		}
	})
}

func TestDryRunMissingCheckoutCreatesNothing(t *testing.T) {
	f := setup(t, false)
	result := f.client.Sync(t.Context(), f.root, f.repo, true)
	if result.Error != "" || result.Status != "planned" || len(result.PlannedActions) != 1 {
		t.Fatalf("dry-run clone = %+v", result)
	}
	if _, err := os.Stat(filepath.Join(f.root, "github.com")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created parent directories: %v", err)
	}
}
