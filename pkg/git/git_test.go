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
