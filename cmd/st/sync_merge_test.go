package main

import (
	stcontext "github.com/cpave3/staccato/pkg/context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncDetectsRegularMerge(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}

	// Push main to origin so origin/main exists
	repo.RunGit("push", "-u", "origin", root)

	// Create stack: main -> m1 -> m2
	runSt(t, "new", "m1")
	repo.CreateFile("m1.txt", "m1")
	repo.AddAndCommit("m1 commit")
	runSt(t, "append", "m2")
	repo.CreateFile("m2.txt", "m2")
	repo.AddAndCommit("m2 commit")

	// Push m1 to origin
	repo.RunGit("push", "origin", "m1")

	// Simulate regular merge: fast-forward origin/main to include m1,
	// then delete origin/m1
	originDir := repo.OriginDir()
	m1SHA, _ := repo.RunGit("rev-parse", "m1")
	// Update origin's trunk to m1's commit (simulating a merge)
	runGitInDir(t, originDir, "update-ref", "refs/heads/"+root, m1SHA)
	// Delete origin/m1
	runGitInDir(t, originDir, "branch", "-D", "m1")

	// Run sync
	repo.Checkout("m2")
	out := runSt(t, "-v", "sync")

	// m1 should be removed from graph
	if graphContains(t, repo, "m1") {
		t.Error("m1 should have been removed from graph")
	}

	// m2 should still exist with parent = root
	g := loadGraph(t, repo)
	m2, ok := g.Branches["m2"]
	if !ok {
		t.Fatal("m2 should still be in graph")
	}
	if m2.Parent != root {
		t.Errorf("m2 parent = %q, want %q", m2.Parent, root)
	}

	// m1 local branch should be deleted
	if repo.BranchExists("m1") {
		t.Error("m1 local branch should have been deleted")
	}

	assertContains(t, out, "Merged")
}

// ---------------------------------------------------------------------------
// TestSyncDetectsSquashMerge
// ---------------------------------------------------------------------------

func TestSyncDetectsSquashMerge(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create stack: main -> m1
	runSt(t, "new", "m1")
	repo.CreateFile("m1.txt", "m1 content")
	repo.AddAndCommit("m1 commit")

	// Push m1
	repo.RunGit("push", "origin", "m1")

	// Simulate squash merge: cherry-pick m1 changes onto main in origin,
	// then delete origin/m1
	originDir := repo.OriginDir()

	// Go back to main, cherry-pick m1's changes as a new commit
	repo.Checkout(root)
	repo.RunGit("cherry-pick", "m1")
	// Push this new main to origin
	repo.RunGit("push", "origin", root)
	// Delete origin/m1 (simulating GitHub deleting the branch after squash merge)
	runGitInDir(t, originDir, "branch", "-D", "m1")

	// Run sync
	out := runSt(t, "-v", "sync")

	// m1 should be removed
	if graphContains(t, repo, "m1") {
		t.Error("m1 should have been removed from graph after squash merge detection")
	}
	if repo.BranchExists("m1") {
		t.Error("m1 local branch should have been deleted")
	}

	assertContains(t, out, "Merged")
}

// ---------------------------------------------------------------------------
// TestSyncDetectsStackedSquashMerges
// ---------------------------------------------------------------------------
// Regression: with main <- m1 <- m2 <- m3 where m1 AND m2 are squash-merged,
// trunk contains m2's changes that aren't in m1, so a plain diff between
// trunk and m1 is non-empty and m1 escaped merged detection. Sync then
// replayed m1's already-merged commits onto main and hit conflicts.

