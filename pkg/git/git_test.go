package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitRunner_Run(t *testing.T) {
	// Create a temp git repo
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	git := NewRunner(tmpDir)

	output, err := git.Run("rev-parse", "--git-dir")
	if err != nil {
		t.Fatalf("failed to run git command: %v", err)
	}

	if !strings.Contains(output, ".git") {
		t.Errorf("expected output to contain .git, got: %s", output)
	}
}

func TestGitRunner_GetCurrentBranch(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	git := NewRunner(tmpDir)

	branch, err := git.GetCurrentBranch()
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}

	// Should be on main or master depending on git version
	if branch != "main" && branch != "master" {
		t.Errorf("expected main or master, got: %s", branch)
	}
}

func TestGitRunner_CreateBranch(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	git := NewRunner(tmpDir)

	err := git.CreateBranch("feature-branch")
	if err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	// Verify branch exists
	output, _ := exec.Command("git", "-C", tmpDir, "branch", "--list", "feature-branch").Output()
	if !strings.Contains(string(output), "feature-branch") {
		t.Error("expected branch to be created")
	}
}

func TestGitRunner_GetCommitSHA(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	git := NewRunner(tmpDir)

	sha, err := git.GetCommitSHA("HEAD")
	if err != nil {
		t.Fatalf("failed to get commit SHA: %v", err)
	}

	if len(sha) != 40 {
		t.Errorf("expected 40 character SHA, got: %s (%d chars)", sha, len(sha))
	}
}

func TestGitRunner_EnableRerere(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	git := NewRunner(tmpDir)

	err := git.EnableRerere()
	if err != nil {
		t.Fatalf("failed to enable rerere: %v", err)
	}

	// Verify rerere is enabled
	output, _ := exec.Command("git", "-C", tmpDir, "config", "rerere.enabled").Output()
	if strings.TrimSpace(string(output)) != "true" {
		t.Errorf("expected rerere.enabled to be true, got: %s", string(output))
	}
}

func initRepoWithCommit(t *testing.T) (string, *Runner) {
	t.Helper()
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()
	return tmpDir, NewRunner(tmpDir)
}

func TestGitRunner_RefExists(t *testing.T) {
	_, git := initRepoWithCommit(t)

	if git.RefExists("refs/staccato/graph") {
		t.Error("ref should not exist initially")
	}
	if !git.RefExists("refs/heads/main") {
		t.Error("refs/heads/main should exist")
	}
}

func TestGitRunner_WriteBlobRef_ReadBlobRef(t *testing.T) {
	_, git := initRepoWithCommit(t)

	ref := "refs/staccato/graph"
	data := []byte(`{"version":1,"root":"main","branches":{}}`)

	if err := git.WriteBlobRef(ref, data); err != nil {
		t.Fatalf("WriteBlobRef failed: %v", err)
	}

	if !git.RefExists(ref) {
		t.Fatal("ref should exist after WriteBlobRef")
	}

	got, err := git.ReadBlobRef(ref)
	if err != nil {
		t.Fatalf("ReadBlobRef failed: %v", err)
	}

	if string(got) != string(data) {
		t.Errorf("ReadBlobRef = %q, want %q", got, data)
	}
}

func TestGitRunner_DeleteRef(t *testing.T) {
	_, git := initRepoWithCommit(t)

	ref := "refs/staccato/graph"
	git.WriteBlobRef(ref, []byte("test data"))

	if !git.RefExists(ref) {
		t.Fatal("ref should exist before delete")
	}

	if err := git.DeleteRef(ref); err != nil {
		t.Fatalf("DeleteRef failed: %v", err)
	}

	if git.RefExists(ref) {
		t.Error("ref should not exist after delete")
	}
}

func TestGitRunner_WriteBlobRef_Overwrite(t *testing.T) {
	_, git := initRepoWithCommit(t)

	ref := "refs/staccato/graph"
	git.WriteBlobRef(ref, []byte("first"))

	if err := git.WriteBlobRef(ref, []byte("second")); err != nil {
		t.Fatalf("second WriteBlobRef failed: %v", err)
	}

	got, _ := git.ReadBlobRef(ref)
	if string(got) != "second" {
		t.Errorf("ReadBlobRef = %q, want %q", got, "second")
	}
}

