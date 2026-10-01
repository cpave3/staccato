package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestackRestoreIntegrity(t *testing.T) {
	t.Run("restore_all_after_conflict_returns_exact_pre_restack_state", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: root -> s1 -> s2 -> s3
		runSt(t, "new", "s1")
		repo.CreateFile("shared.txt", "s1 content")
		repo.CreateFile("s1.txt", "s1 file")
		repo.AddAndCommit("s1 commit")

		runSt(t, "append", "s2")
		repo.CreateFile("s2.txt", "s2 file")
		repo.AddAndCommit("s2 commit")

		runSt(t, "append", "s3")
		repo.CreateFile("s3.txt", "s3 file")
		repo.AddAndCommit("s3 commit")

		// Record pre-restack state
		origS1SHA := getBranchSHA(t, repo, "s1")
		origS2SHA := getBranchSHA(t, repo, "s2")
		origS3SHA := getBranchSHA(t, repo, "s3")

		// Read file content on s1 before restack
		repo.Checkout("s1")
		origContent, _ := os.ReadFile(filepath.Join(repo.Dir, "shared.txt"))

		// Create conflict on root
		repo.Checkout(root)
		repo.CreateFile("shared.txt", "root conflict content")
		repo.AddAndCommit("root conflict")

		// Restack from s3 — conflict at s1
		repo.Checkout("s3")
		runStExpectError(t, "restack")

		// Restore all
		runSt(t, "restore", "--all")

		// Verify all branches have original SHAs
		if getBranchSHA(t, repo, "s1") != origS1SHA {
			t.Errorf("s1 SHA should be restored to original")
		}
		if getBranchSHA(t, repo, "s2") != origS2SHA {
			t.Errorf("s2 SHA should be restored to original")
		}
		if getBranchSHA(t, repo, "s3") != origS3SHA {
			t.Errorf("s3 SHA should be restored to original")
		}

		// Verify graph SHAs match actual branches
		g := loadGraph(t, repo)
		for branchName, branch := range g.Branches {
			actualSHA := getBranchSHA(t, repo, branchName)
			if branch.HeadSHA != actualSHA {
				t.Errorf("graph HeadSHA for %s = %s, actual = %s", branchName, branch.HeadSHA, actualSHA)
			}
		}

		// Verify restack state file is cleared
		if restackStateExists(t, repo) {
			t.Error("restack state file should be cleared after restore")
		}

		// Verify no rebase in progress
		if rebaseInProgress(t, repo) {
			t.Error("rebase should not be in progress after restore")
		}

		// Verify file contents are restored
		repo.Checkout("s1")
		restoredContent, _ := os.ReadFile(filepath.Join(repo.Dir, "shared.txt"))
		if string(restoredContent) != string(origContent) {
			t.Errorf("s1 shared.txt content should be restored, got %q want %q", restoredContent, origContent)
		}
	})

	t.Run("restore_all_after_partial_continue_returns_pre_restack_state", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: root -> s1 -> s2 -> s3
		runSt(t, "new", "s1")
		repo.CreateFile("conflict1.txt", "s1 content")
		repo.CreateFile("s1.txt", "s1 file")
		repo.AddAndCommit("s1 commit")

		runSt(t, "append", "s2")
		repo.CreateFile("conflict2.txt", "s2 content")
		repo.CreateFile("s2.txt", "s2 file")
		repo.AddAndCommit("s2 commit")

		runSt(t, "append", "s3")
		repo.CreateFile("s3.txt", "s3 file")
		repo.AddAndCommit("s3 commit")

		// Record pre-restack SHAs
		origS1SHA := getBranchSHA(t, repo, "s1")
		origS2SHA := getBranchSHA(t, repo, "s2")
		origS3SHA := getBranchSHA(t, repo, "s3")

		// Create conflicts on root
		repo.Checkout(root)
		repo.CreateFile("conflict1.txt", "root conflict1")
		repo.AddAndCommit("root conflict1")
		repo.CreateFile("conflict2.txt", "root conflict2")
		repo.AddAndCommit("root conflict2")

		// Restack from s3 — conflict at s1
		repo.Checkout("s3")
		runStExpectError(t, "restack")

		// Resolve s1 and continue — should hit conflict at s2
		resolveConflictAndStage(t, repo, "conflict1.txt", "resolved1")
		runStContinueExpectError(t)

		// Now restore --all (after partial continue)
		runSt(t, "restore", "--all")

		// Verify ALL branches are back to pre-restack state
		if getBranchSHA(t, repo, "s1") != origS1SHA {
			t.Errorf("s1 SHA should be restored to original after partial continue + restore")
		}
		if getBranchSHA(t, repo, "s2") != origS2SHA {
			t.Errorf("s2 SHA should be restored to original after partial continue + restore")
		}
		if getBranchSHA(t, repo, "s3") != origS3SHA {
			t.Errorf("s3 SHA should be restored to original after partial continue + restore")
		}

		// Verify no rebase artifacts
		if rebaseInProgress(t, repo) {
			t.Error("no rebase should be in progress after restore")
		}
		if restackStateExists(t, repo) {
			t.Error("restack state should be cleared after restore")
		}
	})

	t.Run("restore_aborts_active_rebase_no_artifacts", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: root -> s1
		runSt(t, "new", "s1")
		repo.CreateFile("shared.txt", "s1 content")
		repo.AddAndCommit("s1 commit")

		// Create conflict on root
		repo.Checkout(root)
		repo.CreateFile("shared.txt", "root content")
		repo.AddAndCommit("root conflict")

		// Restack — conflict
		repo.Checkout("s1")
		runStExpectError(t, "restack")

		// Verify rebase IS in progress before restore
		if !rebaseInProgress(t, repo) {
			t.Fatal("rebase should be in progress before restore")
		}

		// Restore --all without resolving conflict
		runSt(t, "restore", "--all")

		// Verify no rebase-merge directory
		rebaseMergePath := filepath.Join(repo.Dir, ".git", "rebase-merge")
		if _, err := os.Stat(rebaseMergePath); err == nil {
			t.Error(".git/rebase-merge should not exist after restore")
		}
		rebaseApplyPath := filepath.Join(repo.Dir, ".git", "rebase-apply")
		if _, err := os.Stat(rebaseApplyPath); err == nil {
			t.Error(".git/rebase-apply should not exist after restore")
		}

		// Verify user is on a valid branch (not detached HEAD)
		cur := getCurrentBranch(t, repo)
		if cur == "HEAD" || cur == "" {
			t.Errorf("should be on a valid branch after restore, got %q", cur)
		}
	})
}

