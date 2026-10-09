package split

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/cpave3/staccato/pkg/backup"
	stcontext "github.com/cpave3/staccato/pkg/context"
)

// Result reports what a split produced.
type Result struct {
	Branch string `json:"branch"`
	Parent string `json:"parent"`
	// DryRun is set when the slices were built but no branch was moved.
	DryRun  bool          `json:"dry_run"`
	Applied bool          `json:"applied"`
	Slices  []SliceResult `json:"slices"`
	// Verified is the command that passed in every slice.
	Verified string `json:"verified,omitempty"`
	// Backup names the branch that keeps the original commits. It is set when
	// the split rewrote them.
	Backup string `json:"backup,omitempty"`
	// Removed names the original branch when no slice kept its name.
	Removed string `json:"removed,omitempty"`
	// Moved lists the descendants re-parented onto the top slice.
	Moved []string `json:"moved,omitempty"`
	// NeedsRestack lists descendants that were already behind their parent
	// before the split. They are left untouched; 'st restack' brings them up.
	NeedsRestack []string `json:"needs_restack,omitempty"`
	// Warnings are things worth knowing about a split that is otherwise fine.
	Warnings []string `json:"warnings,omitempty"`
}

// SliceResult describes one branch of the resulting stack.
type SliceResult struct {
	Name       string `json:"name"`
	Parent     string `json:"parent"`
	SHA        string `json:"sha"`
	Commits    int    `json:"commits"`
	Files      int    `json:"files"`
	Insertions int    `json:"insertions"`
	Deletions  int    `json:"deletions"`
}

// Options controls how a plan is applied.
type Options struct {
	// DryRun builds the slices and reports them without moving any branch.
	// The commits it reports exist as unreferenced objects and can be
	// inspected.
	DryRun bool
	// Exec is a shell command to run in each slice before any branch moves.
	// If it fails in any slice the split is not applied.
	Exec string
	// ExecOutput receives the output of Exec as it runs. When nil, the tail
	// of a failing run is included in the error instead.
	ExecOutput io.Writer
	// BaseDir is the directory that relative override sources resolve against.
	BaseDir string
}

// Apply splits a branch according to plan. Every slice is built before any
// branch moves, so a plan that cannot be carried out changes nothing.
func Apply(sc *stcontext.StaccatoContext, plan *Plan, opts Options) (*Result, error) {
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	t, err := resolveTarget(sc, plan.Branch)
	if err != nil {
		return nil, err
	}

	if err := preflight(sc, t, plan); err != nil {
		return nil, err
	}

	b, cleanup, err := newBuilder(sc.Git, t, opts.BaseDir)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	built, err := b.build(plan.Slices)
	if err != nil {
		return nil, err
	}
	top := built[len(built)-1]
	if dir, elsewhere := checkedOutElsewhere(sc, t.name); elsewhere && top.name != t.name {
		return nil, fmt.Errorf("branch %q is checked out in another worktree (%s), so only the top slice can take its name — name the last slice %q, or run the split from that worktree", t.name, dir, t.name)
	}

	// Descendants only have to move when the split rewrote the branch's tip.
	var moved []movedBranch
	var stale []string
	if top.sha != t.head {
		if moved, stale, err = b.moveDescendants(sc.Graph, t, top.sha); err != nil {
			return nil, err
		}
	}

	result, err := summarise(sc, t, built)
	if err != nil {
		return nil, err
	}
	for _, m := range moved {
		result.Moved = append(result.Moved, m.name)
	}
	result.NeedsRestack = stale
	result.Warnings = b.warnings

	if opts.Exec != "" {
		if err := verify(sc, built, opts.Exec, opts.ExecOutput); err != nil {
			return nil, err
		}
		result.Verified = opts.Exec
	}
	if opts.DryRun {
		result.DryRun = true
		return result, nil
	}
	if err := moveBranches(sc, t, built, moved, result); err != nil {
		return nil, err
	}
	result.Applied = true
	return result, nil
}