func TestGitRunner_HasUncommittedChanges(t *testing.T) {
	tmpDir, git := initRepoWithCommit(t)

	// Clean state — no uncommitted changes
	has, err := git.HasUncommittedChanges()
	if err != nil {
		t.Fatalf("HasUncommittedChanges: %v", err)
	}
	if has {
		t.Error("expected no uncommitted changes on clean repo")
	}

	// Create an unstaged file
	os.WriteFile(filepath.Join(tmpDir, "dirty.txt"), []byte("dirty"), 0644)
	has, err = git.HasUncommittedChanges()
	if err != nil {
		t.Fatalf("HasUncommittedChanges: %v", err)
	}
	if !has {
		t.Error("expected uncommitted changes after creating untracked file")
	}
}

func TestGitRunner_HasUncommittedChanges_StagedFile(t *testing.T) {
	tmpDir, git := initRepoWithCommit(t)

	// Stage a new file (but don't commit)
	os.WriteFile(filepath.Join(tmpDir, "staged.txt"), []byte("staged"), 0644)
	exec.Command("git", "-C", tmpDir, "add", "staged.txt").Run()

	has, err := git.HasUncommittedChanges()
	if err != nil {
		t.Fatalf("HasUncommittedChanges: %v", err)
	}
	if !has {
		t.Error("expected uncommitted changes with staged file")
	}
}

