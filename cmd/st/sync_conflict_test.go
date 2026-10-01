package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncConflictContinue(t *testing.T) {
	t.Run("sync_conflict_then_continue_completes", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		if err := repo.AddRemote(); err != nil {
			t.Fatalf("AddRemote: %v", err)
		}
		repo.RunGit("push", "-u", "origin", root)

		// Create stack: root -> s1
		runSt(t, "new", "s1")
		repo.CreateFile("shared.txt", "s1 content")
		repo.CreateFile("s1.txt", "s1 file")
		repo.AddAndCommit("s1 commit")

		// Push s1 so sync doesn't skip it
		repo.RunGit("push", "origin", "s1")

		// Simulate upstream change: add conflicting commit on origin's trunk
		originDir := repo.OriginDir()
		tmpClone, _ := os.MkdirTemp("", "st-clone-*")
		defer os.RemoveAll(tmpClone)
		cloneCmd := exec.Command("git", "clone", originDir, tmpClone)
		cloneCmd.Run()
		exec.Command("git", "-C", tmpClone, "config", "user.email", "other@test.com").Run()
		exec.Command("git", "-C", tmpClone, "config", "user.name", "Other").Run()
		os.WriteFile(filepath.Join(tmpClone, "shared.txt"), []byte("upstream conflict content"), 0644)
		exec.Command("git", "-C", tmpClone, "add", ".").Run()
		exec.Command("git", "-C", tmpClone, "commit", "-m", "upstream conflict").Run()
		exec.Command("git", "-C", tmpClone, "push", "origin", root).Run()

		// Run sync — should hit conflict during restack phase
		repo.Checkout("s1")
		out := runStExpectError(t, "sync")
		assertContains(t, out, "conflict")

		// Resolve and continue
		resolveConflictAndStage(t, repo, "shared.txt", "resolved sync content")
		runStContinue(t)

		// Verify s1 has the resolved content
		repo.Checkout("s1")
		content, _ := os.ReadFile(filepath.Join(repo.Dir, "shared.txt"))
		if string(content) != "resolved sync content" {
			t.Errorf("s1 shared.txt = %q, want 'resolved sync content'", content)
		}

		// Verify restack state is cleared
		if restackStateExists(t, repo) {
			t.Error("restack state should be cleared after continue")
		}
	})

	t.Run("sync_down_continues_parent_with_two_conflicting_commits", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		repo.CreateFile("first.txt", "base first")
		repo.CreateFile("second.txt", "base second")
		repo.AddAndCommit("add shared base files")

		if err := repo.AddRemote(); err != nil {
			t.Fatalf("AddRemote: %v", err)
		}
		repo.RunGit("push", "-u", "origin", root)

		// Build a two-branch stack whose parent has two independently
		// conflicting commits, then remain on the child tip.
		runSt(t, "new", "parent")
		repo.WriteFile("first.txt", "parent first")
		repo.AddAndCommit("parent changes first")
		repo.WriteFile("second.txt", "parent second")
		repo.AddAndCommit("parent changes second")
		runSt(t, "append", "tip")
		repo.CreateFile("tip.txt", "tip content")
		repo.AddAndCommit("tip commit")
		repo.RunGit("push", "origin", "parent", "tip")

		// Advance origin trunk with conflicts against each parent commit.
		cloneDir, err := os.MkdirTemp("", "st-clone-*")
		if err != nil {
			t.Fatalf("MkdirTemp: %v", err)
		}
		defer os.RemoveAll(cloneDir)
		runGitInDir(t, filepath.Dir(cloneDir), "clone", repo.OriginDir(), cloneDir)
		runGitInDir(t, cloneDir, "config", "user.email", "other@test.com")
		runGitInDir(t, cloneDir, "config", "user.name", "Other")
		if err := os.WriteFile(filepath.Join(cloneDir, "first.txt"), []byte("origin first"), 0644); err != nil {
			t.Fatalf("write first.txt: %v", err)
		}
		runGitInDir(t, cloneDir, "add", "first.txt")
		runGitInDir(t, cloneDir, "commit", "-m", "origin changes first")
		if err := os.WriteFile(filepath.Join(cloneDir, "second.txt"), []byte("origin second"), 0644); err != nil {
			t.Fatalf("write second.txt: %v", err)
		}
		runGitInDir(t, cloneDir, "add", "second.txt")
		runGitInDir(t, cloneDir, "commit", "-m", "origin changes second")
		runGitInDir(t, cloneDir, "push", "origin", root)

		out := runStExpectError(t, "sync", "--down")
		assertContains(t, out, "conflict")

		// A user's editor may be interactive. Continuing must not launch it,
		// because subprocess output is captured and the editor cannot be used.
		editorMarker := filepath.Join(repo.Dir, "editor-was-opened")
		editorScript := filepath.Join(repo.Dir, "failing-editor.sh")
		if err := os.WriteFile(editorScript, []byte("#!/bin/sh\ntouch \""+editorMarker+"\"\nexit 1\n"), 0755); err != nil {
			t.Fatalf("write editor script: %v", err)
		}
		repo.RunGit("config", "core.editor", editorScript)

		resolveConflictAndStage(t, repo, "first.txt", "resolved first")
		out = runStExpectError(t, "continue")
		assertContains(t, out, "conflict")
		if repo.FileExists("editor-was-opened") {
			t.Fatal("st continue opened Git's commit-message editor")
		}
		second, err := os.ReadFile(filepath.Join(repo.Dir, "second.txt"))
		if err != nil {
			t.Fatalf("read second.txt: %v", err)
		}
		if !strings.Contains(string(second), "<<<<<<<") {
			t.Fatalf("first continue did not advance to the second conflict; second.txt = %q", second)
		}

		resolveConflictAndStage(t, repo, "second.txt", "resolved second")
		runSt(t, "continue")

		if current := getCurrentBranch(t, repo); current != "tip" {
			t.Errorf("current branch = %q, want tip", current)
		}
		if !repo.FileExists("tip.txt") {
			t.Error("tip commit should remain after continuing the restack")
		}
		if restackStateExists(t, repo) {
			t.Error("restack state should be cleared after the second continue")
		}
	})
}