// preflight refuses a split that the state of the repository rules out.
func preflight(sc *stcontext.StaccatoContext, t *target, plan *Plan) error {
	if plan.Head != "" {
		expected, err := sc.Git.RunWith(nil, "", "rev-parse", "--verify", "--quiet", "--end-of-options", plan.Head+"^{commit}")
		if err != nil || expected != t.head {
			return fmt.Errorf("branch %q has moved since the plan was written (the plan expects %s, the branch is at %s) — run 'st split describe' again", t.name, plan.Head, t.head)
		}
	}
	if !t.upToDate {
		return fmt.Errorf("branch %q is behind its parent %q — run 'st restack' first", t.name, t.parent)
	}
	if inProgress, _ := sc.Git.IsRebaseInProgress(); inProgress {
		return fmt.Errorf("a rebase is in progress — finish it with 'st continue' or 'st abort' first")
	}
	return checkNames(sc, t, plan)
}

// checkNames makes sure every slice can get the branch name it asks for.
func checkNames(sc *stcontext.StaccatoContext, t *target, plan *Plan) error {
	existing, err := sc.Git.GetAllBranches()
	if err != nil {
		return fmt.Errorf("list branches: %w", err)
	}
	// Branches that will exist next to the new ones once the split is done.
	remaining := map[string]bool{}
	for _, name := range existing {
		remaining[name] = true
	}
	reused := false
	for _, s := range plan.Slices {
		if s.Name == t.name {
			reused = true
		}
	}
	switch {
	case !reused && !plan.DropOriginal:
		return fmt.Errorf(`no slice is named %q: name the top slice %q so that its pull request and child branches stay attached, or set "drop_original": true to delete the branch`, t.name, t.name)
	case reused && plan.DropOriginal:
		return fmt.Errorf(`"drop_original" is set, but a slice is named %q — remove one or the other`, t.name)
	}
	if !reused {
		delete(remaining, t.name)
	}

	for _, s := range plan.Slices {
		if s.Name == t.name {
			continue
		}
		if _, err := sc.Git.Run("check-ref-format", "--branch", s.Name); err != nil {
			return fmt.Errorf("slice name %q is not a valid branch name", s.Name)
		}
		_, tracked := sc.Graph.GetBranch(s.Name)
		if remaining[s.Name] || tracked || s.Name == sc.Graph.Root {
			return fmt.Errorf("branch %q already exists — choose another name for that slice", s.Name)
		}
	}
	for _, s := range plan.Slices {
		if s.Name != t.name {
			remaining[s.Name] = true
		}
	}
	for _, s := range plan.Slices {
		for other := range remaining {
			if strings.HasPrefix(s.Name, other+"/") || strings.HasPrefix(other, s.Name+"/") {
				return fmt.Errorf("branch %q cannot exist alongside branch %q: git cannot hold a branch and branches nested under its name", s.Name, other)
			}
		}
	}
	return nil
}

// checkedOutElsewhere reports whether branch is checked out in a worktree
// other than the one the split runs in, and where.
func checkedOutElsewhere(sc *stcontext.StaccatoContext, branch string) (string, bool) {
	dir, checkedOut, err := sc.Git.BranchWorktree(branch)
	if err != nil || !checkedOut {
		return "", false
	}
	return dir, !samePath(dir, sc.RepoPath)
}

