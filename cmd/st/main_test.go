package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/st/internal/testutil"
)

// stBinaryPath is the path to the compiled st binary
var stBinaryPath = "/var/home/cameron/Projects/restack/st"

// TestNewCommand tests the 'st new' command
func TestNewCommand(t *testing.T) {
	// Build the binary first
	buildCmd := exec.Command("go", "build", "-o", stBinaryPath, "./cmd/st/")
	buildCmd.Dir = "/var/home/cameron/Projects/restack"
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build st: %v\nOutput: %s", err, output)
	}

	// Create isolated git repo
	repo, err := testutil.NewGitRepo()
	if err != nil {
		t.Fatalf("Failed to create test repo: %v", err)
	}
	defer repo.Cleanup()

	// Change to repo directory
	oldWd, _ := os.Getwd()
	os.Chdir(repo.Dir)
	defer os.Chdir(oldWd)

	// Initialize stack with initial commit
	if err := repo.InitStack(); err != nil {
		t.Fatalf("Failed to init stack: %v", err)
	}

	// Run 'st new feature-1'
	cmd := exec.Command(stBinaryPath, "new", "feature-1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("st new failed: %v\nOutput: %s", err, output)
	}

	// Verify branch was created
	if !repo.BranchExists("feature-1") {
		t.Error("Branch 'feature-1' was not created")
	}

	// Verify graph file exists and contains the branch
	graphPath := filepath.Join(repo.Dir, ".git", "stack", "graph.json")
	if _, err := os.Stat(graphPath); os.IsNotExist(err) {
		t.Error("Graph file was not created")
	}

	// Read graph and verify branch is in it
	content, err := os.ReadFile(graphPath)
	if err != nil {
		t.Fatalf("Failed to read graph: %v", err)
	}
	if !strings.Contains(string(content), "feature-1") {
		t.Error("Graph doesn't contain 'feature-1'")
	}

	// Verify we're still on the new branch
	current, _ := repo.CurrentBranch()
	if !strings.Contains(current, "feature-1") {
		t.Errorf("Expected to be on 'feature-1', got '%s'", current)
	}

	t.Logf("✓ st new created branch and updated graph\nOutput: %s", output)
}

// TestAppendCommand tests the 'st append' command
func TestAppendCommand(t *testing.T) {
	repo, err := testutil.NewGitRepo()
	if err != nil {
		t.Fatalf("Failed to create test repo: %v", err)
	}
	defer repo.Cleanup()

	oldWd, _ := os.Getwd()
	os.Chdir(repo.Dir)
	defer os.Chdir(oldWd)

	// Setup: Create initial commit and feature-1
	if err := repo.InitStack(); err != nil {
		t.Fatalf("Failed to init stack: %v", err)
	}

	// Create feature-1 branch and graph entry manually
	if err := repo.CreateBranch("feature-1"); err != nil {
		t.Fatalf("Failed to create feature-1: %v", err)
	}
	if err := repo.CreateFile("feature1.txt", "content"); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddAndCommit("Feature 1 commit"); err != nil {
		t.Fatal(err)
	}

	// Create graph manually
	graphContent := `{"version":1,"root":"main","branches":{"feature-1":{"name":"feature-1","parent":"main","base_sha":"` + getHeadCommit(repo) + `","head_sha":"` + getHeadCommit(repo) + `"}}}`
	stackDir := filepath.Join(repo.Dir, ".git", "stack")
	os.MkdirAll(stackDir, 0755)
	os.WriteFile(filepath.Join(stackDir, "graph.json"), []byte(graphContent), 0644)

	// Checkout feature-1
	if err := repo.Checkout("feature-1"); err != nil {
		t.Fatalf("Failed to checkout feature-1: %v", err)
	}

	// Run 'st append feature-2'
	cmd := exec.Command(stBinaryPath, "append", "feature-2")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("st append failed: %v\nOutput: %s", err, output)
	}

	// Verify feature-2 was created
	if !repo.BranchExists("feature-2") {
		t.Error("Branch 'feature-2' was not created")
	}

	// Verify graph was updated
	content, _ := os.ReadFile(filepath.Join(stackDir, "graph.json"))
	if !strings.Contains(string(content), "feature-2") {
		t.Error("Graph doesn't contain 'feature-2'")
	}

	// Verify we're on feature-2
	current, _ := repo.CurrentBranch()
	if !strings.Contains(current, "feature-2") {
		t.Errorf("Expected to be on 'feature-2', got '%s'", current)
	}

	t.Logf("✓ st append created child branch\nOutput: %s", output)
}

// TestLogCommand tests the 'st log' command
func TestLogCommand(t *testing.T) {
	repo, err := testutil.NewGitRepo()
	if err != nil {
		t.Fatalf("Failed to create test repo: %v", err)
	}
	defer repo.Cleanup()

	oldWd, _ := os.Getwd()
	os.Chdir(repo.Dir)
	defer os.Chdir(oldWd)

	// Setup: Create initial commit
	if err := repo.InitStack(); err != nil {
		t.Fatalf("Failed to init stack: %v", err)
	}

	// Create a simple graph
	graphContent := `{"version":1,"root":"main","branches":{"feature-1":{"name":"feature-1","parent":"main","base_sha":"abc","head_sha":"def"}}}`
	stackDir := filepath.Join(repo.Dir, ".git", "stack")
	os.MkdirAll(stackDir, 0755)
	os.WriteFile(filepath.Join(stackDir, "graph.json"), []byte(graphContent), 0644)

	// Run 'st log'
	cmd := exec.Command(stBinaryPath, "log")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("st log failed: %v\nOutput: %s", err, output)
	}

	// Verify output contains expected structure
	if !strings.Contains(string(output), "main") {
		t.Error("Log output doesn't contain 'main'")
	}
	if !strings.Contains(string(output), "feature-1") {
		t.Error("Log output doesn't contain 'feature-1'")
	}

	t.Logf("✓ st log displayed stack\nOutput: %s", output)
}

