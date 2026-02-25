package attach

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cpave3/staccato/pkg/git"
	"github.com/cpave3/staccato/pkg/graph"
)

func TestAttachment_SuggestsCandidates(t *testing.T) {
	tmpDir := t.TempDir()

	// Init git repo
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	// Create main with commit
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("main"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	// Create feature branch from main
	exec.Command("git", "-C", tmpDir, "checkout", "-b", "feature").Run()
	os.WriteFile(testFile, []byte("feature"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "feature commit").Run()

	gitRunner := git.NewRunner(tmpDir)
	g := graph.NewGraph("main")

	// Simulate that "main" is in the graph but "feature" is not
	mainSHA, _ := gitRunner.GetBranchSHA("main")
	g.AddBranch("main", "", "", mainSHA)

	attacher := NewAttacher(gitRunner, nil)

	candidates, err := attacher.SuggestParents(g, "feature")
	if err != nil {
		t.Fatalf("failed to suggest parents: %v", err)
	}

	// Should suggest main as a parent
	found := false
	for _, candidate := range candidates {
		if candidate == "main" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected 'main' to be suggested as parent, got: %v", candidates)
	}
}

func TestAttachment_CanAttachBranch(t *testing.T) {
	tmpDir := t.TempDir()

	// Init git repo
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	// Create main
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("main"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	// Create feature branch
	exec.Command("git", "-C", tmpDir, "checkout", "-b", "feature").Run()
	os.WriteFile(testFile, []byte("feature"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "feature commit").Run()

	gitRunner := git.NewRunner(tmpDir)
	g := graph.NewGraph("main")

	// Track main in graph
	mainSHA, _ := gitRunner.GetBranchSHA("main")
	g.AddBranch("main", "", "", mainSHA)

	attacher := NewAttacher(gitRunner, nil)

	// Attach feature with main as parent
	err := attacher.AttachBranch(g, "feature", "main")
	if err != nil {
		t.Fatalf("failed to attach branch: %v", err)
	}

	// Verify branch was added to graph
	branch, exists := g.GetBranch("feature")
	if !exists {
		t.Fatal("expected feature to be added to graph")
	}

	if branch.Parent != "main" {
		t.Errorf("expected parent to be 'main', got: %s", branch.Parent)
	}
}

func TestAttachment_DetectsAlreadyInGraph(t *testing.T) {
	g := graph.NewGraph("main")
	g.AddBranch("feature", "main", "abc123", "def456")

	attacher := NewAttacher(nil, nil)

	inGraph := attacher.IsBranchInGraph(g, "feature")
	if !inGraph {
		t.Error("expected feature to be detected as in graph")
	}

	inGraph = attacher.IsBranchInGraph(g, "unknown")
	if inGraph {
		t.Error("expected unknown branch to not be in graph")
	}
}

func TestAttachment_FindsRootBranch(t *testing.T) {
	g := graph.NewGraph("main")
	g.AddBranch("feature-a", "main", "abc123", "def456")
	g.AddBranch("feature-b", "feature-a", "def456", "ghi789")

	attacher := NewAttacher(nil, nil)

	root := attacher.FindRoot(g, "feature-b")
	if root != "main" {
		t.Errorf("expected root to be 'main', got: %s", root)
	}

	root = attacher.FindRoot(g, "feature-a")
	if root != "main" {
		t.Errorf("expected root to be 'main', got: %s", root)
	}
}
