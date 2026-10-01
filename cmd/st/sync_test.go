package main

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TestSync
// ---------------------------------------------------------------------------

func TestSync(t *testing.T) {
	t.Run("dry_run_does_not_push", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		if err := repo.AddRemote(); err != nil {
			t.Fatalf("AddRemote: %v", err)
		}
		repo.RunGit("push", "-u", "origin", root)

		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")

		// dry-run output should be visible WITHOUT -v flag
		out := runSt(t, "sync", "--dry-run")
		assertContains(t, out, "Would push")
		assertContains(t, out, "f1")

		// f1 should NOT have been pushed
		remoteOut, _ := repo.RunGit("ls-remote", "--heads", "origin", "f1")
		if strings.TrimSpace(remoteOut) != "" {
			t.Error("f1 should NOT have been pushed with --dry-run")
		}
	})

	t.Run("dry_run_does_not_modify_local_branches", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		if err := repo.AddRemote(); err != nil {
			t.Fatalf("AddRemote: %v", err)
		}
		repo.RunGit("push", "-u", "origin", root)

		// Create stack: root -> m1
		runSt(t, "new", "m1")
		repo.CreateFile("m1.txt", "m1")
		repo.AddAndCommit("m1 commit")
		repo.RunGit("push", "origin", "m1")

		// Simulate m1 merged: fast-forward origin/trunk to include m1
		originDir := repo.OriginDir()
		m1SHA, _ := repo.RunGit("rev-parse", "m1")
		runGitInDir(t, originDir, "update-ref", "refs/heads/"+root, m1SHA)
		runGitInDir(t, originDir, "branch", "-D", "m1")

		// Record state before dry-run
		m1SHABefore, _ := repo.RunGit("rev-parse", "m1")

		// Run sync --dry-run — output should be visible WITHOUT -v flag
		out := runSt(t, "sync", "--dry-run")
		assertContains(t, out, "Would remove merged branch")

		// m1 local branch should still exist (not deleted)
		if !repo.BranchExists("m1") {
			t.Error("m1 should still exist after --dry-run (no local modifications)")
		}

		// m1 SHA should be unchanged
		m1SHAAfter, _ := repo.RunGit("rev-parse", "m1")
		if m1SHAAfter != m1SHABefore {
			t.Error("m1 SHA should be unchanged after --dry-run")
		}

		// Graph should still contain m1 (not removed)
		if !graphContains(t, repo, "m1") {
			t.Error("m1 should still be in graph after --dry-run")
		}
	})

	t.Run("error_no_remote", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")
		out := runStExpectError(t, "sync")
		assertContains(t, out, "no remote")
	})
}
