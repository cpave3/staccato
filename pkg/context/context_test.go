package context

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cpave3/staccato/internal/testutil"
	"github.com/cpave3/staccato/pkg/git"
	"github.com/cpave3/staccato/pkg/graph"
)

// setupRepo creates an initialized git repo with a stack directory and returns
// the repo helper and the default branch name.
func setupRepo(t *testing.T) (*testutil.GitRepo, string) {
	t.Helper()
	repo, err := testutil.NewGitRepo()
	if err != nil {
		t.Fatalf("NewGitRepo: %v", err)
	}
	if err := repo.InitStack(); err != nil {
		t.Fatalf("InitStack: %v", err)
	}
	branch, err := repo.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	branch = strings.TrimSpace(branch)
	return repo, branch
}

// --- IsTrunkBranch tests ---

func TestIsTrunkBranch_KnownTrunks(t *testing.T) {
	for _, name := range []string{"main", "master", "develop", "trunk"} {
		if !IsTrunkBranch(name) {
			t.Errorf("IsTrunkBranch(%q) = false, want true", name)
		}
	}
}

func TestIsTrunkBranch_NonTrunk(t *testing.T) {
	for _, name := range []string{"feature-x", "release/1.0", "hotfix", ""} {
		if IsTrunkBranch(name) {
			t.Errorf("IsTrunkBranch(%q) = true, want false", name)
		}
	}
}

// --- Load tests ---

func TestLoad_LocalFile(t *testing.T) {
	repo, root := setupRepo(t)
	defer repo.Cleanup()

	// Write a graph to the local file
	g := graph.NewGraph(root)
	g.AddBranch("feature-a", root, "abc123", "def456")
	graphPath := filepath.Join(repo.Dir, graph.DefaultGraphPath)
	if err := g.Save(graphPath); err != nil {
		t.Fatalf("Save graph: %v", err)
	}

	sc, err := Load(repo.Dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if sc.Graph.Root != root {
		t.Errorf("Root = %q, want %q", sc.Graph.Root, root)
	}
	if _, ok := sc.Graph.GetBranch("feature-a"); !ok {
		t.Error("expected branch feature-a in loaded graph")
	}
}

func TestLoad_SharedRef(t *testing.T) {
	repo, root := setupRepo(t)
	defer repo.Cleanup()

	// Write a graph to the shared ref
	g := graph.NewGraph(root)
	g.AddBranch("feature-b", root, "aaa", "bbb")
	data, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	gitRunner := git.NewRunner(repo.Dir)
	if err := gitRunner.WriteBlobRef(graph.SharedGraphRef, data); err != nil {
		t.Fatalf("WriteBlobRef: %v", err)
	}

	sc, err := Load(repo.Dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if sc.Graph.Root != root {
		t.Errorf("Root = %q, want %q", sc.Graph.Root, root)
	}
	if _, ok := sc.Graph.GetBranch("feature-b"); !ok {
		t.Error("expected branch feature-b in loaded graph")
	}
	if sc.Graph.Branches == nil {
		t.Error("expected Branches map to be initialized")
	}
}

func TestLoad_NewGraphFallback(t *testing.T) {
	repo, root := setupRepo(t)
	defer repo.Cleanup()

	// No graph file, no shared ref — should create new graph
	sc, err := Load(repo.Dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if sc.Graph.Root != root {
		t.Errorf("Root = %q, want %q", sc.Graph.Root, root)
	}
	if len(sc.Graph.Branches) != 0 {
		t.Errorf("expected empty branches, got %d", len(sc.Graph.Branches))
	}
}

func TestLoad_NotGitRepo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "st-nogit-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, err = Load(tmpDir)
	if err == nil {
		t.Fatal("expected error for non-git directory")
	}
	if got := err.Error(); got != "not a git repository" {
		t.Errorf("error = %q, want %q", got, "not a git repository")
	}
}

// --- Save tests ---

func TestSave_LocalFile(t *testing.T) {
	repo, root := setupRepo(t)
	defer repo.Cleanup()

	sc, err := Load(repo.Dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	sc.Graph.AddBranch("save-test", root, "aaa", "bbb")

	if err := sc.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify by reading the file directly
	graphPath := filepath.Join(repo.Dir, graph.DefaultGraphPath)
	loaded, err := graph.LoadGraph(graphPath)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	if _, ok := loaded.GetBranch("save-test"); !ok {
		t.Error("expected save-test branch in saved graph")
	}
}

func TestSave_SharedRef(t *testing.T) {
	repo, root := setupRepo(t)
	defer repo.Cleanup()

	// Set up shared ref first
	g := graph.NewGraph(root)
	data, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	gitRunner := git.NewRunner(repo.Dir)
	if err := gitRunner.WriteBlobRef(graph.SharedGraphRef, data); err != nil {
		t.Fatalf("WriteBlobRef: %v", err)
	}

	sc, err := Load(repo.Dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	sc.Graph.AddBranch("shared-test", root, "xxx", "yyy")

	if err := sc.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify by reading the ref directly
	readData, err := gitRunner.ReadBlobRef(graph.SharedGraphRef)
	if err != nil {
		t.Fatalf("ReadBlobRef: %v", err)
	}
	var loaded graph.Graph
	if err := json.Unmarshal(readData, &loaded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, ok := loaded.GetBranch("shared-test"); !ok {
		t.Error("expected shared-test branch in saved graph")
	}
}
