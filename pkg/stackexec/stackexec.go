// Package stackexec runs a shell command on every branch of a stack lineage.
package stackexec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/cpave3/staccato/pkg/graph"
	"github.com/cpave3/staccato/pkg/restack"
)

// Branch statuses.
const (
	StatusPassed  = "passed"
	StatusFailed  = "failed"
	StatusSkipped = "skipped"
)

// Options controls a Run.
type Options struct {
	// Command is run through `sh -c` in each branch.
	Command string
	// FailFast stops at the first branch the command fails on. The branches
	// above it are reported as skipped.
	FailFast bool
	// Output receives the progress lines and the command's own output.
	Output io.Writer
}

// Result reports what the command did on each branch, bottom of the stack
// first.
type Result struct {
	Command  string         `json:"command"`
	Branches []BranchResult `json:"branches"`
	Passed   int            `json:"passed"`
	Failed   int            `json:"failed"`
	Skipped  int            `json:"skipped"`
	// Interrupted is set when a signal stopped the run before the last branch.
	Interrupted bool `json:"interrupted,omitempty"`
}

// BranchResult is the outcome for one branch.
type BranchResult struct {
	Branch string `json:"branch"`
	Parent string `json:"parent"`
	SHA    string `json:"sha"`
	Status string `json:"status"`
	// ExitCode is the command's exit status, nil when the command did not run
	// to completion.
	ExitCode   *int  `json:"exit_code,omitempty"`
	DurationMS int64 `json:"duration_ms"`
	// Note adds what the status does not say: why a branch was skipped or
	// failed without an exit code, or that the command's changes were stashed.
	Note string `json:"note,omitempty"`
}

// Run checks out each branch of the current lineage in turn, bottom to top,
// runs opts.Command in it, and returns to the starting branch. A branch the
// command fails on does not stop the run unless opts.FailFast is set.
//
// The lineage is the current branch, the branches below it down to the trunk,
// and every branch stacked above it. The trunk itself is left out.
//
// A failing command is reported in the Result, not as an error. An error
// means st could not do its own part; when the run had already started, the
// Result is returned with it and covers the branches reached so far.
func Run(sc *stcontext.StaccatoContext, opts Options) (result *Result, err error) {
	g := sc.Git
	start, err := g.GetCurrentBranch()
	if err != nil || start == "HEAD" {
		return nil, fmt.Errorf("HEAD is detached — check out a branch first")
	}
	if start != sc.Graph.Root {
		if _, tracked := sc.Graph.GetBranch(start); !tracked {
			return nil, fmt.Errorf("current branch %q is not in the stack — add it with 'st attach', or check out a branch that is", start)
		}
	}

	var branches []BranchResult
	for _, b := range lineage(sc.Graph, start) {
		sha, err := g.GetCommitSHA("refs/heads/" + b.Name)
		if err != nil {
			return nil, fmt.Errorf("resolve branch %q: %w", b.Name, err)
		}
		branches = append(branches, BranchResult{Branch: b.Name, Parent: b.Parent, SHA: sha})
	}
	if len(branches) == 0 {
		return nil, fmt.Errorf("the stack has no branches to run on — create one with 'st new' or 'st append'")
	}

	if dirty, err := trackedChanges(sc); err != nil {
		return nil, err
	} else if dirty != "" {
		return nil, fmt.Errorf("st exec checks out each branch in turn, and uncommitted changes would be carried into them — commit or stash them first")
	}

	// An interrupt stops the command and still returns to the starting
	// branch, so a cancelled or timed-out run does not strand HEAD on a
	// detached commit.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	defer func() {
		if restoreErr := g.CheckoutBranch(start); restoreErr != nil && err == nil {
			err = fmt.Errorf("return to %q: %w", start, restoreErr)
		}
	}()

	if opts.Output == nil {
		opts.Output = io.Discard
	}
	result = &Result{Command: opts.Command, Branches: branches}
	// skipNote is set once the run has stopped early, and says why the
	// branches that are left were not run.
	skipNote := ""
	for i := range branches {
		b := &branches[i]
		if skipNote != "" {
			b.Status = StatusSkipped
			b.Note = skipNote
			result.Skipped++
			continue
		}

		fmt.Fprintf(opts.Output, "==> [%d/%d] %s: %s\n", i+1, len(branches), b.Branch, opts.Command)
		env := []string{
			"ST_EXEC_BRANCH=" + b.Branch,
			"ST_EXEC_PARENT=" + b.Parent,
			"ST_EXEC_SHA=" + b.SHA,
			fmt.Sprintf("ST_EXEC_INDEX=%d", i+1),
			fmt.Sprintf("ST_EXEC_TOTAL=%d", len(branches)),
		}
		runErr := runOn(ctx, sc, b, env, opts)
		if b.Status == StatusPassed {
			result.Passed++
		} else {
			result.Failed++
		}
		switch {
		case runErr != nil:
			err = runErr
			skipNote = "not run: st exec stopped at " + b.Branch
		case ctx.Err() != nil:
			result.Interrupted = true
			skipNote = "not run: interrupted"
		case b.Status == StatusFailed && opts.FailFast:
			skipNote = "not run: --fail-fast stopped at " + b.Branch
		}
	}
	return result, err
}