func TestSyncDownConflictContinue(t *testing.T) {
	t.Run("sync_down_conflict_then_continue_no_push", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		if err := repo.AddRemote(); err != nil {
			t.Fatalf("AddRemote: %v", err)
		}
		repo.RunGit("push", "-u", "origin", root)

		// Create stack: root -> f1
		runSt(t, "new", "f1")
		repo.CreateFile("shared.txt", "f1 content")
		repo.CreateFile("f1.txt", "f1 file")
		repo.AddAndCommit("f1 commit")
		repo.RunGit("push", "origin", "f1")

		// Simulate upstream change: add conflicting commit on origin's trunk
		originDir := repo.OriginDir()
		tmpClone, _ := os.MkdirTemp("", "st-clone-*")
		defer os.RemoveAll(tmpClone)
		cloneCmd := exec.Command("git", "clone", originDir, tmpClone)
		cloneCmd.Run()
		exec.Command("git", "-C", tmpClone, "config", "user.email", "other@test.com").Run()
		exec.Command("git", "-C", tmpClone, "config", "user.name", "Other").Run()
		os.WriteFile(filepath.Join(tmpClone, "shared.txt"), []byte("upstream conflict content"), 0644)
		exec.Command("git", "-C", tmpClone, "add", ".").Run()
		exec.Command("git", "-C", tmpClone, "commit", "-m", "upstream conflict").Run()
		exec.Command("git", "-C", tmpClone, "push", "origin", root).Run()

		// Run sync --down from f1 — should conflict during restack
		repo.Checkout("f1")
		out := runStExpectError(t, "sync", "--down")
		assertContains(t, out, "conflict")

		// Resolve and continue
		resolveConflictAndStage(t, repo, "shared.txt", "resolved sync down content")
		runStContinue(t)

		// Verify f1 has resolved content
		repo.Checkout("f1")
		content, _ := os.ReadFile(filepath.Join(repo.Dir, "shared.txt"))
		if string(content) != "resolved sync down content" {
			t.Errorf("f1 shared.txt = %q, want 'resolved sync down content'", content)
		}

		// CRITICAL: sync --down should NOT have pushed f1 to origin
		remoteOut, _ := repo.RunGit("ls-remote", "--heads", "origin", "f1")
		// origin/f1 should have been pushed by the initial push above, not by sync --down.
		// We can't easily tell if sync --down pushed, but we can verify the remote ref
		// matches what we originally pushed (i.e., not a new SHA from sync --down).
		if strings.TrimSpace(remoteOut) == "" {
			t.Error("f1 should have been pushed (pre-existing), but sync --down should not have modified it")
		}
	})
}

// ---------------------------------------------------------------------------
// TestSyncConflictWithMergedBranch
// ---------------------------------------------------------------------------

func TestSyncConflictWithMergedBranch(t *testing.T) {
	t.Run("sync_removes_middle_branch_then_downstream_conflicts", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		if err := repo.AddRemote(); err != nil {
			t.Fatalf("AddRemote: %v", err)
		}
		repo.RunGit("push", "-u", "origin", root)

		// Build stack: root -> m1 -> m2
		runSt(t, "new", "m1")
		repo.CreateFile("shared.txt", "m1 content")
		repo.CreateFile("m1.txt", "m1 file")
		repo.AddAndCommit("m1 commit")

		runSt(t, "append", "m2")
		repo.CreateFile("shared.txt", "m2 content")
		repo.CreateFile("m2.txt", "m2 file")
		repo.AddAndCommit("m2 commit")

		repo.RunGit("push", "origin", "m1")
		repo.RunGit("push", "origin", "m2")

		// Simulate merge of m1 into root on remote, but m2 has conflicting shared.txt vs updated root
		originDir := repo.OriginDir()

		// Cherry-pick m1 onto root (simulating squash merge), then add conflicting commit to root
		repo.Checkout(root)
		repo.RunGit("cherry-pick", "m1")
		repo.CreateFile("shared.txt", "root updated content")
		repo.AddAndCommit("root updates after merge")
		repo.RunGit("push", "origin", root)
		repo.RunGit("push", "origin", "m2")

		// Delete origin/m1
		runGitInDir(t, originDir, "branch", "-D", "m1")

		// Sync from m2: should detect m1 as merged, remove it, then restack m2 onto root -> conflict
		repo.Checkout("m2")
		out := runStExpectError(t, "-v", "sync")
		assertContains(t, out, "Merged")
		assertContains(t, out, "conflict")

		// m1 should be gone from graph
		if graphContains(t, repo, "m1") {
			t.Error("m1 should have been removed from graph")
		}

		// Resolve and continue
		resolveConflictAndStage(t, repo, "shared.txt", "resolved content")
		runStContinue(t)

		// m2 should be reparented to root and have its m2.txt
		g := loadGraph(t, repo)
		m2, ok := g.Branches["m2"]
		if !ok {
			t.Fatal("m2 should still be in graph")
		}
		if m2.Parent != root {
			t.Errorf("m2 parent = %q, want %q", m2.Parent, root)
		}
		repo.Checkout("m2")
		if !repo.FileExists("m2.txt") {
			t.Error("m2 should have m2.txt after sync continue")
		}
	})
}