// TestRestackCommand tests the 'st restack' command
func TestRestackCommand(t *testing.T) {
	repo, err := testutil.NewGitRepo()
	if err != nil {
		t.Fatalf("Failed to create test repo: %v", err)
	}
	defer repo.Cleanup()

	oldWd, _ := os.Getwd()
	os.Chdir(repo.Dir)
	defer os.Chdir(oldWd)

	// Setup: Create initial commit
	if err := repo.InitStack(); err != nil {
		t.Fatalf("Failed to init stack: %v", err)
	}

	// Get the actual root branch name (master or main)
	rootBranch, _ := repo.CurrentBranch()
	rootBranch = strings.TrimSpace(rootBranch)

	// Create a feature branch
	if err := repo.CreateBranch("feature-1"); err != nil {
		t.Fatalf("Failed to create feature-1: %v", err)
	}
	if err := repo.CreateFile("feature1.txt", "content1"); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddAndCommit("Feature 1 commit"); err != nil {
		t.Fatal(err)
	}

	// Create graph
	stackDir := filepath.Join(repo.Dir, ".git", "stack")
	os.MkdirAll(stackDir, 0755)
	commit1 := getHeadCommit(repo)
	graphContent := `{"version":1,"root":"` + rootBranch + `","branches":{"feature-1":{"name":"feature-1","parent":"` + rootBranch + `","base_sha":"` + commit1 + `","head_sha":"` + commit1 + `"}}}`
	os.WriteFile(filepath.Join(stackDir, "graph.json"), []byte(graphContent), 0644)

	// Make a commit on root branch to create divergence
	if err := repo.Checkout(rootBranch); err != nil {
		t.Fatalf("Failed to checkout %s: %v", rootBranch, err)
	}
	if err := repo.CreateFile("main-update.txt", "update"); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddAndCommit("Main update"); err != nil {
		t.Fatal(err)
	}

	// Checkout feature-1 and restack
	if err := repo.Checkout("feature-1"); err != nil {
		t.Fatalf("Failed to checkout feature-1: %v", err)
	}

	// Run 'st restack'
	cmd := exec.Command(stBinaryPath, "restack")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("st restack failed: %v\nOutput: %s", err, output)
	}

	// Verify output shows success
	if !strings.Contains(string(output), "Restacked") {
		t.Errorf("Expected 'Restacked' in output, got: %s", output)
	}

	t.Logf("✓ st restack completed\nOutput: %s", output)
}

// TestAttachAutoCommand tests 'st attach --auto' (non-interactive mode)
func TestAttachAutoCommand(t *testing.T) {
	repo, err := testutil.NewGitRepo()
	if err != nil {
		t.Fatalf("Failed to create test repo: %v", err)
	}
	defer repo.Cleanup()

	oldWd, _ := os.Getwd()
	os.Chdir(repo.Dir)
	defer os.Chdir(oldWd)

	// Setup: Create initial commit
	if err := repo.InitStack(); err != nil {
		t.Fatalf("Failed to init stack: %v", err)
	}

	// Get root branch name
	rootBranch := getCurrentBranchName(repo)

	// Create a tracked feature branch using st
	cmd := exec.Command(stBinaryPath, "new", "feature-1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("st new failed: %v\nOutput: %s", err, output)
	}

	// Checkout root to go back
	if err := repo.Checkout(rootBranch); err != nil {
		t.Fatalf("Failed to checkout %s: %v", rootBranch, err)
	}

	// Create a manual branch (outside of st)
	if err := repo.CreateBranch("manual-branch"); err != nil {
		t.Fatalf("Failed to create manual-branch: %v", err)
	}
	if err := repo.CreateFile("manual.txt", "manual content"); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddAndCommit("Manual commit"); err != nil {
		t.Fatal(err)
	}

	// Checkout the manual branch
	if err := repo.Checkout("manual-branch"); err != nil {
		t.Fatalf("Failed to checkout manual-branch: %v", err)
	}

	// Run 'st attach --auto' - should attach manual-branch to the best candidate
	cmd = exec.Command(stBinaryPath, "attach", "--auto")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("st attach --auto failed: %v\nOutput: %s", err, output)
	}

	// Verify graph was updated with manual-branch
	stackDir := filepath.Join(repo.Dir, ".git", "stack")
	content, _ := os.ReadFile(filepath.Join(stackDir, "graph.json"))
	if !strings.Contains(string(content), "manual-branch") {
		t.Errorf("Graph doesn't contain 'manual-branch'. Content: %s", string(content))
	}

	t.Logf("✓ st attach --auto attached branch\nOutput: %s", output)
}

// Helper to get root branch name
func getCurrentBranchName(repo *testutil.GitRepo) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repo.Dir
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

// Helper function to get current HEAD commit
func getHeadCommit(repo *testutil.GitRepo) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repo.Dir
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}