// lineage returns the tracked branches of start's lineage, a parent always
// before its children: the branches from the trunk up to start, then
// everything stacked on start. Siblings are in name order, so that every run
// visits the branches in the same order.
func lineage(g *graph.Graph, start string) []*graph.Branch {
	var branches []*graph.Branch
	for _, name := range restack.GetAncestors(g, start) {
		if b, tracked := g.GetBranch(name); tracked && name != g.Root {
			branches = append(branches, b)
		}
	}
	seen := map[string]bool{start: true}
	var descend func(parent string)
	descend = func(parent string) {
		children := g.GetChildren(parent)
		sort.Slice(children, func(i, j int) bool { return children[i].Name < children[j].Name })
		for _, child := range children {
			if seen[child.Name] {
				continue
			}
			seen[child.Name] = true
			branches = append(branches, child)
			descend(child.Name)
		}
	}
	descend(start)
	return branches
}

// runOn checks out b's tip, runs the command there with env added to its
// environment, and records the outcome in b. It returns an error only when the
// run as a whole cannot go on.
func runOn(ctx context.Context, sc *stcontext.StaccatoContext, b *BranchResult, env []string, opts Options) error {
	g := sc.Git
	// Detached, so a branch checked out in another worktree is no obstacle.
	// --no-overwrite-ignore: a branch that tracks a path which is an ignored
	// file here fails instead of overwriting that file.
	if _, err := g.Run("checkout", "--quiet", "--no-overwrite-ignore", "--detach", b.SHA); err != nil {
		fmt.Fprintln(opts.Output, err)
		b.Status = StatusFailed
		b.Note = "the branch could not be checked out: " + gitMessage(err)
		return nil
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", opts.Command)
	cmd.Dir = sc.RepoPath
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout, cmd.Stderr = opts.Output, opts.Output
	// Its own process group, so an interrupt reaches whatever it started.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
	cmd.WaitDelay = 5 * time.Second
	began := time.Now()
	runErr := cmd.Run()
	b.DurationMS = time.Since(began).Milliseconds()

	var exitErr *exec.ExitError
	switch {
	case ctx.Err() != nil:
		b.Status = StatusFailed
		b.Note = "interrupted"
	case runErr == nil:
		b.Status = StatusPassed
		b.ExitCode = new(int)
	case errors.As(runErr, &exitErr) && exitErr.ExitCode() >= 0:
		code := exitErr.ExitCode()
		b.Status = StatusFailed
		b.ExitCode = &code
	default:
		b.Status = StatusFailed
		b.Note = runErr.Error()
	}

	// What the command changed in tracked files would be carried into the
	// next branch and into the starting checkout. It is stashed rather than
	// discarded.
	changed, err := trackedChanges(sc)
	if err != nil {
		return err
	}
	if changed != "" {
		if _, err := g.Run("stash", "push", "--quiet", "-m", "st exec: changes made by the command on "+b.Branch); err != nil {
			return fmt.Errorf("branch %q: the command changed tracked files and they could not be stashed: %w", b.Branch, err)
		}
		b.addNote("the command changed tracked files; they were saved with 'git stash'")
	}
	return nil
}

func (b *BranchResult) addNote(note string) {
	if b.Note != "" {
		b.Note += "; "
	}
	b.Note += note
}

// trackedChanges returns the porcelain status of tracked files, empty when
// there are no staged or unstaged changes. Untracked files are ignored.
func trackedChanges(sc *stcontext.StaccatoContext) (string, error) {
	out, err := sc.Git.Output(nil, "", "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return "", fmt.Errorf("check working tree: %w", err)
	}
	return strings.TrimRight(out, "\n"), nil
}

// gitMessage reduces a git.Runner error to what git itself printed, on one
// line.
func gitMessage(err error) string {
	msg := err.Error()
	if _, printed, ok := strings.Cut(msg, "\nOutput: "); ok {
		msg = printed
	}
	return strings.Join(strings.Fields(msg), " ")
}
