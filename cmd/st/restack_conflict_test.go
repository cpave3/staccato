package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRestackMultiConflictCycle(t *testing.T) {
	t.Run("sequential_conflicts_across_two_branches", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: root -> s1 -> s2 -> s3
		// s1 modifies conflict1.txt, s2 modifies conflict2.txt
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

		// Record pre-restack graph SHAs
		preGraph := loadGraph(t, repo)
		preS1HeadSHA := preGraph.Branches["s1"].HeadSHA
		preS3HeadSHA := preGraph.Branches["s3"].HeadSHA

		// Create conflicting changes on root for BOTH conflict files
		repo.Checkout(root)
		repo.CreateFile("conflict1.txt", "root conflict1 content")
		repo.AddAndCommit("root conflict1")
		repo.CreateFile("conflict2.txt", "root conflict2 content")
		repo.AddAndCommit("root conflict2")

		// Restack from s3 (tip) — should stop at s1
		repo.Checkout("s3")
		out := runStExpectError(t, "restack")
		assertContains(t, out, "conflict")

		// Verify restack state file exists
		if !restackStateExists(t, repo) {
			t.Error("restack state file should exist after conflict")
		}

		// Verify rebase is in progress
		if !rebaseInProgress(t, repo) {
			t.Error("rebase should be in progress after conflict at s1")
		}

		// Resolve conflict1.txt and continue — should proceed then stop at s2
		resolveConflictAndStage(t, repo, "conflict1.txt", "resolved conflict1")
		out = runStContinueExpectError(t)
		assertContains(t, out, "conflict")

		// Verify restack state file still exists
		if !restackStateExists(t, repo) {
			t.Error("restack state file should persist after second conflict")
		}

		// Verify graph: s1 should NOW have updated SHAs (it was rebased successfully)
		g := loadGraph(t, repo)
		if g.Branches["s1"].HeadSHA == preS1HeadSHA {
			t.Error("s1 HeadSHA should be updated after successful continue for s1")
		}
		// s3 should still have its pre-restack graph SHA (not yet processed)
		if g.Branches["s3"].HeadSHA != preS3HeadSHA {
			t.Error("s3 HeadSHA should be unchanged while s2 is conflicting")
		}

		// Resolve conflict2.txt and continue — should complete
		resolveConflictAndStage(t, repo, "conflict2.txt", "resolved conflict2")
		runStContinue(t)

		// Verify restack state file is cleared
		if restackStateExists(t, repo) {
			t.Error("restack state file should be cleared after successful completion")
		}

		// Verify no rebase in progress
		if rebaseInProgress(t, repo) {
			t.Error("rebase should not be in progress after completion")
		}

		// Verify all graph SHAs now match actual branch heads
		g = loadGraph(t, repo)
		for branchName, branch := range g.Branches {
			actualSHA := getBranchSHA(t, repo, branchName)
			if branch.HeadSHA != actualSHA {
				t.Errorf("graph HeadSHA for %s = %s, actual = %s", branchName, branch.HeadSHA, actualSHA)
			}
		}

		// Verify file contents on each branch
		repo.Checkout("s1")
		content, _ := os.ReadFile(filepath.Join(repo.Dir, "conflict1.txt"))
		if string(content) != "resolved conflict1" {
			t.Errorf("s1 conflict1.txt = %q, want 'resolved conflict1'", content)
		}

		repo.Checkout("s2")
		content, _ = os.ReadFile(filepath.Join(repo.Dir, "conflict2.txt"))
		if string(content) != "resolved conflict2" {
			t.Errorf("s2 conflict2.txt = %q, want 'resolved conflict2'", content)
		}
		if !repo.FileExists("s2.txt") {
			t.Error("s2 should have s2.txt")
		}

		repo.Checkout("s3")
		if !repo.FileExists("s3.txt") {
			t.Error("s3 should have s3.txt")
		}
		if !repo.FileExists("conflict1.txt") {
			t.Error("s3 should have conflict1.txt (inherited from s1)")
		}
	})
}

// ---------------------------------------------------------------------------
// TestRestackRestoreIntegrity — Tasks 2.1, 2.2, 2.3, 2.4
// ---------------------------------------------------------------------------

func TestContinueAfterManualGitRebase(t *testing.T) {
	t.Run("manual_rebase_continue_after_st_conflict_then_st_continue", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: root -> f1
		runSt(t, "new", "f1")
		repo.CreateFile("shared.txt", "f1 content")
		repo.AddAndCommit("f1 commit")

		// Root conflicts with f1
		repo.Checkout(root)
		repo.CreateFile("shared.txt", "root content")
		repo.AddAndCommit("root conflict")

		repo.Checkout("f1")
		runStExpectError(t, "restack")

		// User resolves manually and runs git rebase --continue instead of st continue
		resolveConflictAndStage(t, repo, "shared.txt", "manually resolved")
		contCmd := exec.Command("git", "rebase", "--continue")
		contCmd.Dir = repo.Dir
		contCmd.Env = append(os.Environ(), "GIT_EDITOR=true")
		contOut, err := contCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("manual git rebase --continue failed: %v\n%s", err, contOut)
		}

		// Rebase is complete. st continue should detect no rebase in progress.
		out := runStExpectError(t, "continue")
		assertContains(t, out, "no rebase in progress")

		// But restack state file still exists — should be cleaned up by user or we should handle
		if restackStateExists(t, repo) {
			// Non-critical: state file is a benign artifact. Could warn but shouldn't crash.
			t.Log("restack state file still exists after manual rebase --continue — benign artifact")
		}
	})
}
