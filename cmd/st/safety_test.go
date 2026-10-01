package main

import (
	"testing"
)

// ---------------------------------------------------------------------------
// TestDetachedHEADGuard
// ---------------------------------------------------------------------------

func TestDetachedHEADGuard(t *testing.T) {
	t.Run("log_with_detached_HEAD_returns_error", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)

		// Create a branch so the graph has content
		runSt(t, "new", "feature-1")

		// Detach HEAD
		sha := repo.HeadSHA()
		repo.RunGit("checkout", sha)

		out := runStExpectError(t, "log")
		assertContains(t, out, "HEAD is detached")
		assertContains(t, out, "check out a branch first")
	})
}

// ---------------------------------------------------------------------------
// TestBranchAlreadyExists
// ---------------------------------------------------------------------------

func TestBranchAlreadyExists(t *testing.T) {
	t.Run("new_with_existing_branch_suggests_attach", func(t *testing.T) {
		_, _ = setupRepoWithStack(t)

		runSt(t, "new", "feature-1")

		out := runStExpectError(t, "new", "feature-1")
		assertContains(t, out, "branch 'feature-1' already exists")
		assertContains(t, out, "st attach feature-1")
	})

	t.Run("append_with_existing_branch_suggests_attach", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		runSt(t, "new", "feature-1")
		repo.Checkout(root)

		out := runStExpectError(t, "append", "feature-1")
		assertContains(t, out, "branch 'feature-1' already exists")
		assertContains(t, out, "st attach feature-1")
	})
}

// ---------------------------------------------------------------------------
// TestContinueSafetyCheck
// ---------------------------------------------------------------------------

func TestContinueSafetyCheck(t *testing.T) {
	t.Run("continue_with_manual_rebase_suggests_git_rebase_continue", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Create two branches with conflicting changes
		runSt(t, "new", "feature-1")
		repo.CreateFile("conflict.txt", "feature-1 content")
		repo.AddAndCommit("feature-1 commit")

		repo.Checkout(root)
		repo.CreateFile("conflict.txt", "root content")
		repo.AddAndCommit("root commit")

		// Start a manual git rebase (not via st)
		repo.RunGit("rebase", "feature-1")
		// This should fail with conflicts — if it doesn't, skip
		if !rebaseInProgress(t, repo) {
			t.Skip("manual rebase didn't produce conflicts")
		}

		// st continue should detect no restack state
		out := runStExpectError(t, "continue")
		assertContains(t, out, "no st restack in progress")
		assertContains(t, out, "git rebase --continue")

		// Cleanup: abort the rebase
		repo.RunGit("rebase", "--abort")
	})
}

// ---------------------------------------------------------------------------
// TestDirtyTreeWarning
// ---------------------------------------------------------------------------

func TestDirtyTreeWarning(t *testing.T) {
	t.Run("restack_with_dirty_tree_warns_but_proceeds", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)

		// Create a branch
		runSt(t, "new", "feature-1")
		repo.CreateFile("feature.txt", "feature content")
		repo.AddAndCommit("feature commit")

		// Create uncommitted changes
		repo.WriteFile("dirty.txt", "uncommitted")

		// Restack should warn but succeed
		out := runSt(t, "-v", "restack")
		assertContains(t, out, "uncommitted changes")
	})
}
