// Package testutil provides test helpers for st
package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitRepo is a test helper for managing temporary git repositories
type GitRepo struct {
	Dir    string
	origin string
}

// NewGitRepo creates a new temporary git repository for testing
func NewGitRepo() (*GitRepo, error) {
	tmpDir, err := os.MkdirTemp("", "st-test-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	repo := &GitRepo{Dir: tmpDir}

	// Initialize git repo
	if err := repo.runGit("init"); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("failed to init git: %w", err)
	}

	// Configure git user (isolated from system config)
	if err := repo.runGit("config", "user.email", "test@example.com"); err != nil {
		os.RemoveAll(tmpDir)
		return nil, err
	}
	if err := repo.runGit("config", "user.name", "Test User"); err != nil {
		os.RemoveAll(tmpDir)
		return nil, err
	}

	return repo, nil
}

// Cleanup removes the temporary repository
func (r *GitRepo) Cleanup() {
	os.RemoveAll(r.Dir)
}

// runGit runs a git command in the repo directory
func (r *GitRepo) runGit(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	cmd.Env = r.gitEnv()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v failed: %w\nOutput: %s", args, err, output)
	}
	return nil
}

// gitEnv returns environment variables for isolated git operations
func (r *GitRepo) gitEnv() []string {
	// Get current env and modify it
	env := os.Environ()
	var newEnv []string

	for _, e := range env {
		if !strings.HasPrefix(e, "HOME=") {
			newEnv = append(newEnv, e)
		}
	}

	// Set HOME to temp dir for isolation
	newEnv = append(newEnv, "HOME="+r.Dir)
	newEnv = append(newEnv, "GIT_TERMINAL_PROMPT=0")

	return newEnv
}

// CreateFile creates a file with content and commits it
func (r *GitRepo) CreateFile(filename, content string) error {
	path := filepath.Join(r.Dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}

// AddAndCommit stages all changes and creates a commit
func (r *GitRepo) AddAndCommit(message string) error {
	if err := r.runGit("add", "."); err != nil {
		return err
	}
	if err := r.runGit("commit", "-m", message); err != nil {
		return err
	}
	return nil
}

// CreateBranch creates and checks out a new branch
func (r *GitRepo) CreateBranch(name string) error {
	return r.runGit("checkout", "-b", name)
}

// Checkout switches to a branch
func (r *GitRepo) Checkout(name string) error {
	return r.runGit("checkout", name)
}

// CurrentBranch returns the current branch name
func (r *GitRepo) CurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = r.Dir
	cmd.Env = r.gitEnv()
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// BranchExists checks if a branch exists
func (r *GitRepo) BranchExists(name string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", name)
	cmd.Dir = r.Dir
	cmd.Env = r.gitEnv()
	err := cmd.Run()
	return err == nil
}

// InitStack initializes a stack graph for testing
func (r *GitRepo) InitStack() error {
	// Create initial commit
	if err := r.CreateFile("README.md", "# Test Repo"); err != nil {
		return err
	}
	if err := r.AddAndCommit("Initial commit"); err != nil {
		return err
	}

	// Create .git/stack directory
	stackDir := filepath.Join(r.Dir, ".git", "stack")
	if err := os.MkdirAll(stackDir, 0755); err != nil {
		return err
	}

	return nil
}
