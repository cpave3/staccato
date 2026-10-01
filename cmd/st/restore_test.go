package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TestRestore
// ---------------------------------------------------------------------------

func TestRestore(t *testing.T) {
	t.Run("restores_from_automatic_backup", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)

		// Create f1 with a commit
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1 original")
		repo.AddAndCommit("f1 commit")

		// Manually create a backup branch matching the automatic backup format
		// (backup/<branch>/<timestamp>) so restore can find it
		cmd := exec.Command("git", "branch", "backup/f1/1234567890", "f1")
		cmd.Dir = repo.Dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to create backup branch: %v\n%s", err, out)
		}

		// Restore f1 from the backup
		runSt(t, "restore", "f1")
	})

	t.Run("restore_current_branch", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)

		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1 original")
		repo.AddAndCommit("f1 commit")

		// Create a backup branch
		cmd := exec.Command("git", "branch", "backup/f1/9999999999", "f1")
		cmd.Dir = repo.Dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to create backup: %v\n%s", err, out)
		}

		// Restore without specifying branch name (uses current branch)
		out := runSt(t, "restore")
		assertContains(t, out, "Restored")
	})

	t.Run("restore_all", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)

		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2")
		repo.AddAndCommit("f2 commit")

		// Create auto backup branches (restore --all uses ListBackups which matches auto format)
		cmd := exec.Command("git", "branch", "backup/auto/f1/9999999999", "f1")
		cmd.Dir = repo.Dir
		cmd.Run()
		cmd = exec.Command("git", "branch", "backup/auto/f2/9999999999", "f2")
		cmd.Dir = repo.Dir
		cmd.Run()

		// Restore all
		out := runSt(t, "restore", "--all")
		assertContains(t, out, "Restored")
	})

	t.Run("error_no_backups", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")
		out := runStExpectError(t, "restore", "f1")
		assertContains(t, out, "no backups found")
	})

	// Cycle 2: restore --all works during active rebase
	t.Run("restore_all_aborts_active_rebase", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: main -> f1 -> f2
		runSt(t, "new", "f1")
		repo.CreateFile("shared.txt", "f1 content")
		repo.CreateFile("f1.txt", "f1 file")
		repo.AddAndCommit("f1 commit")

		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2 file")
		repo.AddAndCommit("f2 commit")

		// Record original SHAs
		origF1SHA, _ := repo.RunGit("rev-parse", "f1")
		origF2SHA, _ := repo.RunGit("rev-parse", "f2")

		// Create conflicting change on root
		repo.Checkout(root)
		repo.CreateFile("shared.txt", "root content")
		repo.AddAndCommit("root conflict")

		// Restack -> conflict leaves rebase in progress
		repo.Checkout("f2")
		runStExpectError(t, "restack")

		// Verify rebase IS in progress
		_, err := repo.RunGit("rev-parse", "--git-path", "rebase-merge")
		if err != nil {
			// If we can't even check, skip further assertions
			t.Log("Warning: could not check rebase state")
		}

		// restore --all should succeed despite active rebase
		runSt(t, "restore", "--all")

		// Verify no rebase in progress after restore
		rebasePath, _ := repo.RunGit("rev-parse", "--git-path", "rebase-merge")
		if rebasePath != "" {
			// Check if the directory actually exists
			dirCheck := exec.Command("test", "-d", rebasePath)
			dirCheck.Dir = repo.Dir
			if dirCheck.Run() == nil {
				t.Error("rebase should not be in progress after restore --all")
			}
		}

		// Verify branches at original SHAs
		afterF1SHA, _ := repo.RunGit("rev-parse", "f1")
		afterF2SHA, _ := repo.RunGit("rev-parse", "f2")

		if afterF1SHA != origF1SHA {
			t.Errorf("f1 SHA = %s, want original %s", afterF1SHA, origF1SHA)
		}
		if afterF2SHA != origF2SHA {
			t.Errorf("f2 SHA = %s, want original %s", afterF2SHA, origF2SHA)
		}
	})

	// Cycle 3: restore --all updates graph state
	t.Run("restore_all_updates_graph_state", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: main -> f1 -> f2
		runSt(t, "new", "f1")
		repo.CreateFile("shared.txt", "f1 content")
		repo.CreateFile("f1.txt", "f1 file")
		repo.AddAndCommit("f1 commit")

		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2 file")
		repo.AddAndCommit("f2 commit")

		// Create conflicting change on root
		repo.Checkout(root)
		repo.CreateFile("shared.txt", "root content")
		repo.AddAndCommit("root conflict")

		// Restack -> conflict
		repo.Checkout("f2")
		runStExpectError(t, "restack")

		// Restore --all
		runSt(t, "restore", "--all")

		// Load graph and check that SHAs match actual branch tips
		g := loadGraph(t, repo)
		for branchName, branch := range g.Branches {
			actualSHA, err := repo.RunGit("rev-parse", branchName)
			if err != nil {
				t.Fatalf("failed to get SHA for %s: %v", branchName, err)
			}
			if branch.HeadSHA != actualSHA {
				t.Errorf("graph HeadSHA for %s = %s, actual = %s", branchName, branch.HeadSHA, actualSHA)
			}
		}
	})

	// Cycle 4: restore --all preserves branch upstream tracking
	t.Run("restore_all_preserves_upstream_tracking", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Set up a remote so we can set upstream tracking
		if err := repo.AddRemote(); err != nil {
			t.Fatalf("AddRemote: %v", err)
		}
		repo.RunGit("push", "-u", "origin", root)

		// Build stack: main -> f1 -> f2
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1 file")
		repo.CreateFile("shared.txt", "f1 content")
		repo.AddAndCommit("f1 commit")
		repo.RunGit("push", "-u", "origin", "f1")

		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2 file")
		repo.AddAndCommit("f2 commit")
		repo.RunGit("push", "-u", "origin", "f2")

		// Create conflicting change on root
		repo.Checkout(root)
		repo.CreateFile("shared.txt", "root content")
		repo.AddAndCommit("root conflict")

		// Restack -> conflict
		repo.Checkout("f2")
		runStExpectError(t, "restack")

		// Verify branches have upstream tracking before restore
		for _, branch := range []string{"f1", "f2", root} {
			remote, err := repo.RunGit("config", "--get", fmt.Sprintf("branch.%s.remote", branch))
			if err != nil {
				t.Fatalf("branch %s expected to have tracking remote before restore", branch)
			}
			if strings.TrimSpace(remote) != "origin" {
				t.Fatalf("branch %s tracking remote=%q, want origin", branch, remote)
			}
		}

		// Restore --all
		runSt(t, "restore", "--all")

		// Assert upstream tracking survived for all branches
		for _, branch := range []string{"f1", "f2", root} {
			remote, err := repo.RunGit("config", "--get", fmt.Sprintf("branch.%s.remote", branch))
			if err != nil {
				t.Errorf("branch %s lost upstream tracking remote after restore", branch)
				continue
			}
			if strings.TrimSpace(remote) != "origin" {
				t.Errorf("branch %s tracking remote=%q after restore, want origin", branch, remote)
			}
		}
	})
}

func TestRestoreOneRepairsGraph(t *testing.T) {
	repo, _ := setupRepoWithStack(t)
	runSt(t, "new", "restore-one")
	repo.CreateFile("original.txt", "original")
	repo.AddAndCommit("original")
	original, _ := repo.RunGit("rev-parse", "restore-one")
	repo.RunGit("branch", "backup/auto/restore-one/9999999999", "restore-one")
	repo.CreateFile("later.txt", "later")
	repo.AddAndCommit("later")
	runSt(t, "restore", "restore-one")
	g := loadGraph(t, repo)
	if g.Branches["restore-one"].HeadSHA != original {
		t.Fatalf("restore did not update graph head: %+v", g.Branches["restore-one"])
	}
}
