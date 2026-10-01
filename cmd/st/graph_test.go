package main

import (
	stcontext "github.com/cpave3/staccato/pkg/context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TestGraph
// ---------------------------------------------------------------------------

func TestGraph(t *testing.T) {
	t.Run("which_defaults_to_local", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")

		out := runSt(t, "graph", "which")
		assertContains(t, out, "Local")
	})

	t.Run("share_moves_to_ref", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")

		runSt(t, "graph", "share")

		// Local file should be gone
		localPath := filepath.Join(repo.Dir, ".git", "stack", "graph.json")
		if _, err := os.Stat(localPath); !os.IsNotExist(err) {
			t.Error("local graph file should be removed after share")
		}

		// Per-user ref should exist
		out := runSt(t, "graph", "which")
		assertContains(t, out, "Shared")
	})

	t.Run("which_reports_shared_after_share", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")
		runSt(t, "graph", "share")

		out := runSt(t, "graph", "which")
		assertContains(t, out, "Shared")
	})

	t.Run("log_works_after_share", func(t *testing.T) {
		_, root := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		runSt(t, "graph", "share")

		out := runSt(t, "log")
		assertContains(t, out, root)
		assertContains(t, out, "f1")
	})

	t.Run("local_moves_back_from_ref", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		runSt(t, "graph", "share")
		runSt(t, "graph", "local")

		// Local file should be restored
		localPath := filepath.Join(repo.Dir, ".git", "stack", "graph.json")
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			t.Error("local graph file should exist after local")
		}

		// Should be back to local mode
		out := runSt(t, "graph", "which")
		assertContains(t, out, "Local")
	})

	t.Run("share_when_no_local_graph_errors", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		// Remove the graph file so share has nothing to move
		os.Remove(filepath.Join(repo.Dir, ".git", "stack", "graph.json"))
		out := runStExpectError(t, "graph", "share")
		assertContains(t, out, "no local graph")
	})

	t.Run("share_when_already_shared_errors", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")
		runSt(t, "graph", "share")
		out := runStExpectError(t, "graph", "share")
		assertContains(t, out, "already shared")
	})

	t.Run("local_when_already_local_errors", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")
		out := runStExpectError(t, "graph", "local")
		assertContains(t, out, "already local")
	})

	t.Run("new_branch_works_after_share", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "graph", "share")

		// Create another branch while in shared mode
		runSt(t, "append", "f2")

		g := loadGraph(t, repo)
		if _, ok := g.Branches["f2"]; !ok {
			t.Error("f2 should be in graph after append in shared mode")
		}
		if g.Branches["f2"].Parent != "f1" {
			t.Errorf("f2 parent = %q, want f1", g.Branches["f2"].Parent)
		}
	})
}

func TestGraphRepairRestoredStack(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	runSt(t, "new", "repair-parent")
	repo.CreateFile("parent.txt", "parent")
	repo.AddAndCommit("parent change")
	parent, _ := repo.RunGit("rev-parse", "repair-parent")
	runSt(t, "append", "repair-child")
	repo.CreateFile("child.txt", "child")
	repo.AddAndCommit("child change")
	child, _ := repo.RunGit("rev-parse", "repair-child")
	g := loadGraph(t, repo)
	g.UpdateBranch("repair-parent", "stale", "stale")
	g.UpdateBranch("repair-child", "stale", "stale")
	sc, err := stcontext.Load(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	sc.Graph = g
	if err := sc.Save(); err != nil {
		t.Fatal(err)
	}
	out := runSt(t, "graph", "repair", "--dry-run")
	assertContains(t, out, "repair-child")
	if loadGraph(t, repo).Branches["repair-child"].BaseSHA != "stale" {
		t.Fatal("dry run changed the graph")
	}
	runSt(t, "graph", "repair")
	repaired := loadGraph(t, repo)
	if repaired.Branches["repair-child"].BaseSHA != parent || repaired.Branches["repair-child"].HeadSHA != child {
		t.Fatalf("child metadata not repaired: %+v", repaired.Branches["repair-child"])
	}
	if repaired.Branches["repair-parent"].Parent != root {
		t.Fatal("parent relationship changed")
	}
}

func TestRestackRefusesUntrustworthyBase(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	runSt(t, "new", "unsafe-parent")
	repo.CreateFile("shared.txt", "parent")
	repo.AddAndCommit("parent change")
	runSt(t, "append", "unsafe-child")
	repo.CreateFile("child.txt", "child")
	repo.AddAndCommit("child change")
	child, _ := repo.RunGit("rev-parse", "unsafe-child")
	g := loadGraph(t, repo)
	// Simulate a restored child whose parent was rewritten independently.
	repo.Checkout(root)
	repo.CreateFile("shared.txt", "rewritten")
	repo.AddAndCommit("rewritten parent")
	repo.RunGit("branch", "-f", "unsafe-parent", "HEAD")
	g.Branches["unsafe-child"].BaseSHA, _ = repo.RunGit("rev-parse", "unsafe-parent")
	sc, err := stcontext.Load(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	sc.Graph = g
	if err := sc.Save(); err != nil {
		t.Fatal(err)
	}
	repo.Checkout("unsafe-child")
	out := runStExpectError(t, "restack")
	assertContains(t, out, "st graph repair")
	after, _ := repo.RunGit("rev-parse", "unsafe-child")
	if after != child {
		t.Fatal("restack rewrote the child before detecting unsafe metadata")
	}
	repo.RunGit("rebase", "--abort")
	out = runStExpectError(t, "graph", "repair")
	assertContains(t, out, "cannot repair unsafe-child")
}

func TestGraphRepairOnlyCurrentStack(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	runSt(t, "new", "repair-left")
	repo.CreateFile("left.txt", "left")
	repo.AddAndCommit("left")
	left, _ := repo.RunGit("rev-parse", "repair-left")
	runSt(t, "append", "repair-left-child")
	repo.CreateFile("left-child.txt", "child")
	repo.AddAndCommit("left child")
	repo.Checkout(root)
	runSt(t, "new", "repair-right")
	repo.CreateFile("right.txt", "right")
	repo.AddAndCommit("right")
	repo.Checkout("repair-left-child")
	g := loadGraph(t, repo)
	g.Branches["repair-left"].BaseSHA = "stale-left"
	g.Branches["repair-right"].BaseSHA = "stale-right"
	sc, err := stcontext.Load(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	sc.Graph = g
	if err := sc.Save(); err != nil {
		t.Fatal(err)
	}
	out := runSt(t, "graph", "repair")
	assertContains(t, out, "repair-left")
	if strings.Contains(out, "repair-right") {
		t.Fatalf("repaired sibling stack: %s", out)
	}
	repaired := loadGraph(t, repo)
	if repaired.Branches["repair-left"].HeadSHA != left || repaired.Branches["repair-left"].BaseSHA == "stale-left" {
		t.Fatal("current stack was not repaired")
	}
	if repaired.Branches["repair-right"].BaseSHA != "stale-right" {
		t.Fatal("sibling stack changed")
	}
}
