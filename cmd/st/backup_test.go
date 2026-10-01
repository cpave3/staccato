package main

import (
	"os/exec"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TestBackup
// ---------------------------------------------------------------------------

func TestBackup(t *testing.T) {
	t.Run("creates_manual_backup_branches", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")

		runSt(t, "backup")

		// Check for backup/manual/* branches (new naming scheme)
		cmd := exec.Command("git", "branch", "--list", "backup/manual/*")
		cmd.Dir = repo.Dir
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git branch --list: %v", err)
		}
		if !strings.Contains(string(out), "f1") {
			t.Errorf("backup branches should contain f1, got: %s", out)
		}
	})

	t.Run("error_no_branches", func(t *testing.T) {
		setupRepoWithStack(t)
		out := runStExpectError(t, "backup")
		assertContains(t, out, "no branches")
	})
}

// ---------------------------------------------------------------------------
// TestBackupList
// ---------------------------------------------------------------------------

func TestBackupList(t *testing.T) {
	t.Run("no_backups", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")

		out := runSt(t, "backup", "list")
		assertContains(t, out, "No backups found")
	})

	t.Run("lists_backups", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")

		// Create a manual backup
		runSt(t, "backup")

		out := runSt(t, "backup", "list")
		assertContains(t, out, "Found")
		assertContains(t, out, "f1")
	})
}