func TestSyncDetectsStackedSquashMerges(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// m1 and m2 get two commits each so their squash commits have different
	// patch-ids than the originals — rebase can't silently drop the replayed
	// commits, which is what makes the real-world case conflict.
	runSt(t, "new", "m1")
	repo.CreateFile("m1.txt", "m1 line1\n")
	repo.AddAndCommit("m1 commit 1")
	repo.WriteFile("m1.txt", "m1 line1\nm1 line2\n")
	repo.AddAndCommit("m1 commit 2")
	runSt(t, "append", "m2")
	repo.CreateFile("m2.txt", "m2 line1\n")
	repo.AddAndCommit("m2 commit 1")
	repo.WriteFile("m2.txt", "m2 line1\nm2 line2\n")
	repo.AddAndCommit("m2 commit 2")
	runSt(t, "append", "m3")
	repo.CreateFile("m3.txt", "m3\n")
	repo.AddAndCommit("m3 commit")

	repo.RunGit("push", "origin", "m1")
	repo.RunGit("push", "origin", "m2")

	// Simulate GitHub squash-merging m1 then m2 into main.
	repo.Checkout(root)
	if out, err := repo.RunGit("merge", "--squash", "m1"); err != nil {
		t.Fatalf("squash merge m1: %v\n%s", err, out)
	}
	repo.RunGit("commit", "-m", "m1 (squash)")
	if out, err := repo.RunGit("merge", "--squash", "m2"); err != nil {
		t.Fatalf("squash merge m2: %v\n%s", err, out)
	}
	repo.RunGit("commit", "-m", "m2 (squash)")
	repo.RunGit("push", "origin", root)
	originDir := repo.OriginDir()
	runGitInDir(t, originDir, "branch", "-D", "m1")
	runGitInDir(t, originDir, "branch", "-D", "m2")

	// Sync from the top of the stack.
	repo.Checkout("m3")
	runSt(t, "-v", "sync", "--down")

	g := loadGraph(t, repo)
	if _, ok := g.Branches["m1"]; ok {
		t.Error("m1 should have been removed from graph")
	}
	if _, ok := g.Branches["m2"]; ok {
		t.Error("m2 should have been removed from graph")
	}
	if repo.BranchExists("m1") {
		t.Error("m1 local branch should have been deleted")
	}
	if repo.BranchExists("m2") {
		t.Error("m2 local branch should have been deleted")
	}

	m3, ok := g.Branches["m3"]
	if !ok {
		t.Fatal("m3 should still be in graph")
	}
	if m3.Parent != root {
		t.Errorf("m3 parent = %q, want %q", m3.Parent, root)
	}

	// m3 should sit directly on the new trunk with only its own commit.
	mergeBase, _ := repo.RunGit("merge-base", "m3", root)
	rootSHA, _ := repo.RunGit("rev-parse", root)
	if mergeBase != rootSHA {
		t.Errorf("m3 should be restacked onto %s, merge-base = %s", rootSHA, mergeBase)
	}
	count, _ := repo.RunGit("rev-list", "--count", root+"..m3")
	if count != "1" {
		t.Errorf("m3 should have exactly its own commit on top of trunk, got %s", count)
	}
}

// ---------------------------------------------------------------------------
// TestSyncDetectsSquashMergeWithAdvancedTrunk
// ---------------------------------------------------------------------------
// Regression: a squash-merged branch escaped detection whenever any other
// commit landed on trunk after the merge, because the diff between trunk and
// the branch was no longer empty.

func TestSyncDetectsSquashMergeWithAdvancedTrunk(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	runSt(t, "new", "m1")
	repo.CreateFile("m1.txt", "m1 line1\n")
	repo.AddAndCommit("m1 commit 1")
	repo.WriteFile("m1.txt", "m1 line1\nm1 line2\n")
	repo.AddAndCommit("m1 commit 2")
	repo.RunGit("push", "origin", "m1")

	// Squash-merge m1, then land an unrelated commit on trunk.
	repo.Checkout(root)
	if out, err := repo.RunGit("merge", "--squash", "m1"); err != nil {
		t.Fatalf("squash merge m1: %v\n%s", err, out)
	}
	repo.RunGit("commit", "-m", "m1 (squash)")
	repo.CreateFile("other.txt", "unrelated\n")
	repo.AddAndCommit("unrelated change")
	repo.RunGit("push", "origin", root)
	runGitInDir(t, repo.OriginDir(), "branch", "-D", "m1")

	runSt(t, "-v", "sync", "--down")

	if graphContains(t, repo, "m1") {
		t.Error("m1 should have been removed from graph after squash merge detection")
	}
	if repo.BranchExists("m1") {
		t.Error("m1 local branch should have been deleted")
	}
}

// ---------------------------------------------------------------------------
// TestSyncRestackIgnoresForeignBaseSHA
// ---------------------------------------------------------------------------
// Regression: if a branch's recorded base_sha points at a commit that isn't
// in the branch's history (e.g. metadata recorded from another branch), the
// fork-point picker kept it, and `rebase --onto parent <base_sha>` replayed
// large stretches of mainline history instead of just the branch's own
// commits, conflicting. A fork-point candidate must be an ancestor of the
// branch to be usable.

