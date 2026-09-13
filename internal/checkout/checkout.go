// Package checkout maintains primary checkouts without discarding local work.
package checkout

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jmcampanini/fleet/internal/inventory"
	"github.com/jmcampanini/fleet/internal/process"
	"golang.org/x/sys/unix"
)

// Status classifies one repository's outcome in a clone or sync report.
type Status string

// Sync reports current, updated, missing, planned, or failed. Clone reports
// cloned, present, planned, or failed.
const (
	StatusCloned  Status = "cloned"
	StatusCurrent Status = "current"
	StatusFailed  Status = "failed"
	StatusMissing Status = "missing"
	StatusPlanned Status = "planned"
	StatusPresent Status = "present"
	StatusUpdated Status = "updated"
)

// Kind names one checkout change.
type Kind string

// Clone performs KindClone; sync performs the other kinds.
const (
	KindClone        Kind = "clone"
	KindCreateBranch Kind = "create_branch"
	KindSwitchBranch Kind = "switch_branch"
	KindUpdate       Kind = "update"
)

// Action records one completed or planned checkout change.
type Action struct {
	Kind   Kind   `json:"kind"`
	Branch string `json:"branch"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
}

// Result preserves completed actions even if a later step fails.
type Result struct {
	Repository        string   `json:"repository"`
	Path              string   `json:"path"`
	Branch            string   `json:"branch"`
	Commit            string   `json:"commit"`
	Status            Status   `json:"status"`
	Actions           []Action `json:"actions"`
	PlannedActions    []Action `json:"planned_actions"`
	HistoryUnresolved bool     `json:"history_unresolved"`
	Error             string   `json:"error,omitempty"`
}

// Client executes Git at a replaceable subprocess boundary.
type Client struct {
	Run process.Run
}

// ValidateRoot checks CODE_DIR before any repository operation.
func ValidateRoot(root string) error {
	if !filepath.IsAbs(root) {
		return fmt.Errorf("CODE_DIR must be an absolute existing directory; set it with export CODE_DIR=\"$HOME/Code\"")
	}
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("CODE_DIR %q must be an accessible directory: %w", root, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("CODE_DIR %q must be a directory", root)
	}
	if err := unix.Access(root, unix.R_OK|unix.X_OK); err != nil {
		return fmt.Errorf("CODE_DIR %q must be readable and searchable: %w", root, err)
	}
	return nil
}

// Sync brings an existing checkout to its target branch and returns all
// completed and intended actions. A missing or empty destination is
// reported with status "missing" and is never cloned.
func (c Client) Sync(ctx context.Context, root string, repo inventory.Repository, dryRun bool) Result {
	r := newResult(root, repo)
	if err := c.sync(ctx, root, repo, dryRun, &r); err != nil {
		r.Status, r.Error = StatusFailed, err.Error()
	}
	return r
}

// Clone creates a missing checkout. An existing checkout of the configured
// repository is reported with status "present" and left untouched, so
// repeated clones are safe.
func (c Client) Clone(ctx context.Context, root string, repo inventory.Repository, dryRun bool) Result {
	r := newResult(root, repo)
	if err := c.clone(ctx, root, repo, dryRun, &r); err != nil {
		r.Status, r.Error = StatusFailed, err.Error()
	}
	return r
}

func newResult(root string, repo inventory.Repository) Result {
	return Result{Repository: repo.ID, Path: filepath.Join(root, filepath.FromSlash(repo.ID)), Branch: repo.Branch, Actions: []Action{}, PlannedActions: []Action{}}
}

// destinationEmpty reports whether the checkout path is absent or an empty
// directory, after rejecting identities and path components that must not
// reach the filesystem or a Git command line.
func destinationEmpty(root, id, path string) (bool, error) {
	if err := inventory.ValidateIdentity(id); err != nil {
		return false, err
	}
	if err := checkPath(root, id); err != nil {
		return false, err
	}
	entries, err := os.ReadDir(path)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect destination: %w", err)
	}
	return len(entries) == 0, nil
}

func (c Client) sync(ctx context.Context, root string, repo inventory.Repository, dryRun bool, r *Result) error {
	missing, err := destinationEmpty(root, repo.ID, r.Path)
	if err != nil {
		return err
	}
	if missing {
		r.Status = StatusMissing
		return nil
	}

	if err := c.checkout(ctx, r.Path, repo.ID); err != nil {
		return err
	}
	startingBranch, err := c.clean(ctx, r.Path)
	if err != nil {
		return err
	}
	r.Branch, r.Commit, err = c.remoteTarget(ctx, r.Path, "origin", repo.Branch)
	if err != nil {
		return err
	}
	if err := c.availableBranch(ctx, r.Path, r.Branch); err != nil {
		return err
	}
	remoteRef := "refs/remotes/origin/" + r.Branch
	if !dryRun {
		if _, err := c.Run(ctx, r.Path, "git", "fetch", "--no-tags", "--no-prune", "--no-recurse-submodules", "origin", "+refs/heads/"+r.Branch+":"+remoteRef); err != nil {
			return fmt.Errorf("fetch target: %w", err)
		}
		r.Commit, err = c.Run(ctx, r.Path, "git", "rev-parse", "--verify", remoteRef+"^{commit}")
		if err != nil {
			return err
		}
	}
	localCommit, err := c.localTarget(ctx, r.Path, r.Branch)
	if err != nil {
		return err
	}
	if localCommit != "" {
		if _, err := c.Run(ctx, r.Path, "git", "cat-file", "-e", r.Commit+"^{commit}"); err != nil {
			if !dryRun {
				return err
			}
			r.HistoryUnresolved = true
		} else {
			ahead, err := c.Run(ctx, r.Path, "git", "rev-list", "--count", r.Commit+".."+localCommit)
			if err != nil {
				return err
			}
			count, err := strconv.Atoi(ahead)
			if err != nil {
				return fmt.Errorf("read target history: %w", err)
			}
			if count > 0 {
				return fmt.Errorf("target branch %q has %d local commits absent from origin; resolve its ahead or diverged history before rerunning", r.Branch, count)
			}
		}
	}

	plan := planActions(startingBranch, localCommit, r.Branch, r.Commit)
	if dryRun {
		r.Status, r.PlannedActions = StatusPlanned, plan
		return nil
	}

	// The fetch above may have taken long enough for the user to touch the
	// checkout; recheck the state the plan was built from before changing it.
	branch, err := c.clean(ctx, r.Path)
	if err != nil {
		return err
	}
	if branch != startingBranch {
		return fmt.Errorf("checked-out branch changed during sync; inspect the checkout")
	}
	currentTarget, err := c.localTarget(ctx, r.Path, r.Branch)
	if err != nil {
		return err
	}
	if currentTarget != localCommit {
		return fmt.Errorf("target branch changed during sync; inspect the checkout")
	}

	for _, action := range plan {
		if err := c.apply(ctx, r.Path, action); err != nil {
			return err
		}
		r.Actions = append(r.Actions, action)
	}
	if err := c.verify(ctx, repo.ID, r); err != nil {
		return err
	}
	r.Status = StatusCurrent
	if len(r.Actions) > 0 {
		r.Status = StatusUpdated
	}
	return nil
}

// planActions is the single source of the actions a sync performs; a dry
// run reports the plan and a real run executes it in order.
func planActions(startingBranch, localCommit, branch, commit string) []Action {
	plan := []Action{}
	if localCommit == "" {
		plan = append(plan, Action{Kind: KindCreateBranch, Branch: branch, To: commit})
	}
	if startingBranch != branch {
		plan = append(plan, Action{Kind: KindSwitchBranch, Branch: branch, From: startingBranch, To: branch})
	}
	if localCommit != "" && localCommit != commit {
		plan = append(plan, Action{Kind: KindUpdate, Branch: branch, From: localCommit, To: commit})
	}
	return plan
}

func (c Client) apply(ctx context.Context, path string, action Action) error {
	switch action.Kind {
	case KindCreateBranch:
		if _, err := c.Run(ctx, path, "git", "branch", "--track", action.Branch, "refs/remotes/origin/"+action.Branch); err != nil {
			return fmt.Errorf("create target branch: %w", err)
		}
	case KindSwitchBranch:
		if _, err := c.Run(ctx, path, "git", "switch", "--no-guess", "--no-overwrite-ignore", action.To); err != nil {
			return fmt.Errorf("switch target branch: %w", err)
		}
	case KindUpdate:
		if _, err := c.Run(ctx, path, "git", "merge", "--ff-only", "--no-autostash", "--no-overwrite-ignore", action.To); err != nil {
			return fmt.Errorf("fast-forward target branch: %w", err)
		}
	default:
		return fmt.Errorf("sync cannot apply action %q", action.Kind)
	}
	return nil
}

func (c Client) localTarget(ctx context.Context, path, branch string) (string, error) {
	ref := "refs/heads/" + branch
	refs, err := c.Run(ctx, path, "git", "for-each-ref", "--format=%(refname)%09%(objectname)", ref)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(refs, "\n") {
		name, commit, _ := strings.Cut(line, "\t")
		if name == ref {
			return commit, nil
		}
	}
	return "", nil
}

func checkPath(root, id string) error {
	path := root
	for _, component := range strings.Split(id, "/") {
		path = filepath.Join(path, component)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect path %q: %w", path, err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("path %q must be a directory, not a file or symbolic link", path)
		}
	}
	return nil
}

func (c Client) checkout(ctx context.Context, path, id string) error {
	root, err := c.Run(ctx, path, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("expected a checkout root: %w", err)
	}
	expected, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	actual, err := filepath.EvalSymlinks(root)
	if err != nil || actual != expected {
		return fmt.Errorf("path belongs to another checkout %q", root)
	}
	gitDir, err := c.Run(ctx, path, "git", "rev-parse", "--absolute-git-dir")
	if err != nil {
		return err
	}
	common, err := c.Run(ctx, path, "git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return err
	}
	if gitDir != common {
		return fmt.Errorf("expected a primary checkout; %q is an additional worktree", path)
	}
	origin, err := c.Run(ctx, path, "git", "remote", "get-url", "--all", "origin")
	if err != nil {
		return err
	}
	if originIdentity(origin) != id {
		return fmt.Errorf("origin does not identify configured repository %q", id)
	}
	return nil
}

func originIdentity(origin string) string {
	origin = strings.TrimSuffix(strings.TrimRight(origin, "/"), ".git")
	if strings.HasPrefix(origin, "git@") && !strings.Contains(origin, "://") {
		host, path, ok := strings.Cut(strings.TrimPrefix(origin, "git@"), ":")
		if ok {
			return host + "/" + path
		}
	}
	u, err := url.Parse(origin)
	if err != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "ssh") {
		return ""
	}
	return u.Hostname() + "/" + strings.TrimPrefix(u.Path, "/")
}

func (c Client) clean(ctx context.Context, path string) (string, error) {
	status, err := c.Run(ctx, path, "git", "status", "--porcelain=v1", "--untracked-files=all", "--ignore-submodules=none")
	if err != nil {
		return "", err
	}
	if status != "" {
		return "", fmt.Errorf("dirty working tree (including untracked files and submodules); preserve or resolve local work before rerunning: %s", status)
	}
	gitDir, err := c.Run(ctx, path, "git", "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", err
	}
	for _, marker := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "rebase-merge", "rebase-apply", "sequencer", "BISECT_LOG", "index.lock"} {
		if _, err := os.Lstat(filepath.Join(gitDir, marker)); err == nil {
			return "", fmt.Errorf("unfinished Git operation (%s); finish or abort it before rerunning", marker)
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	branch, err := c.Run(ctx, path, "git", "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("detached HEAD or unreadable branch is an unexpected state: %w", err)
	}
	return branch, nil
}

func (c Client) remoteTarget(ctx context.Context, dir, remote, override string) (string, string, error) {
	branch := override
	if branch == "" {
		heads, err := c.Run(ctx, dir, "git", "ls-remote", "--symref", remote, "HEAD")
		if err != nil {
			return "", "", fmt.Errorf("discover remote default branch: %w", err)
		}
		for _, line := range strings.Split(heads, "\n") {
			if strings.HasPrefix(line, "ref: refs/heads/") && strings.HasSuffix(line, "\tHEAD") {
				branch = strings.TrimSuffix(strings.TrimPrefix(line, "ref: refs/heads/"), "\tHEAD")
			}
		}
		if branch == "" {
			return "", "", fmt.Errorf("origin does not advertise a default branch; configure a branch override")
		}
	}
	if _, err := c.Run(ctx, dir, "git", "check-ref-format", "--branch", branch); err != nil {
		return branch, "", fmt.Errorf("invalid remote branch %q: %w", branch, err)
	}
	refs, err := c.Run(ctx, dir, "git", "ls-remote", "--heads", remote, "refs/heads/"+branch)
	if err != nil {
		return branch, "", err
	}
	for _, line := range strings.Split(refs, "\n") {
		commit, ref, ok := strings.Cut(line, "\t")
		if ok && ref == "refs/heads/"+branch {
			return branch, commit, nil
		}
	}
	return branch, "", fmt.Errorf("branch %q does not exist on origin", branch)
}

func (c Client) availableBranch(ctx context.Context, path, branch string) error {
	worktrees, err := c.Run(ctx, path, "git", "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return err
	}
	expected, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	var worktree string
	for _, field := range strings.Split(worktrees, "\x00") {
		if strings.HasPrefix(field, "worktree ") {
			worktree = strings.TrimPrefix(field, "worktree ")
		}
		if field == "branch refs/heads/"+branch && worktree != expected {
			return fmt.Errorf("target branch %q is occupied by another worktree %q", branch, worktree)
		}
	}
	return nil
}

func (c Client) clone(ctx context.Context, root string, repo inventory.Repository, dryRun bool, r *Result) error {
	missing, err := destinationEmpty(root, repo.ID, r.Path)
	if err != nil {
		return err
	}
	if !missing {
		return c.present(ctx, repo.ID, r)
	}

	server, rest, _ := strings.Cut(repo.ID, "/")
	address := "git@" + server + ":" + rest + ".git"
	r.Branch, r.Commit, err = c.remoteTarget(ctx, root, address, repo.Branch)
	if err != nil {
		return err
	}
	if dryRun {
		r.Status = StatusPlanned
		r.PlannedActions = append(r.PlannedActions, Action{Kind: KindClone, Branch: r.Branch, To: r.Commit})
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(r.Path), 0o755); err != nil {
		return fmt.Errorf("create parent directories: %w", err)
	}
	if _, err := c.Run(ctx, root, "git", "clone", "--no-recurse-submodules", "--origin", "origin", "--branch", r.Branch, "--", address, r.Path); err != nil {
		return fmt.Errorf("clone failed; inspect any partial destination at %q before rerunning: %w", r.Path, err)
	}
	r.Actions = append(r.Actions, Action{Kind: KindClone, Branch: r.Branch})
	// The branch can advance between discovery and cloning.
	r.Commit, err = c.Run(ctx, r.Path, "git", "rev-parse", "--verify", "refs/remotes/origin/"+r.Branch+"^{commit}")
	if err != nil {
		return err
	}
	r.Actions[0].To = r.Commit
	if err := c.verify(ctx, repo.ID, r); err != nil {
		return err
	}
	r.Status = StatusCloned
	return nil
}

// present records the state of an existing checkout without changing it.
// Any other content at the destination is an error, so clone never
// touches a directory it does not recognize.
func (c Client) present(ctx context.Context, id string, r *Result) error {
	if err := c.checkout(ctx, r.Path, id); err != nil {
		return err
	}
	branch, err := c.Run(ctx, r.Path, "git", "branch", "--show-current")
	if err != nil {
		return err
	}
	commit, err := c.Run(ctx, r.Path, "git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	r.Branch, r.Commit, r.Status = branch, commit, StatusPresent
	return nil
}

func (c Client) verify(ctx context.Context, id string, r *Result) error {
	if err := c.checkout(ctx, r.Path, id); err != nil {
		return err
	}
	branch, err := c.clean(ctx, r.Path)
	if err != nil {
		return err
	}
	commit, err := c.Run(ctx, r.Path, "git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if branch != r.Branch || commit != r.Commit {
		return fmt.Errorf("checkout changed during sync; expected branch %q at %s", r.Branch, r.Commit)
	}
	return nil
}
