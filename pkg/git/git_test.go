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
	cmd := exec.Command("git", "init")
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
	cmd := exec.Command("git", "init")
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
	cmd := exec.Command("git", "init")
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
	cmd := exec.Command("git", "init")
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
	cmd := exec.Command("git", "init")
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

func TestGitRunner_GetMergeBase(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init")
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