func TestSyncRestackRejectsForeignBaseSHA(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}

	repo.CreateFile("f.txt", "v0\n")
	repo.AddAndCommit("f v0")

	// Side branch from this point; its head is NOT an ancestor of m1.
	repo.RunGit("checkout", "-b", "foreign")
	repo.CreateFile("foreign.txt", "foreign\n")
	repo.AddAndCommit("foreign commit")
	foreignSHA := repo.HeadSHA()
	repo.Checkout(root)

	// Two sequential edits to the same file on trunk: replaying the first
	// edit on top of the second is guaranteed to conflict.
	repo.WriteFile("f.txt", "v1\n")
	repo.AddAndCommit("f v1")
	repo.WriteFile("f.txt", "v2\n")
	repo.AddAndCommit("f v2")
	repo.RunGit("push", "-u", "origin", root)

	runSt(t, "new", "m1")
	repo.CreateFile("m1.txt", "m1\n")
	repo.AddAndCommit("m1 commit")

	// Corrupt the graph: point m1's base_sha at the foreign branch head.
	sc, err := stcontext.Load(repo.Dir)
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	sc.Graph.Branches["m1"].BaseSHA = foreignSHA
	if err := sc.Save(); err != nil {
		t.Fatalf("save graph: %v", err)
	}

	out := runStExpectError(t, "-v", "sync", "--down")
	assertContains(t, out, "st graph repair")

}

// ---------------------------------------------------------------------------
// TestSyncMultipleMergedBranches
// ---------------------------------------------------------------------------

func TestSyncMultipleMergedBranches(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create stack: main -> m1 -> m2 -> m3
	runSt(t, "new", "m1")
	repo.CreateFile("m1.txt", "m1")
	repo.AddAndCommit("m1 commit")
	runSt(t, "append", "m2")
	repo.CreateFile("m2.txt", "m2")
	repo.AddAndCommit("m2 commit")
	runSt(t, "append", "m3")
	repo.CreateFile("m3.txt", "m3")
	repo.AddAndCommit("m3 commit")

	// Push m1 and m2
	repo.RunGit("push", "origin", "m1")
	repo.RunGit("push", "origin", "m2")

	// Simulate both m1 and m2 merged: fast-forward origin/trunk to m2
	originDir := repo.OriginDir()
	m2SHA, _ := repo.RunGit("rev-parse", "m2")
	runGitInDir(t, originDir, "update-ref", "refs/heads/"+root, m2SHA)
	runGitInDir(t, originDir, "branch", "-D", "m1")
	runGitInDir(t, originDir, "branch", "-D", "m2")

	// Run sync from m3
	repo.Checkout("m3")
	runSt(t, "-v", "sync")

	// m1 and m2 should be gone
	g := loadGraph(t, repo)
	if _, ok := g.Branches["m1"]; ok {
		t.Error("m1 should have been removed")
	}
	if _, ok := g.Branches["m2"]; ok {
		t.Error("m2 should have been removed")
	}

	// m3 should be reparented to root
	m3, ok := g.Branches["m3"]
	if !ok {
		t.Fatal("m3 should still be in graph")
	}
	if m3.Parent != root {
		t.Errorf("m3 parent = %q, want %q", m3.Parent, root)
	}
}

// ---------------------------------------------------------------------------
// TestSyncChildSurvivesSecondSync
// ---------------------------------------------------------------------------
// Regression: after merging a middle branch (root <- m1 <- m2) and running
// sync twice, m2 should still be in the graph reparented to root.

func TestSyncSkipsUnpushedBranches(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create m1 with unique content — never push it
	runSt(t, "new", "m1")
	repo.CreateFile("m1-unique.txt", "unique content that is NOT on main")
	repo.AddAndCommit("m1 commit")

	// Run sync
	runSt(t, "-v", "sync")

	// m1 should still be in graph (not removed — it was never pushed
	// and has unique content not on main)
	if !graphContains(t, repo, "m1") {
		t.Error("m1 should NOT have been removed — it was never pushed and has unique diff")
	}
	if !repo.BranchExists("m1") {
		t.Error("m1 local branch should still exist")
	}
}

// ---------------------------------------------------------------------------
// TestSyncDefersMergedBranchCheckedOutInWorktree
// ---------------------------------------------------------------------------

