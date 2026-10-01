package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncDown(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create branch with a commit
	runSt(t, "new", "d1")
	repo.CreateFile("d1.txt", "d1")
	repo.AddAndCommit("d1 commit")

	// Run sync --down
	runSt(t, "-v", "sync", "--down")

	// d1 should still be in graph
	if !graphContains(t, repo, "d1") {
		t.Error("d1 should still be in graph")
	}

	// d1 should NOT have been pushed to origin
	out, _ := repo.RunGit("ls-remote", "--heads", "origin", "d1")
	if strings.TrimSpace(out) != "" {
		t.Error("d1 should NOT have been pushed to origin with --down flag")
	}
}

// ---------------------------------------------------------------------------
// TestSyncOnlyPushesCurrentLineage
// ---------------------------------------------------------------------------

func TestSyncOnlyPushesCurrentLineage(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create two independent stacks off root:
	// root -> stack-a1 -> stack-a2
	// root -> stack-b1
	runSt(t, "new", "stack-a1")
	repo.CreateFile("a1.txt", "a1")
	repo.AddAndCommit("a1 commit")
	runSt(t, "append", "stack-a2")
	repo.CreateFile("a2.txt", "a2")
	repo.AddAndCommit("a2 commit")

	repo.Checkout(root)
	runSt(t, "new", "stack-b1")
	repo.CreateFile("b1.txt", "b1")
	repo.AddAndCommit("b1 commit")

	// Sync from stack-a2 — only stack-a1 and stack-a2 should be pushed
	repo.Checkout("stack-a2")
	runSt(t, "-v", "sync")

	// stack-a1 should have been pushed
	outA1, _ := repo.RunGit("ls-remote", "--heads", "origin", "stack-a1")
	if strings.TrimSpace(outA1) == "" {
		t.Error("stack-a1 should have been pushed")
	}

	// stack-a2 should have been pushed
	outA2, _ := repo.RunGit("ls-remote", "--heads", "origin", "stack-a2")
	if strings.TrimSpace(outA2) == "" {
		t.Error("stack-a2 should have been pushed")
	}

	// stack-b1 should NOT have been pushed — it's a different lineage
	outB1, _ := repo.RunGit("ls-remote", "--heads", "origin", "stack-b1")
	if strings.TrimSpace(outB1) != "" {
		t.Error("stack-b1 should NOT have been pushed — it's not in the current lineage")
	}
}

// ---------------------------------------------------------------------------
// TestSyncDownRestacks
// ---------------------------------------------------------------------------

func TestSyncDownRestacks(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create stack: root -> f1
	runSt(t, "new", "f1")
	repo.CreateFile("f1.txt", "f1")
	repo.AddAndCommit("f1 commit")

	// Push f1 so it has a remote tracking branch
	repo.RunGit("push", "origin", "f1")

	// Simulate upstream change: clone origin, commit, push back
	tmpClone, _ := os.MkdirTemp("", "st-clone-*")
	defer os.RemoveAll(tmpClone)
	cloneCmd := exec.Command("git", "clone", repo.OriginDir(), tmpClone)
	cloneCmd.Run()
	exec.Command("git", "-C", tmpClone, "config", "user.email", "other@example.com").Run()
	exec.Command("git", "-C", tmpClone, "config", "user.name", "Other").Run()
	os.WriteFile(filepath.Join(tmpClone, "upstream-change.txt"), []byte("new upstream content"), 0644)
	exec.Command("git", "-C", tmpClone, "add", ".").Run()
	exec.Command("git", "-C", tmpClone, "commit", "-m", "upstream commit").Run()
	exec.Command("git", "-C", tmpClone, "push", "origin", root).Run()

	// Run sync --down
	repo.Checkout("f1")
	runSt(t, "-v", "sync", "--down")

	// trunk should have the upstream change (fast-forwarded)
	repo.Checkout(root)
	if !repo.FileExists("upstream-change.txt") {
		t.Fatal("trunk should have upstream-change.txt after sync --down")
	}

	// f1 should also have the upstream change (restacked onto updated trunk)
	repo.Checkout("f1")
	if !repo.FileExists("upstream-change.txt") {
		t.Error("f1 should have upstream-change.txt after sync --down restacks onto updated trunk")
	}
}
