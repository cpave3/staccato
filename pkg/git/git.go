package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// Runner provides an interface for running git commands
type Runner struct {
	repoPath string
}

// NewRunner creates a new git runner for the specified repository
func NewRunner(repoPath string) *Runner {
	return &Runner{repoPath: repoPath}
}

// Run executes a git command and returns the output
func (r *Runner) Run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w\nOutput: %s", strings.Join(args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// GetCurrentBranch returns the name of the current branch
func (r *Runner) GetCurrentBranch() (string, error) {
	return r.Run("rev-parse", "--abbrev-ref", "HEAD")
}

// CreateBranch creates a new branch from the current HEAD
func (r *Runner) CreateBranch(name string) error {
	_, err := r.Run("branch", name)
	return err
}

// CreateAndCheckoutBranch creates a new branch and switches to it
func (r *Runner) CreateAndCheckoutBranch(name string) error {
	_, err := r.Run("checkout", "-b", name)
	return err
}

// CheckoutBranch switches to an existing branch
func (r *Runner) CheckoutBranch(name string) error {
	_, err := r.Run("checkout", name)
	return err
}

// GetCommitSHA returns the full SHA of a commit reference
func (r *Runner) GetCommitSHA(ref string) (string, error) {
	return r.Run("rev-parse", ref)
}

// EnableRerere enables git's rerere (reuse recorded resolution) feature
func (r *Runner) EnableRerere() error {
	_, err := r.Run("config", "rerere.enabled", "true")
	return err
}

// GetMergeBase returns the best common ancestor between two commits
func (r *Runner) GetMergeBase(a, b string) (string, error) {
	return r.Run("merge-base", a, b)
}

// Rebase rebases the current branch onto the specified target
func (r *Runner) Rebase(target string) error {
	_, err := r.Run("rebase", target)
	return err
}

// RebaseOnto rebases the current branch onto newBase, replaying only commits after upstream.
// Equivalent to: git rebase --onto <newBase> <upstream>
func (r *Runner) RebaseOnto(newBase, upstream string) error {
	_, err := r.Run("rebase", "--onto", newBase, upstream)
	return err
}

// RebaseContinue continues a rebase after conflict resolution
func (r *Runner) RebaseContinue() error {
	_, err := r.Run("rebase", "--continue")
	return err
}

// RebaseAbort aborts the current rebase operation
func (r *Runner) RebaseAbort() error {
	_, err := r.Run("rebase", "--abort")
	return err
}

// IsRebaseInProgress checks if a rebase is currently in progress
func (r *Runner) IsRebaseInProgress() (bool, error) {
	output, err := r.Run("rev-parse", "--git-path", "rebase-merge")
	if err != nil {
		return false, err
	}

	// Check if the rebase directory exists
	cmd := exec.Command("test", "-d", output)
	cmd.Dir = r.repoPath
	err = cmd.Run()
	if err == nil {
		return true, nil
	}

	// Also check for rebase-apply
	output, err = r.Run("rev-parse", "--git-path", "rebase-apply")
	if err != nil {
		return false, err
	}

	cmd = exec.Command("test", "-d", output)
	cmd.Dir = r.repoPath
	err = cmd.Run()
	return err == nil, nil
}

// BranchExists checks if a branch exists
func (r *Runner) BranchExists(name string) (bool, error) {
	_, err := r.Run("rev-parse", "--verify", name)
	return err == nil, nil
}

// Push pushes the current branch to the remote
func (r *Runner) Push(branch string, force bool) error {
	args := []string{"push", "-u", "origin", branch}
	if force {
		args = append(args, "--force-with-lease")
	}
	_, err := r.Run(args...)
	return err
}

// PushAll pushes all branches in the stack to the remote
func (r *Runner) PushAll(branches []string, dryRun bool) error {
	for _, branch := range branches {
		args := []string{"push", "-u", "origin", branch}
		if dryRun {
			args = append(args, "--dry-run")
		}
		_, err := r.Run(args...)
		if err != nil {
			return fmt.Errorf("failed to push branch %s: %w", branch, err)
		}
	}
	return nil
}

// Fetch fetches updates from the remote
func (r *Runner) Fetch() error {
	_, err := r.Run("fetch", "origin")
	return err
}

// FetchPrune fetches from origin and prunes deleted remote branches
func (r *Runner) FetchPrune() error {
	_, err := r.Run("fetch", "origin", "--prune")
	return err
}

// RemoteBranchExists checks if a branch exists on the remote
func (r *Runner) RemoteBranchExists(name string) bool {
	_, err := r.Run("rev-parse", "--verify", "refs/remotes/origin/"+name)
	return err == nil
}

// IsAncestor checks if ancestor is an ancestor of commit
func (r *Runner) IsAncestor(ancestor, commit string) (bool, error) {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", ancestor, commit)
	cmd.Dir = r.repoPath
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	// Exit code 1 means not an ancestor (not an error)
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

// DiffIsEmpty checks if there are no differences between two refs
func (r *Runner) DiffIsEmpty(a, b string) (bool, error) {
	output, err := r.Run("diff", a+".."+b)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) == "", nil
}

// FastForwardBranch updates a branch ref to point to target without checkout
func (r *Runner) FastForwardBranch(branch, target string) error {
	sha, err := r.GetCommitSHA(target)
	if err != nil {
		return fmt.Errorf("failed to resolve %s: %w", target, err)
	}
	_, err = r.Run("update-ref", "refs/heads/"+branch, sha)
	return err
}

// MergeFFOnly performs a fast-forward only merge
func (r *Runner) MergeFFOnly(target string) error {
	_, err := r.Run("merge", "--ff-only", target)
	return err
}