func TestSyncDefersMergedBranchCheckedOutInWorktree(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	runSt(t, "new", "merged-worktree-branch")
	repo.CreateFile("merged.txt", "merged")
	repo.AddAndCommit("merged branch")
	mergedSHA, _ := repo.RunGit("rev-parse", "merged-worktree-branch")
	repo.RunGit("push", "origin", "merged-worktree-branch")
	repo.Checkout(root)

	worktreeDir := filepath.Join(t.TempDir(), "merged-worktree")
	if out, err := repo.RunGit("worktree", "add", worktreeDir, "merged-worktree-branch"); err != nil {
		t.Fatalf("add worktree: %v\n%s", err, out)
	}

	originDir := repo.OriginDir()
	runGitInDir(t, originDir, "update-ref", "refs/heads/"+root, mergedSHA)
	runGitInDir(t, originDir, "branch", "-D", "merged-worktree-branch")

	out := runSt(t, "sync", "--down")
	assertContains(t, out, "Merged branch 'merged-worktree-branch' is checked out in another worktree; local deletion deferred")
	if graphContains(t, repo, "merged-worktree-branch") {
		t.Error("merged branch should be removed from the stack graph")
	}
	if !repo.BranchExists("merged-worktree-branch") {
		t.Error("merged branch should remain while checked out in a worktree")
	}
}

// ---------------------------------------------------------------------------
// TestSyncUserOnMergedBranch
// ---------------------------------------------------------------------------

func TestSyncUserOnMergedBranch(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create m1
	runSt(t, "new", "m1")
	repo.CreateFile("m1.txt", "m1")
	repo.AddAndCommit("m1 commit")

	// Push m1
	repo.RunGit("push", "origin", "m1")

	// Simulate merge
	originDir := repo.OriginDir()
	m1SHA, _ := repo.RunGit("rev-parse", "m1")
	runGitInDir(t, originDir, "update-ref", "refs/heads/"+root, m1SHA)
	runGitInDir(t, originDir, "branch", "-D", "m1")

	// Stay on m1 (the merged branch)
	repo.Checkout("m1")

	// Run sync
	runSt(t, "-v", "sync")

	// Should end up on trunk
	cur := getCurrentBranch(t, repo)
	if cur != root {
		t.Errorf("current branch = %q, want %q (trunk)", cur, root)
	}

	// m1 should be gone
	if repo.BranchExists("m1") {
		t.Error("m1 should have been deleted")
	}
}

// runGitInDir runs a git command in a specific directory (for bare repo manipulation)
func runGitInDir(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s failed: %v\nOutput: %s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// ---------------------------------------------------------------------------
// TestSyncDown
// ---------------------------------------------------------------------------

func TestSyncStashesUncommittedOnMergedBranch(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create branch m1 with a commit
	runSt(t, "new", "m1")
	repo.CreateFile("m1.txt", "m1")
	repo.AddAndCommit("m1 commit")
	repo.RunGit("push", "origin", "m1")

	// Simulate m1 merged on remote
	originDir := repo.OriginDir()
	m1SHA, _ := repo.RunGit("rev-parse", "m1")
	runGitInDir(t, originDir, "update-ref", "refs/heads/"+root, m1SHA)
	runGitInDir(t, originDir, "branch", "-D", "m1")

	// User is on m1 with uncommitted work
	repo.Checkout("m1")
	repo.WriteFile("wip.txt", "work in progress")
	repo.RunGit("add", "wip.txt")

	// Run sync — should stash the uncommitted changes
	out := runSt(t, "-v", "sync")
	assertContains(t, out, "Stashing")

	// Should end up on trunk
	cur := getCurrentBranch(t, repo)
	if cur != root {
		t.Errorf("current branch = %q, want %q", cur, root)
	}

	// m1 should be gone
	if repo.BranchExists("m1") {
		t.Error("m1 should have been deleted")
	}

	// Stash should contain our work
	stashOut, _ := repo.RunGit("stash", "list")
	if !strings.Contains(stashOut, "st-sync") {
		t.Errorf("stash should contain st-sync entry, got: %s", stashOut)
	}
}

// ---------------------------------------------------------------------------
// TestSyncSkipsNewEmptyBranch
// ---------------------------------------------------------------------------

func TestSyncSkipsNewEmptyBranch(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create a new empty branch (no commits)
	runSt(t, "new", "empty-branch")

	// Run sync — the empty branch should NOT be removed
	runSt(t, "sync")

	// Branch should still exist in git
	if !repo.BranchExists("empty-branch") {
		t.Error("empty-branch should still exist in git after sync")
	}

	// Branch should still exist in graph
	if !graphContains(t, repo, "empty-branch") {
		t.Error("empty-branch should still be in graph after sync")
	}

	// Branch should have been pushed to remote
	remoteOut, _ := repo.RunGit("ls-remote", "--heads", "origin", "empty-branch")
	if strings.TrimSpace(remoteOut) == "" {
		t.Error("empty-branch should have been pushed to remote")
	}
}