func samePath(a, b string) bool {
	if resolved, err := filepath.EvalSymlinks(a); err == nil {
		a = resolved
	}
	if resolved, err := filepath.EvalSymlinks(b); err == nil {
		b = resolved
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// summarise measures each built slice against the one below it.
func summarise(sc *stcontext.StaccatoContext, t *target, built []builtSlice) (*Result, error) {
	result := &Result{Branch: t.name, Parent: t.parent}
	parent, prev := t.parent, t.base
	for _, s := range built {
		changes, err := diffChanges(sc.Git, prev, s.sha)
		if err != nil {
			return nil, err
		}
		commits, err := listCommits(sc.Git, prev, s.sha)
		if err != nil {
			return nil, err
		}
		sr := SliceResult{Name: s.name, Parent: parent, SHA: s.sha, Commits: len(commits), Files: len(changes)}
		for _, ch := range changes {
			sr.Insertions += ch.insertions
			sr.Deletions += ch.deletions
		}
		result.Slices = append(result.Slices, sr)
		parent, prev = s.name, s.sha
	}
	return result, nil
}

// moveBranches points every branch at its new commit in one step and rewrites
// the stack graph to match.
func moveBranches(sc *stcontext.StaccatoContext, t *target, built []builtSlice, moved []movedBranch, result *Result) error {
	g := sc.Git
	top := built[len(built)-1]

	// Once the branch moves nothing else points at commits the split rewrote,
	// so keep them reachable. A split that only cuts at existing commits
	// rewrites nothing and needs no backup.
	if top.sha != t.head {
		label, err := backup.NewManager(g, sc.RepoPath).CreateManualBackup([]string{t.name})
		if err != nil {
			return fmt.Errorf("back up %q: %w", t.name, err)
		}
		result.Backup = strings.Join([]string{backup.BackupPrefix, backup.ManualSubpath, label, t.name}, "/")
	}

	// A ref can move underneath a checkout when the content stays the same,
	// which is the case for the top slice and for every moved descendant. HEAD
	// only has to step aside when the original branch is checked out here and
	// its name goes to a lower slice or goes away.
	current, _ := g.GetCurrentBranch()
	relocate := current == t.name && top.name != t.name
	detached := false
	rollback := func() {
		if detached {
			g.CheckoutBranch(t.name)
		}
		if result.Backup != "" {
			g.DeleteBranch(result.Backup, true)
			result.Backup = ""
		}
	}
	if relocate {
		if _, err := g.Run("checkout", "--quiet", "--detach"); err != nil {
			rollback()
			return fmt.Errorf("detach HEAD: %w", err)
		}
		detached = true
	}

	// One transaction: either every branch moves or none does. The expected
	// old values also catch a branch that moved while the slices were built.
	var refs, undo strings.Builder
	reused := false
	for _, s := range built {
		if s.name == t.name {
			reused = true
			fmt.Fprintf(&refs, "update refs/heads/%s %s %s\n", s.name, s.sha, t.head)
			fmt.Fprintf(&undo, "update refs/heads/%s %s %s\n", s.name, t.head, s.sha)
		} else {
			fmt.Fprintf(&refs, "create refs/heads/%s %s\n", s.name, s.sha)
			fmt.Fprintf(&undo, "delete refs/heads/%s %s\n", s.name, s.sha)
		}
	}
	if !reused {
		fmt.Fprintf(&refs, "delete refs/heads/%s %s\n", t.name, t.head)
		fmt.Fprintf(&undo, "create refs/heads/%s %s\n", t.name, t.head)
		result.Removed = t.name
	}
	for _, m := range moved {
		fmt.Fprintf(&refs, "update refs/heads/%s %s %s\n", m.name, m.newTip, m.oldTip)
		fmt.Fprintf(&undo, "update refs/heads/%s %s %s\n", m.name, m.oldTip, m.newTip)
	}
	if _, err := g.RunWith(nil, refs.String(), "update-ref", "-m", "st split: "+t.name, "--stdin"); err != nil {
		rollback()
		return fmt.Errorf("nothing was changed: the branches could not be moved together (if one of them moved meanwhile, run 'st split describe' again): %w", err)
	}

	children := childNames(sc.Graph, t.name)
	sc.Graph.RemoveBranch(t.name)
	parent, base := t.parent, t.base
	for _, s := range built {
		sc.Graph.AddBranch(s.name, parent, base, s.sha)
		parent, base = s.name, s.sha
	}
	// Children that were already behind keep their recorded fork point, which
	// is what a later restack replays from.
	for _, name := range children {
		sc.Graph.Branches[name].Parent = top.name
	}
	for _, m := range moved {
		sc.Graph.UpdateBranch(m.name, m.base, m.newTip)
	}
	if err := sc.Save(); err != nil {
		g.RunWith(nil, undo.String(), "update-ref", "-m", "st split: undo", "--stdin")
		rollback()
		return fmt.Errorf("save graph: %w", err)
	}

	if relocate {
		if err := g.CheckoutBranch(top.name); err != nil {
			return fmt.Errorf("check out %q: %w", top.name, err)
		}
	}
	return nil
}