// HasRemote checks if the repository has a remote configured
func (r *Runner) HasRemote() (bool, error) {
	output, err := r.Run("remote")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) != "", nil
}

// GetBranchSHA returns the SHA of a branch
func (r *Runner) GetBranchSHA(name string) (string, error) {
	return r.Run("rev-parse", name)
}

// DeleteBranch deletes a branch
func (r *Runner) DeleteBranch(name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := r.Run("branch", flag, name)
	return err
}

// GetUserEmail returns the configured git user.email.
func (r *Runner) GetUserEmail() (string, error) {
	return r.Run("config", "user.email")
}

// GetRemoteURL returns the URL of the named remote
func (r *Runner) GetRemoteURL(name string) (string, error) {
	return r.Run("remote", "get-url", name)
}

// CopyBranch creates a backup of a branch with a new name
func (r *Runner) CopyBranch(source, destination string) error {
	_, err := r.Run("branch", destination, source)
	return err
}

// RefExists checks if an arbitrary ref exists
func (r *Runner) RefExists(ref string) bool {
	_, err := r.Run("rev-parse", "--verify", ref)
	return err == nil
}

// ReadBlobRef reads the raw content stored at a ref (e.g. a blob ref)
func (r *Runner) ReadBlobRef(ref string) ([]byte, error) {
	output, err := r.Run("show", ref)
	if err != nil {
		return nil, err
	}
	return []byte(output), nil
}

// WriteBlobRef writes data as a blob and points ref at it
func (r *Runner) WriteBlobRef(ref string, data []byte) error {
	cmd := exec.Command("git", "hash-object", "-w", "--stdin")
	cmd.Dir = r.repoPath
	cmd.Stdin = strings.NewReader(string(data))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git hash-object failed: %w\nOutput: %s", err, out)
	}
	sha := strings.TrimSpace(string(out))
	_, err = r.Run("update-ref", ref, sha)
	return err
}

// DeleteRef removes a ref
func (r *Runner) DeleteRef(ref string) error {
	_, err := r.Run("update-ref", "-d", ref)
	return err
}

// PushRef pushes a single ref to origin (force, since it's mutable state)
func (r *Runner) PushRef(ref string) error {
	_, err := r.Run("push", "origin", ref+":"+ref, "--force")
	return err
}

// FetchRef fetches a single ref from origin. Returns error on failure.
func (r *Runner) FetchRef(ref string) error {
	_, err := r.Run("fetch", "origin", ref+":"+ref)
	return err
}

// AddFetchRefspec adds a fetch refspec to remote.origin.fetch
func (r *Runner) AddFetchRefspec(refspec string) error {
	_, err := r.Run("config", "--add", "remote.origin.fetch", refspec)
	return err
}

// RemoveFetchRefspec removes a fetch refspec from remote.origin.fetch
func (r *Runner) RemoveFetchRefspec(refspec string) error {
	_, err := r.Run("config", "--unset", "remote.origin.fetch", refspec)
	return err
}

// HasFetchRefspec checks if a fetch refspec pattern is configured
func (r *Runner) HasFetchRefspec(pattern string) bool {
	output, err := r.Run("config", "--get-all", "remote.origin.fetch")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, pattern) {
			return true
		}
	}
	return false
}

// HasUncommittedChanges checks if the working tree has any uncommitted changes
func (r *Runner) HasUncommittedChanges() (bool, error) {
	output, err := r.Run("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) != "", nil
}

// StashPush stashes uncommitted changes with a descriptive message
func (r *Runner) StashPush(message string) error {
	_, err := r.Run("stash", "push", "-m", message)
	return err
}

// CherryPick cherry-picks one or more commits onto the current branch
func (r *Runner) CherryPick(commits ...string) (string, error) {
	args := append([]string{"cherry-pick"}, commits...)
	return r.Run(args...)
}

// Reset resets HEAD to ref with the given mode (soft, mixed, or hard)
func (r *Runner) Reset(ref, mode string) (string, error) {
	switch mode {
	case "soft", "mixed", "hard":
	default:
		return "", fmt.Errorf("invalid reset mode %q: must be soft, mixed, or hard", mode)
	}
	args := []string{"reset", "--" + mode}
	if ref != "" {
		args = append(args, ref)
	}
	return r.Run(args...)
}

// Add stages files at the given paths
func (r *Runner) Add(paths []string) (string, error) {
	args := append([]string{"add", "--"}, paths...)
	return r.Run(args...)
}

// Commit creates a commit with the given message
func (r *Runner) Commit(message string) (string, error) {
	return r.Run("commit", "-m", message)
}

// Status returns porcelain status output
func (r *Runner) Status() (string, error) {
	return r.Run("status", "--porcelain")
}

// Diff returns diff output, optionally staged and/or filtered to paths
func (r *Runner) Diff(staged bool, paths []string) (string, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--staged")
	}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	return r.Run(args...)
}

// Log returns log output with optional range, limit, and stat
func (r *Runner) Log(rangeSpec string, limit int, stat bool) (string, error) {
	args := []string{"log", "--oneline"}
	if stat {
		args = append(args, "--stat")
	}
	if limit > 0 {
		args = append(args, fmt.Sprintf("-n%d", limit))
	}
	if rangeSpec != "" {
		args = append(args, rangeSpec)
	}
	return r.Run(args...)
}

// DiffStat returns diff --stat output against a ref
func (r *Runner) DiffStat(ref string) (string, error) {
	return r.Run("diff", "--stat", ref)
}

// StashPop pops the top stash entry
func (r *Runner) StashPop() error {
	_, err := r.Run("stash", "pop")
	return err
}

// GetAllBranches returns all local branch names
func (r *Runner) GetAllBranches() ([]string, error) {
	output, err := r.Run("branch", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}

	var branches []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}