// ---------------------------------------------------------------------------
// TestRestackBackupIntegrity — Tasks 3.1, 3.2, 3.3
// ---------------------------------------------------------------------------

func TestRestackBackupIntegrity(t *testing.T) {
	t.Run("backups_survive_multiple_continue_cycles", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: root -> s1 -> s2
		runSt(t, "new", "s1")
		repo.CreateFile("conflict1.txt", "s1 content")
		repo.CreateFile("s1.txt", "s1 file")
		repo.AddAndCommit("s1 commit")

		runSt(t, "append", "s2")
		repo.CreateFile("conflict2.txt", "s2 content")
		repo.CreateFile("s2.txt", "s2 file")
		repo.AddAndCommit("s2 commit")

		// Record pre-restack SHAs
		origS1SHA := getBranchSHA(t, repo, "s1")
		origS2SHA := getBranchSHA(t, repo, "s2")

		// Create conflicts on root
		repo.Checkout(root)
		repo.CreateFile("conflict1.txt", "root c1")
		repo.AddAndCommit("root c1")
		repo.CreateFile("conflict2.txt", "root c2")
		repo.AddAndCommit("root c2")

		// Restack from s2 — conflict at s1
		repo.Checkout("s2")
		runStExpectError(t, "restack")

		// Check backups exist for both branches after first conflict
		s1Backups, _ := repo.RunGit("for-each-ref", "--format=%(refname:short)", "refs/heads/backup/auto/s1/")
		s2Backups, _ := repo.RunGit("for-each-ref", "--format=%(refname:short)", "refs/heads/backup/auto/s2/")
		if s1Backups == "" {
			t.Error("s1 auto backup should exist after restack conflict")
		}
		if s2Backups == "" {
			t.Error("s2 auto backup should exist after restack conflict")
		}

		// Resolve s1, continue — hits conflict at s2
		resolveConflictAndStage(t, repo, "conflict1.txt", "resolved c1")
		runStContinueExpectError(t)

		// Backups should STILL exist after first continue
		s1BackupsAfter, _ := repo.RunGit("for-each-ref", "--format=%(refname:short)", "refs/heads/backup/auto/s1/")
		s2BackupsAfter, _ := repo.RunGit("for-each-ref", "--format=%(refname:short)", "refs/heads/backup/auto/s2/")
		if s1BackupsAfter == "" {
			t.Error("s1 auto backup should still exist after first continue")
		}
		if s2BackupsAfter == "" {
			t.Error("s2 auto backup should still exist after first continue")
		}

		// Verify backups point to pre-restack SHAs
		s1BackupSHA := getBranchSHA(t, repo, strings.TrimSpace(s1BackupsAfter))
		s2BackupSHA := getBranchSHA(t, repo, strings.TrimSpace(s2BackupsAfter))
		if s1BackupSHA != origS1SHA {
			t.Errorf("s1 backup SHA = %s, want original %s", s1BackupSHA, origS1SHA)
		}
		if s2BackupSHA != origS2SHA {
			t.Errorf("s2 backup SHA = %s, want original %s", s2BackupSHA, origS2SHA)
		}

		// Resolve s2, continue to completion
		resolveConflictAndStage(t, repo, "conflict2.txt", "resolved c2")
		runStContinue(t)

		// Backups should be cleaned up after final success
		s1BackupsFinal, _ := repo.RunGit("for-each-ref", "--format=%(refname:short)", "refs/heads/backup/auto/s1/")
		s2BackupsFinal, _ := repo.RunGit("for-each-ref", "--format=%(refname:short)", "refs/heads/backup/auto/s2/")
		if strings.TrimSpace(s1BackupsFinal) != "" {
			t.Error("s1 auto backups should be cleaned up after successful completion")
		}
		if strings.TrimSpace(s2BackupsFinal) != "" {
			t.Error("s2 auto backups should be cleaned up after successful completion")
		}
	})

	t.Run("restore_after_multiple_continues_returns_pre_restack_state", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: root -> s1 -> s2
		runSt(t, "new", "s1")
		repo.CreateFile("conflict1.txt", "s1 original")
		repo.AddAndCommit("s1 commit")

		runSt(t, "append", "s2")
		repo.CreateFile("conflict2.txt", "s2 original")
		repo.AddAndCommit("s2 commit")

		// Record pre-restack SHAs
		origS1SHA := getBranchSHA(t, repo, "s1")
		origS2SHA := getBranchSHA(t, repo, "s2")

		// Create conflicts on root
		repo.Checkout(root)
		repo.CreateFile("conflict1.txt", "root c1")
		repo.AddAndCommit("root c1")
		repo.CreateFile("conflict2.txt", "root c2")
		repo.AddAndCommit("root c2")

		// Restack from s2 — conflict at s1
		repo.Checkout("s2")
		runStExpectError(t, "restack")

		// Resolve s1, continue — conflict at s2
		resolveConflictAndStage(t, repo, "conflict1.txt", "resolved c1")
		runStContinueExpectError(t)

		// Now restore (s1 was rebased, s2 still conflicting)
		runSt(t, "restore", "--all")

		// Both should be back to pre-restack state
		if getBranchSHA(t, repo, "s1") != origS1SHA {
			t.Errorf("s1 should be restored to pre-restack SHA")
		}
		if getBranchSHA(t, repo, "s2") != origS2SHA {
			t.Errorf("s2 should be restored to pre-restack SHA")
		}

		// Verify file contents match originals
		repo.Checkout("s1")
		content, _ := os.ReadFile(filepath.Join(repo.Dir, "conflict1.txt"))
		if string(content) != "s1 original" {
			t.Errorf("s1 conflict1.txt = %q, want 's1 original'", content)
		}

		repo.Checkout("s2")
		content, _ = os.ReadFile(filepath.Join(repo.Dir, "conflict2.txt"))
		if string(content) != "s2 original" {
			t.Errorf("s2 conflict2.txt = %q, want 's2 original'", content)
		}
	})
}

