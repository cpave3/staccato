package main

import (
	"testing"
)

// ---------------------------------------------------------------------------
// TestLog
// ---------------------------------------------------------------------------

func TestLog(t *testing.T) {
	t.Run("displays_tree", func(t *testing.T) {
		_, root := setupRepoWithStack(t)
		runSt(t, "new", "f1")

		out := runSt(t, "log")
		assertContains(t, out, root)
		assertContains(t, out, "f1")
	})

	t.Run("shows_current_marker", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")

		out := runSt(t, "log")
		assertContains(t, out, "●")
	})
}

// ---------------------------------------------------------------------------
// TestStalenessWarningOnLog
// ---------------------------------------------------------------------------

func TestStalenessWarningOnLog(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create a branch
	runSt(t, "new", "f1")
	repo.CreateFile("f1.txt", "f1")
	repo.AddAndCommit("f1 commit")
	repo.RunGit("push", "origin", "f1")

	// Advance origin/main (simulating a PR merge on another machine)
	originDir := repo.OriginDir()
	f1SHA, _ := repo.RunGit("rev-parse", "f1")
	runGitInDir(t, originDir, "update-ref", "refs/heads/"+root, f1SHA)

	// Fetch to get the new remote tracking ref (but don't sync)
	repo.RunGit("fetch", "origin")

	// Now local main is behind origin/main — log should warn
	repo.Checkout(root)
	out := runSt(t, "log")
	assertContains(t, out, "behind remote")
}
