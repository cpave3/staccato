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

// ---------------------------------------------------------------------------
// TestGetUnattachedBranches
// ---------------------------------------------------------------------------

func TestGetUnattachedBranches(t *testing.T) {
	t.Run("returns_untracked_branches", func(t *testing.T) {
		tmpDir := t.TempDir()

		cmd := exec.Command("git", "init", "-b", "main")
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
		exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

		testFile := filepath.Join(tmpDir, "test.txt")
		os.WriteFile(testFile, []byte("main"), 0644)
		exec.Command("git", "-C", tmpDir, "add", ".").Run()
		exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

		// Create tracked and untracked branches
		exec.Command("git", "-C", tmpDir, "branch", "feature").Run()
		exec.Command("git", "-C", tmpDir, "branch", "other").Run()

		gitRunner := git.NewRunner(tmpDir)
		g := graph.NewGraph("main")
		mainSHA, _ := gitRunner.GetBranchSHA("main")
		featureSHA, _ := gitRunner.GetBranchSHA("feature")
		g.AddBranch("main", "", "", mainSHA)
		g.AddBranch("feature", "main", mainSHA, featureSHA)

		attacher := NewAttacher(gitRunner, nil)
		unattached, err := attacher.GetUnattachedBranches(g)
		if err != nil {
			t.Fatalf("GetUnattachedBranches: %v", err)
		}

		found := false
		for _, b := range unattached {
			if b == "other" {
				found = true
			}
			if b == "feature" || b == "main" {
				t.Errorf("tracked branch %q should not be in unattached list", b)
			}
		}
		if !found {
			t.Errorf("expected 'other' in unattached, got: %v", unattached)
		}
	})

	t.Run("all_attached_returns_empty", func(t *testing.T) {
		tmpDir := t.TempDir()

		cmd := exec.Command("git", "init", "-b", "main")
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
		exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

		testFile := filepath.Join(tmpDir, "test.txt")
		os.WriteFile(testFile, []byte("main"), 0644)
		exec.Command("git", "-C", tmpDir, "add", ".").Run()
		exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

		gitRunner := git.NewRunner(tmpDir)
		g := graph.NewGraph("main")

		attacher := NewAttacher(gitRunner, nil)
		unattached, err := attacher.GetUnattachedBranches(g)
		if err != nil {
			t.Fatalf("GetUnattachedBranches: %v", err)
		}

		if len(unattached) != 0 {
			t.Errorf("expected empty unattached list, got: %v", unattached)
		}
	})
}

// ---------------------------------------------------------------------------
// TestAutoAttach
// ---------------------------------------------------------------------------

