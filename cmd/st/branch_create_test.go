package main

import (
	"testing"
)

// ---------------------------------------------------------------------------
// TestNew
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	t.Run("creates_branch_and_updates_graph", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		runSt(t, "new", "feature-1")

		if !repo.BranchExists("feature-1") {
			t.Error("branch feature-1 not created")
		}

		g := loadGraph(t, repo)
		b, ok := g.Branches["feature-1"]
		if !ok {
			t.Fatal("feature-1 not in graph")
		}
		if b.Parent != root {
			t.Errorf("parent = %q, want %q", b.Parent, root)
		}
		if cur := getCurrentBranch(t, repo); cur != "feature-1" {
			t.Errorf("current branch = %q, want feature-1", cur)
		}
	})

	t.Run("multiple_branches_from_root", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		runSt(t, "new", "a")
		repo.Checkout(root)
		runSt(t, "new", "b")

		g := loadGraph(t, repo)
		for _, name := range []string{"a", "b"} {
			b, ok := g.Branches[name]
			if !ok {
				t.Fatalf("%s not in graph", name)
			}
			if b.Parent != root {
				t.Errorf("%s parent = %q, want %q", name, b.Parent, root)
			}
		}
	})

	t.Run("creates_branch_from_root_not_current_head", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Create branch foo with a unique commit
		runSt(t, "new", "foo")
		if err := repo.CreateFile("foo.txt", "foo content"); err != nil {
			t.Fatal(err)
		}
		if err := repo.AddAndCommit("foo commit"); err != nil {
			t.Fatal(err)
		}
		fooSHA := repo.HeadSHA()

		// While still on foo, create bar via st new
		runSt(t, "new", "bar")

		// bar should be checked out
		if cur := getCurrentBranch(t, repo); cur != "bar" {
			t.Errorf("current branch = %q, want bar", cur)
		}

		// bar should be at the root's HEAD, not foo's HEAD
		barSHA := repo.HeadSHA()
		// Get root SHA for comparison
		if err := repo.Checkout(root); err != nil {
			t.Fatal(err)
		}
		rootSHA := repo.HeadSHA()
		if err := repo.Checkout("bar"); err != nil {
			t.Fatal(err)
		}

		if barSHA == fooSHA {
			t.Error("bar has same SHA as foo — should be based on root, not foo")
		}
		if barSHA != rootSHA {
			t.Errorf("bar SHA = %q, want root SHA %q", barSHA, rootSHA)
		}

		// bar should NOT have foo.txt
		if repo.FileExists("foo.txt") {
			t.Error("bar contains foo.txt — should not have foo's commits")
		}
	})
}

// ---------------------------------------------------------------------------
// TestAppend
// ---------------------------------------------------------------------------

func TestAppend(t *testing.T) {
	t.Run("creates_child_from_current", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)

		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")

		g := loadGraph(t, repo)
		b, ok := g.Branches["f2"]
		if !ok {
			t.Fatal("f2 not in graph")
		}
		if b.Parent != "f1" {
			t.Errorf("f2 parent = %q, want f1", b.Parent)
		}
		if cur := getCurrentBranch(t, repo); cur != "f2" {
			t.Errorf("current branch = %q, want f2", cur)
		}
	})

	t.Run("error_current_not_in_stack", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		// Create a tracked branch so graph exists with root != "untracked"
		runSt(t, "new", "tracked")
		repo.Checkout(root)
		repo.CreateBranch("untracked")
		out := runStExpectError(t, "append", "child")
		assertContains(t, out, "not in the stack")
	})
}

// ---------------------------------------------------------------------------
// TestInsert
// ---------------------------------------------------------------------------

func TestInsert(t *testing.T) {
	t.Run("inserts_before_current_and_reparents", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")

		runSt(t, "insert", "pre-f")

		g := loadGraph(t, repo)
		pref, ok := g.Branches["pre-f"]
		if !ok {
			t.Fatal("pre-f not in graph")
		}
		if pref.Parent != root {
			t.Errorf("pre-f parent = %q, want %q", pref.Parent, root)
		}

		f1, ok := g.Branches["f1"]
		if !ok {
			t.Fatal("f1 not in graph")
		}
		if f1.Parent != "pre-f" {
			t.Errorf("f1 parent = %q, want pre-f", f1.Parent)
		}

		if cur := getCurrentBranch(t, repo); cur != "pre-f" {
			t.Errorf("current branch = %q, want pre-f", cur)
		}
	})

	t.Run("error_current_not_in_stack", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		repo.CreateBranch("untracked")
		out := runStExpectError(t, "insert", "x")
		assertContains(t, out, "not in the stack")
	})
}