// ---------------------------------------------------------------------------
// TestRestackRerere — Task 4.1
// ---------------------------------------------------------------------------

func TestAbortRestoresBackups(t *testing.T) {
	t.Run("abort_during_restack_conflict_restores_original_shas", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: root -> f1 -> f2
		runSt(t, "new", "f1")
		repo.CreateFile("shared.txt", "f1 content")
		repo.CreateFile("f1.txt", "f1 file")
		repo.AddAndCommit("f1 commit")

		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2 file")
		repo.AddAndCommit("f2 commit")

		// Record original SHAs
		origF1SHA := getBranchSHA(t, repo, "f1")
		origF2SHA := getBranchSHA(t, repo, "f2")

		// Root conflicts with f1's shared.txt
		repo.Checkout(root)
		repo.CreateFile("shared.txt", "root content")
		repo.AddAndCommit("root conflict")

		// Restack from f2 -> conflict at f1
		repo.Checkout("f2")
		runStExpectError(t, "restack")

		// Verify rebase is in progress
		if !rebaseInProgress(t, repo) {
			t.Fatal("rebase should be in progress")
		}

		// Abort: should restore f1 and f2 to original SHAs
		runSt(t, "abort")

		if rebaseInProgress(t, repo) {
			t.Error("rebase should not be in progress after abort")
		}

		if getBranchSHA(t, repo, "f1") != origF1SHA {
			t.Errorf("f1 SHA not restored: got %s, want %s", getBranchSHA(t, repo, "f1"), origF1SHA)
		}
		if getBranchSHA(t, repo, "f2") != origF2SHA {
			t.Errorf("f2 SHA not restored: got %s, want %s", getBranchSHA(t, repo, "f2"), origF2SHA)
		}
	})
}