func TestGitRunner_StashPush(t *testing.T) {
	tmpDir, git := initRepoWithCommit(t)

	// Create a dirty file
	os.WriteFile(filepath.Join(tmpDir, "dirty.txt"), []byte("dirty"), 0644)
	exec.Command("git", "-C", tmpDir, "add", "dirty.txt").Run()

	err := git.StashPush("test stash message")
	if err != nil {
		t.Fatalf("StashPush: %v", err)
	}

	// Working tree should be clean now
	has, _ := git.HasUncommittedChanges()
	if has {
		t.Error("expected clean working tree after stash push")
	}

	// Stash list should have our entry
	out, _ := git.Run("stash", "list")
	if !strings.Contains(out, "test stash message") {
		t.Errorf("stash list should contain our message, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// TestReset
// ---------------------------------------------------------------------------

func TestGitRunner_Reset(t *testing.T) {
	t.Run("valid_soft", func(t *testing.T) {
		_, git := initRepoWithCommit(t)
		_, err := git.Reset("", "soft")
		if err != nil {
			t.Fatalf("Reset soft: %v", err)
		}
	})

	t.Run("valid_mixed", func(t *testing.T) {
		_, git := initRepoWithCommit(t)
		_, err := git.Reset("", "mixed")
		if err != nil {
			t.Fatalf("Reset mixed: %v", err)
		}
	})

	t.Run("valid_hard", func(t *testing.T) {
		_, git := initRepoWithCommit(t)
		_, err := git.Reset("", "hard")
		if err != nil {
			t.Fatalf("Reset hard: %v", err)
		}
	})

	t.Run("invalid_mode", func(t *testing.T) {
		_, git := initRepoWithCommit(t)
		_, err := git.Reset("", "bogus")
		if err == nil {
			t.Fatal("expected error for invalid mode")
		}
		if !strings.Contains(err.Error(), "invalid reset mode") {
			t.Errorf("expected 'invalid reset mode' error, got: %v", err)
		}
	})

	t.Run("with_ref", func(t *testing.T) {
		_, git := initRepoWithCommit(t)
		_, err := git.Reset("HEAD", "soft")
		if err != nil {
			t.Fatalf("Reset with ref: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// TestDiff
// ---------------------------------------------------------------------------

func TestGitRunner_Diff(t *testing.T) {
	t.Run("unstaged_no_paths", func(t *testing.T) {
		tmpDir, git := initRepoWithCommit(t)
		os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("changed"), 0644)
		out, err := git.Diff(false, nil)
		if err != nil {
			t.Fatalf("Diff: %v", err)
		}
		if !strings.Contains(out, "changed") {
			t.Error("expected diff to show change")
		}
	})

	t.Run("staged", func(t *testing.T) {
		tmpDir, git := initRepoWithCommit(t)
		os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("staged change"), 0644)
		exec.Command("git", "-C", tmpDir, "add", "test.txt").Run()
		out, err := git.Diff(true, nil)
		if err != nil {
			t.Fatalf("Diff staged: %v", err)
		}
		if !strings.Contains(out, "staged change") {
			t.Error("expected staged diff to show change")
		}
	})

	t.Run("with_paths", func(t *testing.T) {
		tmpDir, git := initRepoWithCommit(t)
		os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("changed"), 0644)
		os.WriteFile(filepath.Join(tmpDir, "other.txt"), []byte("other"), 0644)
		exec.Command("git", "-C", tmpDir, "add", "other.txt").Run()
		exec.Command("git", "-C", tmpDir, "commit", "-m", "add other").Run()
		os.WriteFile(filepath.Join(tmpDir, "other.txt"), []byte("other changed"), 0644)

		out, err := git.Diff(false, []string{"other.txt"})
		if err != nil {
			t.Fatalf("Diff with paths: %v", err)
		}
		if !strings.Contains(out, "other changed") {
			t.Error("expected diff filtered to other.txt")
		}
	})
}

// ---------------------------------------------------------------------------
// TestLog
// ---------------------------------------------------------------------------

func TestGitRunner_Log(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		_, git := initRepoWithCommit(t)
		out, err := git.Log("", 0, false)
		if err != nil {
			t.Fatalf("Log: %v", err)
		}
		if !strings.Contains(out, "initial") {
			t.Error("expected log to contain 'initial'")
		}
	})

	t.Run("with_limit", func(t *testing.T) {
		tmpDir, git := initRepoWithCommit(t)
		os.WriteFile(filepath.Join(tmpDir, "second.txt"), []byte("2"), 0644)
		exec.Command("git", "-C", tmpDir, "add", ".").Run()
		exec.Command("git", "-C", tmpDir, "commit", "-m", "second").Run()

		out, err := git.Log("", 1, false)
		if err != nil {
			t.Fatalf("Log with limit: %v", err)
		}
		if !strings.Contains(out, "second") {
			t.Error("expected most recent commit")
		}
		// With limit=1, should only have one line
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) != 1 {
			t.Errorf("expected 1 line, got %d", len(lines))
		}
	})

	t.Run("with_stat", func(t *testing.T) {
		_, git := initRepoWithCommit(t)
		out, err := git.Log("", 0, true)
		if err != nil {
			t.Fatalf("Log with stat: %v", err)
		}
		if !strings.Contains(out, "test.txt") {
			t.Error("expected stat output to mention test.txt")
		}
	})

	t.Run("with_range", func(t *testing.T) {
		tmpDir, git := initRepoWithCommit(t)
		exec.Command("git", "-C", tmpDir, "checkout", "-b", "feature").Run()
		os.WriteFile(filepath.Join(tmpDir, "feat.txt"), []byte("f"), 0644)
		exec.Command("git", "-C", tmpDir, "add", ".").Run()
		exec.Command("git", "-C", tmpDir, "commit", "-m", "feature commit").Run()

		out, err := git.Log("main..feature", 0, false)
		if err != nil {
			t.Fatalf("Log with range: %v", err)
		}
		if !strings.Contains(out, "feature commit") {
			t.Error("expected range log to contain 'feature commit'")
		}
	})
}

// ---------------------------------------------------------------------------
// TestMCPHelpers
// ---------------------------------------------------------------------------

func TestGitRunner_Add(t *testing.T) {
	tmpDir, git := initRepoWithCommit(t)
	os.WriteFile(filepath.Join(tmpDir, "new.txt"), []byte("new"), 0644)

	_, err := git.Add([]string{"new.txt"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Verify it's staged
	out, _ := git.Status()
	if !strings.Contains(out, "new.txt") {
		t.Error("expected new.txt to be staged")
	}
}

func TestGitRunner_Commit(t *testing.T) {
	tmpDir, git := initRepoWithCommit(t)
	os.WriteFile(filepath.Join(tmpDir, "new.txt"), []byte("new"), 0644)
	exec.Command("git", "-C", tmpDir, "add", "new.txt").Run()

	_, err := git.Commit("test commit message")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	out, _ := git.Log("", 1, false)
	if !strings.Contains(out, "test commit message") {
		t.Error("expected commit message in log")
	}
}

func TestGitRunner_Status(t *testing.T) {
	tmpDir, git := initRepoWithCommit(t)

	// Clean repo
	out, err := git.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected clean status, got: %q", out)
	}

	// Dirty repo
	os.WriteFile(filepath.Join(tmpDir, "dirty.txt"), []byte("dirty"), 0644)
	out, err = git.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !strings.Contains(out, "dirty.txt") {
		t.Error("expected dirty.txt in status")
	}
}

func TestGitRunner_StashPop(t *testing.T) {
	tmpDir, git := initRepoWithCommit(t)

	// Stage and stash a change
	os.WriteFile(filepath.Join(tmpDir, "stashed.txt"), []byte("stash me"), 0644)
	exec.Command("git", "-C", tmpDir, "add", "stashed.txt").Run()
	git.StashPush("test stash")

	// Working tree should be clean
	has, _ := git.HasUncommittedChanges()
	if has {
		t.Fatal("expected clean after stash push")
	}

	// Pop the stash
	err := git.StashPop()
	if err != nil {
		t.Fatalf("StashPop: %v", err)
	}

	// File should be back
	has, _ = git.HasUncommittedChanges()
	if !has {
		t.Error("expected uncommitted changes after stash pop")
	}
}

func TestGitRunner_GetAllBranches(t *testing.T) {
	tmpDir, git := initRepoWithCommit(t)

	exec.Command("git", "-C", tmpDir, "branch", "feature-1").Run()
	exec.Command("git", "-C", tmpDir, "branch", "feature-2").Run()

	branches, err := git.GetAllBranches()
	if err != nil {
		t.Fatalf("GetAllBranches: %v", err)
	}

	found := map[string]bool{}
	for _, b := range branches {
		found[b] = true
	}
	for _, expected := range []string{"main", "feature-1", "feature-2"} {
		if !found[expected] {
			t.Errorf("expected branch %s in list, got: %v", expected, branches)
		}
	}
}

func TestGitRunner_CherryPick(t *testing.T) {
	tmpDir, git := initRepoWithCommit(t)

	// Create a branch with a commit
	exec.Command("git", "-C", tmpDir, "checkout", "-b", "feature").Run()
	os.WriteFile(filepath.Join(tmpDir, "cherry.txt"), []byte("cherry"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "cherry commit").Run()

	// Get the SHA
	shaOut, _ := exec.Command("git", "-C", tmpDir, "rev-parse", "HEAD").Output()
	sha := strings.TrimSpace(string(shaOut))

	// Go back to main and cherry-pick
	exec.Command("git", "-C", tmpDir, "checkout", "main").Run()
	_, err := git.CherryPick(sha)
	if err != nil {
		t.Fatalf("CherryPick: %v", err)
	}

	// Verify the file is present
	if _, statErr := os.Stat(filepath.Join(tmpDir, "cherry.txt")); os.IsNotExist(statErr) {
		t.Error("expected cherry.txt after cherry-pick")
	}
}

func TestGitRunner_DiffStat(t *testing.T) {
	tmpDir, git := initRepoWithCommit(t)

	exec.Command("git", "-C", tmpDir, "checkout", "-b", "feature").Run()
	os.WriteFile(filepath.Join(tmpDir, "new.txt"), []byte("new"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "add new").Run()

	out, err := git.DiffStat("main")
	if err != nil {
		t.Fatalf("DiffStat: %v", err)
	}
	if !strings.Contains(out, "new.txt") {
		t.Error("expected DiffStat to mention new.txt")
	}
}

func TestGitRunner_GetMergeBase(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	// Create initial commit on main
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	// Get base SHA
	baseOutput, _ := exec.Command("git", "-C", tmpDir, "rev-parse", "HEAD").Output()
	baseSHA := strings.TrimSpace(string(baseOutput))

	// Create feature branch with a commit
	exec.Command("git", "-C", tmpDir, "checkout", "-b", "feature").Run()
	os.WriteFile(testFile, []byte("feature change"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "feature commit").Run()

	git := NewRunner(tmpDir)

	mergeBase, err := git.GetMergeBase("feature", "main")
	if err != nil {
		t.Fatalf("failed to get merge base: %v", err)
	}

	if mergeBase != baseSHA {
		t.Errorf("expected merge base %s, got: %s", baseSHA, mergeBase)
	}
}
