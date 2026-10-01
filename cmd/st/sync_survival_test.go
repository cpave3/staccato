package main

import (
	"encoding/json"
	"github.com/cpave3/staccato/pkg/graph"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSyncChildSurvivesSecondSync(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create stack: root -> m1 -> m2
	runSt(t, "new", "m1")
	repo.CreateFile("m1.txt", "m1 content")
	repo.AddAndCommit("m1 commit")
	runSt(t, "append", "m2")
	repo.CreateFile("m2.txt", "m2 content")
	repo.AddAndCommit("m2 commit")

	// Push m1 to origin
	repo.RunGit("push", "origin", "m1")

	// Simulate regular merge of m1 into root on GitHub:
	// fast-forward origin/root to m1's commit, then delete origin/m1
	originDir := repo.OriginDir()
	m1SHA, _ := repo.RunGit("rev-parse", "m1")
	runGitInDir(t, originDir, "update-ref", "refs/heads/"+root, m1SHA)
	runGitInDir(t, originDir, "branch", "-D", "m1")

	// First sync from m2 (top of stack)
	repo.Checkout("m2")
	out1 := runSt(t, "-v", "sync")

	// m1 should be detected as merged
	assertContains(t, out1, "Merged")

	// After first sync: m2 should be reparented to root
	g1 := loadGraph(t, repo)
	m2b, ok := g1.Branches["m2"]
	if !ok {
		t.Fatal("m2 should still be in graph after first sync")
	}
	if m2b.Parent != root {
		t.Errorf("m2 parent after first sync = %q, want %q", m2b.Parent, root)
	}

	// Second sync — should be a no-op for merge detection
	out2 := runSt(t, "-v", "sync")

	// m2 should still be in the graph after second sync
	g2 := loadGraph(t, repo)
	m2b2, ok := g2.Branches["m2"]
	if !ok {
		t.Fatal("m2 should still be in graph after second sync")
	}
	if m2b2.Parent != root {
		t.Errorf("m2 parent after second sync = %q, want %q", m2b2.Parent, root)
	}

	// The second sync should NOT detect any merged branches
	_ = out2
}

// ---------------------------------------------------------------------------
// TestSyncChildSurvivesSecondSyncSquashMerge
// ---------------------------------------------------------------------------
// Same as above but with squash merge (more common on GitHub).

func TestSyncChildSurvivesSecondSyncSquashMerge(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create stack: root -> m1 -> m2
	runSt(t, "new", "m1")
	repo.CreateFile("m1.txt", "m1 content")
	repo.AddAndCommit("m1 commit")
	runSt(t, "append", "m2")
	repo.CreateFile("m2.txt", "m2 content")
	repo.AddAndCommit("m2 commit")

	// Push both to origin (realistic: user pushes their stack before merge)
	repo.RunGit("push", "origin", "m1")
	repo.RunGit("push", "origin", "m2")

	// Simulate squash merge of m1 into root:
	// Cherry-pick m1 changes onto root as a new commit, delete origin/m1
	repo.Checkout(root)
	repo.RunGit("cherry-pick", "m1")
	repo.RunGit("push", "origin", root)
	originDir := repo.OriginDir()
	runGitInDir(t, originDir, "branch", "-D", "m1")

	// First sync from m2
	repo.Checkout("m2")
	out1 := runSt(t, "-v", "sync")
	assertContains(t, out1, "Merged")

	g1 := loadGraph(t, repo)
	m2b, ok := g1.Branches["m2"]
	if !ok {
		t.Fatal("m2 should still be in graph after first sync")
	}
	if m2b.Parent != root {
		t.Errorf("m2 parent after first sync = %q, want %q", m2b.Parent, root)
	}

	// Second sync
	out2 := runSt(t, "-v", "sync")

	g2 := loadGraph(t, repo)
	m2b2, ok := g2.Branches["m2"]
	if !ok {
		t.Fatal("m2 should still be in graph after second sync")
	}
	if m2b2.Parent != root {
		t.Errorf("m2 parent after second sync = %q, want %q", m2b2.Parent, root)
	}
	_ = out2
}

// ---------------------------------------------------------------------------
// TestSyncChildSurvivesSharedGraphMode
// ---------------------------------------------------------------------------
// Regression: in shared graph mode, after merging a middle branch and syncing,
// the child branch should survive with correct parent after a second sync.

func TestSyncChildSurvivesSharedGraphMode(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create stack: root -> m1 -> m2
	runSt(t, "new", "m1")
	repo.CreateFile("m1.txt", "m1 content")
	repo.AddAndCommit("m1 commit")
	runSt(t, "append", "m2")
	repo.CreateFile("m2.txt", "m2 content")
	repo.AddAndCommit("m2 commit")

	// Enable shared graph mode (after branches exist so graph file exists)
	runSt(t, "graph", "share")

	// Push both to origin
	repo.RunGit("push", "origin", "m1")
	repo.RunGit("push", "origin", "m2")

	// Sync to push graph ref to origin
	runSt(t, "-v", "sync")

	// Simulate regular merge of m1 into root on GitHub:
	originDir := repo.OriginDir()
	m1SHA, _ := repo.RunGit("rev-parse", "m1")
	runGitInDir(t, originDir, "update-ref", "refs/heads/"+root, m1SHA)
	runGitInDir(t, originDir, "branch", "-D", "m1")

	// First sync from m2
	repo.Checkout("m2")
	out1 := runSt(t, "-v", "sync")
	assertContains(t, out1, "Merged")

	g1 := loadGraph(t, repo)
	m2b, ok := g1.Branches["m2"]
	if !ok {
		t.Fatal("m2 should still be in graph after first sync (shared mode)")
	}
	if m2b.Parent != root {
		t.Errorf("m2 parent after first sync = %q, want %q", m2b.Parent, root)
	}

	// Second sync
	out2 := runSt(t, "-v", "sync")

	g2 := loadGraph(t, repo)
	m2b2, ok := g2.Branches["m2"]
	if !ok {
		t.Fatal("m2 should still be in graph after second sync (shared mode)")
	}
	if m2b2.Parent != root {
		t.Errorf("m2 parent after second sync = %q, want %q", m2b2.Parent, root)
	}
	_ = out2
}

// ---------------------------------------------------------------------------
// TestSyncChildSurvivesSharedGraphStaleRemote
// ---------------------------------------------------------------------------
// Regression: in shared graph mode, if the graph ref push fails on first sync,
// the second sync's fetch overwrites the local ref with the stale remote graph.
// ReconcileGraphs must preserve the local parent (reparented during merge removal)
// rather than reverting to the stale remote parent.

func TestSyncChildSurvivesSharedGraphStaleRemote(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	// Create stack: root -> m1 -> m2
	runSt(t, "new", "m1")
	repo.CreateFile("m1.txt", "m1 content")
	repo.AddAndCommit("m1 commit")
	runSt(t, "append", "m2")
	repo.CreateFile("m2.txt", "m2 content")
	repo.AddAndCommit("m2 commit")

	// Enable shared graph mode
	runSt(t, "graph", "share")

	// Push both + graph ref to origin
	repo.RunGit("push", "origin", "m1")
	repo.RunGit("push", "origin", "m2")
	runSt(t, "-v", "sync")

	// Simulate regular merge of m1 into root on GitHub
	originDir := repo.OriginDir()
	m1SHA, _ := repo.RunGit("rev-parse", "m1")
	runGitInDir(t, originDir, "update-ref", "refs/heads/"+root, m1SHA)
	runGitInDir(t, originDir, "branch", "-D", "m1")

	// First sync from m2: detect merge, reparent m2
	repo.Checkout("m2")
	runSt(t, "-v", "sync")

	g1 := loadGraph(t, repo)
	if _, ok := g1.Branches["m2"]; !ok {
		t.Fatal("m2 should be in graph after first sync")
	}

	// Simulate the graph ref push having failed: revert origin's per-user ref
	// to the OLD version (with m1 still present, m2 parented under m1).
	email, _ := repo.RunGit("config", "user.email")
	userRef := graph.UserGraphRef(email)
	oldGraph := graph.NewGraph(root)
	oldGraph.AddBranch("m1", root, "aaa", m1SHA)
	m2SHA, _ := repo.RunGit("rev-parse", "m2")
	oldGraph.AddBranch("m2", "m1", "bbb", m2SHA)
	oldGraphJSON, _ := json.Marshal(oldGraph)

	// Write stale graph directly into origin bare repo's per-user ref
	tmpFile := filepath.Join(originDir, "stale-graph.json")
	os.WriteFile(tmpFile, oldGraphJSON, 0644)
	blobHash := runGitInDir(t, originDir, "hash-object", "-w", tmpFile)
	runGitInDir(t, originDir, "update-ref", userRef, blobHash)
	os.Remove(tmpFile)

	// Second sync: fetch will overwrite local graph ref with stale remote.
	// This should succeed — reconciliation should prefer local parent.
	out2 := runSt(t, "-v", "sync")

	g2 := loadGraph(t, repo)
	m2b2, ok := g2.Branches["m2"]
	if !ok {
		t.Fatal("m2 should still be in graph after second sync with stale remote")
	}
	if m2b2.Parent != root {
		t.Errorf("m2 parent after second sync = %q, want %q (stale remote should not revert reparent)", m2b2.Parent, root)
	}
	// m1 should NOT be re-added (it was merged and deleted locally)
	if graphContains(t, repo, "m1") {
		t.Error("m1 should not be re-added from stale remote graph")
	}
	_ = out2
}

// ---------------------------------------------------------------------------
// TestSyncSkipsUnpushedBranches
// ---------------------------------------------------------------------------

func TestSyncReconcileSharedGraph(t *testing.T) {
	// Same user, two machines. Machine A creates branchX, pushes.
	// Machine B (same user) creates branchY, syncs.
	// After sync on B, both X and Y should be in the graph.

	// Set up "Machine A" repo
	repoA, rootA := setupRepoWithStack(t)
	if err := repoA.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repoA.RunGit("push", "-u", "origin", rootA)

	// Create a branch first so graph.json exists, then switch to shared mode
	runSt(t, "new", "branchX")
	repoA.CreateFile("x.txt", "x")
	repoA.AddAndCommit("x commit")

	// Switch to shared graph mode
	runSt(t, "graph", "share")

	// Get the per-user ref name
	userRef := runSt(t, "graph", "which")

	// Push the per-user graph ref to remote
	email, _ := repoA.RunGit("config", "user.email")
	ref := graph.UserGraphRef(email)
	repoA.RunGit("push", "origin", ref+":"+ref, "--force")

	// Sync from A (pushes branchX and graph ref)
	runSt(t, "sync")

	// "Machine B": clone from the same bare origin (same user!)
	originDir := repoA.OriginDir()
	tmpB := t.TempDir()
	cloneCmd := exec.Command("git", "clone", originDir, tmpB)
	if out, err := cloneCmd.CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}

	// Save Machine A's wd and switch to Machine B
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpB); err != nil {
		t.Fatalf("chdir to B: %v", err)
	}
	t.Cleanup(func() { os.Chdir(oldWd) })

	// Same user on Machine B (same email — this is the key!)
	runGitInDir(t, tmpB, "config", "user.email", email)
	runGitInDir(t, tmpB, "config", "user.name", "Test User")

	// Set up shared graph fetch refspec on B and fetch
	runGitInDir(t, tmpB, "config", "--add", "remote.origin.fetch", "+refs/staccato/*:refs/staccato/*")
	runGitInDir(t, tmpB, "fetch", "origin")

	// Create local tracking branch for branchX on Machine B
	runGitInDir(t, tmpB, "checkout", "-b", "branchX", "origin/branchX")
	runGitInDir(t, tmpB, "checkout", rootA)

	// Machine B creates branch Y — st new adds it to the per-user graph ref
	runSt(t, "new", "branchY")
	os.WriteFile(filepath.Join(tmpB, "y.txt"), []byte("y"), 0644)
	runGitInDir(t, tmpB, "add", ".")
	runGitInDir(t, tmpB, "commit", "-m", "y commit")

	// Sync from B should reconcile: fetch brings A's version of the graph
	// (with branchX), local has branchY. Union merge keeps both.
	runSt(t, "sync")

	// Load the graph on Machine B and check both branches exist
	g := loadGraphInDir(t, tmpB)
	if _, ok := g.Branches["branchX"]; !ok {
		t.Error("branchX should be in graph after reconciliation on Machine B")
	}
	if _, ok := g.Branches["branchY"]; !ok {
		t.Error("branchY should be in graph after reconciliation on Machine B")
	}
	_ = userRef
}