func TestAutoAttach(t *testing.T) {
	t.Run("auto_selects_best_parent", func(t *testing.T) {
		tmpDir := t.TempDir()

		cmd := exec.Command("git", "init", "-b", "main")
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
		exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

		testFile := filepath.Join(tmpDir, "test.txt")
		os.WriteFile(testFile, []byte("main"), 0644)
		exec.Command("git", "-C", tmpDir, "add", ".").Run()
		exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

		exec.Command("git", "-C", tmpDir, "checkout", "-b", "feature").Run()
		os.WriteFile(testFile, []byte("feature"), 0644)
		exec.Command("git", "-C", tmpDir, "add", ".").Run()
		exec.Command("git", "-C", tmpDir, "commit", "-m", "feature commit").Run()
		exec.Command("git", "-C", tmpDir, "checkout", "main").Run()

		gitRunner := git.NewRunner(tmpDir)
		g := graph.NewGraph("main")
		mainSHA, _ := gitRunner.GetBranchSHA("main")
		g.AddBranch("main", "", "", mainSHA)

		attacher := NewAttacher(gitRunner, nil)
		err := attacher.AutoAttach(g, "feature", true)
		if err != nil {
			t.Fatalf("AutoAttach: %v", err)
		}

		if _, ok := g.GetBranch("feature"); !ok {
			t.Error("feature should be in graph after AutoAttach")
		}
	})

	t.Run("already_in_graph_returns_nil", func(t *testing.T) {
		g := graph.NewGraph("main")
		g.AddBranch("feature", "main", "abc", "def")

		attacher := NewAttacher(nil, nil)
		err := attacher.AutoAttach(g, "feature", true)
		if err != nil {
			t.Errorf("expected nil for already-tracked branch, got: %v", err)
		}
	})

	t.Run("no_candidates_returns_error", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create two completely unrelated repos to ensure no merge base
		cmd := exec.Command("git", "init", "-b", "main")
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
		exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

		testFile := filepath.Join(tmpDir, "test.txt")
		os.WriteFile(testFile, []byte("main"), 0644)
		exec.Command("git", "-C", tmpDir, "add", ".").Run()
		exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

		// Create orphan branch (no common ancestor)
		exec.Command("git", "-C", tmpDir, "checkout", "--orphan", "orphan").Run()
		exec.Command("git", "-C", tmpDir, "rm", "-rf", ".").Run()
		os.WriteFile(filepath.Join(tmpDir, "orphan.txt"), []byte("orphan"), 0644)
		exec.Command("git", "-C", tmpDir, "add", ".").Run()
		exec.Command("git", "-C", tmpDir, "commit", "-m", "orphan initial").Run()
		exec.Command("git", "-C", tmpDir, "checkout", "main").Run()

		gitRunner := git.NewRunner(tmpDir)
		g := graph.NewGraph("main")

		attacher := NewAttacher(gitRunner, nil)
		err := attacher.AutoAttach(g, "orphan", true)
		if err == nil {
			t.Fatal("expected error for orphan branch with no candidates")
		}
	})
}

// ---------------------------------------------------------------------------
// TestRecursivelyAttach
// ---------------------------------------------------------------------------

func TestRecursivelyAttach(t *testing.T) {
	t.Run("parent_in_graph", func(t *testing.T) {
		tmpDir := t.TempDir()

		cmd := exec.Command("git", "init", "-b", "main")
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
		exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

		testFile := filepath.Join(tmpDir, "test.txt")
		os.WriteFile(testFile, []byte("main"), 0644)
		exec.Command("git", "-C", tmpDir, "add", ".").Run()
		exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

		exec.Command("git", "-C", tmpDir, "checkout", "-b", "child").Run()
		os.WriteFile(testFile, []byte("child"), 0644)
		exec.Command("git", "-C", tmpDir, "add", ".").Run()
		exec.Command("git", "-C", tmpDir, "commit", "-m", "child commit").Run()
		exec.Command("git", "-C", tmpDir, "checkout", "main").Run()

		gitRunner := git.NewRunner(tmpDir)
		g := graph.NewGraph("main")

		attacher := NewAttacher(gitRunner, nil)
		err := attacher.RecursivelyAttach(g, "child", "main")
		if err != nil {
			t.Fatalf("RecursivelyAttach: %v", err)
		}

		if _, ok := g.GetBranch("child"); !ok {
			t.Error("child should be in graph")
		}
	})

	t.Run("parent_not_in_graph_errors", func(t *testing.T) {
		tmpDir := t.TempDir()

		cmd := exec.Command("git", "init", "-b", "main")
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
		exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

		testFile := filepath.Join(tmpDir, "test.txt")
		os.WriteFile(testFile, []byte("main"), 0644)
		exec.Command("git", "-C", tmpDir, "add", ".").Run()
		exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

		exec.Command("git", "-C", tmpDir, "branch", "child").Run()
		exec.Command("git", "-C", tmpDir, "branch", "middle").Run()

		gitRunner := git.NewRunner(tmpDir)
		g := graph.NewGraph("main")

		attacher := NewAttacher(gitRunner, nil)
		err := attacher.RecursivelyAttach(g, "child", "middle")
		if err == nil {
			t.Fatal("expected error when parent not in graph")
		}
	})
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
