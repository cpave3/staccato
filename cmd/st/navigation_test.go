package main

import (
	"testing"
)

// ---------------------------------------------------------------------------
// TestUp
// ---------------------------------------------------------------------------

func TestUp(t *testing.T) {
	t.Run("checks_out_single_child", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")

		// Go back to f1, then up should take us to f2
		repo.Checkout("f1")
		runSt(t, "up")
		if cur := getCurrentBranch(t, repo); cur != "f2" {
			t.Errorf("current branch = %q, want f2", cur)
		}

		// From root, up should go to f1 (single child)
		repo.Checkout(root)
		runSt(t, "up")
		if cur := getCurrentBranch(t, repo); cur != "f1" {
			t.Errorf("current branch = %q, want f1", cur)
		}
	})

	t.Run("error_multiple_children", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "a")
		repo.Checkout(root)
		runSt(t, "new", "b")
		repo.Checkout(root)

		out := runStExpectError(t, "up")
		assertContains(t, out, "multiple")
	})

	t.Run("error_at_tip", func(t *testing.T) {
		_, _ = setupRepoWithStack(t)
		runSt(t, "new", "f1")

		out := runStExpectError(t, "up")
		assertContains(t, out, "tip")
	})

	t.Run("error_not_in_stack", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "tracked")
		repo.Checkout(root)
		repo.CreateBranch("untracked")

		out := runStExpectError(t, "up")
		assertContains(t, out, "not in the stack")
	})
}

// ---------------------------------------------------------------------------
// TestDown
// ---------------------------------------------------------------------------

func TestDown(t *testing.T) {
	t.Run("checks_out_parent", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")

		// From f2, down should go to f1
		runSt(t, "down")
		if cur := getCurrentBranch(t, repo); cur != "f1" {
			t.Errorf("current branch = %q, want f1", cur)
		}

		// From f1, down should go to root
		runSt(t, "down")
		if cur := getCurrentBranch(t, repo); cur != root {
			t.Errorf("current branch = %q, want %q", cur, root)
		}
	})

	t.Run("error_at_root", func(t *testing.T) {
		_, _ = setupRepoWithStack(t)

		out := runStExpectError(t, "down")
		assertContains(t, out, "bottom")
	})

	t.Run("error_not_in_stack", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "tracked")
		repo.Checkout(root)
		repo.CreateBranch("untracked")

		out := runStExpectError(t, "down")
		assertContains(t, out, "not in the stack")
	})
}

// ---------------------------------------------------------------------------
// TestTop
// ---------------------------------------------------------------------------

func TestTop(t *testing.T) {
	t.Run("follows_single_child_to_tip", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2")
		repo.AddAndCommit("f2 commit")
		runSt(t, "append", "f3")

		// From root, top should go to f3
		repo.Checkout(root)
		runSt(t, "top")
		if cur := getCurrentBranch(t, repo); cur != "f3" {
			t.Errorf("current branch = %q, want f3", cur)
		}
	})

	t.Run("error_on_fork", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2a")
		repo.Checkout("f1")
		runSt(t, "append", "f2b")
		repo.Checkout("f1")

		out := runStExpectError(t, "top")
		assertContains(t, out, "multiple")
	})

	t.Run("already_at_tip", func(t *testing.T) {
		_, _ = setupRepoWithStack(t)
		runSt(t, "new", "f1")

		out := runSt(t, "top")
		assertContains(t, out, "already")
	})
}

// ---------------------------------------------------------------------------
// TestBottom
// ---------------------------------------------------------------------------

func TestBottom(t *testing.T) {
	t.Run("navigates_to_first_child_of_root", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2")
		repo.AddAndCommit("f2 commit")
		runSt(t, "append", "f3")

		// From f3, bottom should go to f1
		runSt(t, "bottom")
		if cur := getCurrentBranch(t, repo); cur != "f1" {
			t.Errorf("current branch = %q, want f1", cur)
		}
	})

	t.Run("already_at_bottom", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")
		repo.Checkout("f1")

		out := runSt(t, "bottom")
		assertContains(t, out, "already")
	})

	t.Run("on_root_with_single_child", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.Checkout(root)

		runSt(t, "bottom")
		if cur := getCurrentBranch(t, repo); cur != "f1" {
			t.Errorf("current branch = %q, want f1", cur)
		}
	})
}
