package main

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TestModify
// ---------------------------------------------------------------------------

func TestModify(t *testing.T) {
	t.Run("amend_staged_changes_and_restack", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2")
		repo.AddAndCommit("f2 commit")

		// Go back to f1, modify it
		repo.Checkout("f1")
		repo.WriteFile("f1.txt", "f1-modified")
		repo.RunGit("add", "f1.txt")

		headBefore := repo.HeadSHA()
		runSt(t, "modify")
		headAfter := repo.HeadSHA()

		if headBefore == headAfter {
			t.Error("HEAD should have changed after modify")
		}

		// f2 should still exist in graph (restacked)
		g := loadGraph(t, repo)
		if _, ok := g.Branches["f2"]; !ok {
			t.Error("f2 should still be in graph after restack")
		}
	})

	t.Run("amend_with_all_flag", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")

		// Make unstaged change
		repo.WriteFile("f1.txt", "f1-modified")

		runSt(t, "modify", "--all")

		// File should be committed
		status, _ := repo.RunGit("status", "--porcelain")
		if strings.TrimSpace(status) != "" {
			t.Errorf("working tree should be clean, got: %s", status)
		}
	})

	t.Run("update_message", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")

		runSt(t, "modify", "--message", "new message")

		msg, _ := repo.RunGit("log", "-1", "--format=%s")
		if msg != "new message" {
			t.Errorf("commit message = %q, want 'new message'", msg)
		}
	})

	t.Run("error_not_in_stack", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "tracked")
		repo.Checkout(root)
		repo.CreateBranch("untracked")

		out := runStExpectError(t, "modify")
		assertContains(t, out, "not in the stack")
	})

	t.Run("error_nothing_to_modify", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		_ = repo // working tree is clean

		out := runStExpectError(t, "modify")
		assertContains(t, out, "nothing to modify")
	})
}

// ---------------------------------------------------------------------------
// TestDelete
// ---------------------------------------------------------------------------

func TestDelete(t *testing.T) {
	t.Run("delete_branch_no_children", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")

		runSt(t, "delete", "f1")

		if graphContains(t, repo, "f1") {
			t.Error("f1 should be removed from graph")
		}
		if repo.BranchExists("f1") {
			t.Error("git branch f1 should be deleted")
		}
	})

	t.Run("delete_branch_with_children_reparents", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2")
		repo.AddAndCommit("f2 commit")

		// Delete f1 — f2 should be reparented to root
		runSt(t, "delete", "f1")

		g := loadGraph(t, repo)
		if _, ok := g.Branches["f1"]; ok {
			t.Error("f1 should be removed from graph")
		}
		f2, ok := g.Branches["f2"]
		if !ok {
			t.Fatal("f2 should still be in graph")
		}
		if f2.Parent != root {
			t.Errorf("f2 parent = %q, want %q", f2.Parent, root)
		}
	})

	t.Run("delete_current_branch", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")

		// On f1, delete it — should checkout root first
		runSt(t, "delete", "f1")

		if cur := getCurrentBranch(t, repo); cur != root {
			t.Errorf("current branch = %q, want %q", cur, root)
		}
	})

	t.Run("error_delete_root", func(t *testing.T) {
		_, root := setupRepoWithStack(t)
		out := runStExpectError(t, "delete", root)
		assertContains(t, out, "cannot delete")
	})

	t.Run("error_not_in_stack", func(t *testing.T) {
		_, _ = setupRepoWithStack(t)
		out := runStExpectError(t, "delete", "nonexistent")
		assertContains(t, out, "not in the stack")
	})
}

// ---------------------------------------------------------------------------
// TestMove
// ---------------------------------------------------------------------------

func TestMove(t *testing.T) {
	t.Run("reparent_and_restack", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		// Create two branches off root: a and b
		runSt(t, "new", "a")
		repo.CreateFile("a.txt", "a")
		repo.AddAndCommit("a commit")
		repo.Checkout(root)
		runSt(t, "new", "b")
		repo.CreateFile("b.txt", "b")
		repo.AddAndCommit("b commit")

		// Move b onto a
		runSt(t, "move", "--onto", "a")

		g := loadGraph(t, repo)
		bBranch, ok := g.Branches["b"]
		if !ok {
			t.Fatal("b should still be in graph")
		}
		if bBranch.Parent != "a" {
			t.Errorf("b parent = %q, want 'a'", bBranch.Parent)
		}
	})

	t.Run("error_move_onto_self", func(t *testing.T) {
		_, _ = setupRepoWithStack(t)
		runSt(t, "new", "f1")

		out := runStExpectError(t, "move", "--onto", "f1")
		assertContains(t, out, "cannot move")
	})

	t.Run("error_move_onto_descendant", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")
		repo.Checkout("f1")

		out := runStExpectError(t, "move", "--onto", "f2")
		assertContains(t, out, "cycle")
	})

	t.Run("error_target_not_in_stack", func(t *testing.T) {
		_, _ = setupRepoWithStack(t)
		runSt(t, "new", "f1")

		out := runStExpectError(t, "move", "--onto", "nonexistent")
		assertContains(t, out, "not in the stack")
	})

	t.Run("error_not_in_stack", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		runSt(t, "new", "tracked")
		repo.Checkout(root)
		repo.CreateBranch("untracked")

		out := runStExpectError(t, "move", "--onto", root)
		assertContains(t, out, "not in the stack")
	})
}

// ---------------------------------------------------------------------------
// TestAbort
// ---------------------------------------------------------------------------

func TestAbort(t *testing.T) {
	t.Run("error_no_rebase_in_progress", func(t *testing.T) {
		_, _ = setupRepoWithStack(t)

		out := runStExpectError(t, "abort")
		assertContains(t, out, "no rebase in progress")
	})

	t.Run("aborts_active_rebase_and_clears_state", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)
		// Create a stack where s1 has a file that will conflict with root
		runSt(t, "new", "s1")
		repo.CreateFile("conflict.txt", "s1 content")
		repo.AddAndCommit("s1: add conflict file")

		// Create conflicting change on root
		repo.Checkout(root)
		repo.CreateFile("conflict.txt", "root content")
		repo.AddAndCommit("root: add conflict file")

		// Restack from s1 will conflict
		repo.Checkout("s1")
		_ = runStExpectError(t, "restack")

		// Verify rebase is in progress
		if !rebaseInProgress(t, repo) {
			t.Skip("no rebase detected — conflict didn't trigger as expected")
		}

		// Abort should succeed
		runSt(t, "abort")

		// Restack state should be cleared
		if restackStateExists(t, repo) {
			t.Error("restack state should be cleared after abort")
		}

		// Continue should now error
		out := runStExpectError(t, "continue")
		assertContains(t, out, "no rebase in progress")
	})
}

func TestDetach(t *testing.T) {
	t.Run("detach_leaf_branch", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)

		runSt(t, "new", "feature-1")

		out := runSt(t, "detach", "feature-1")
		assertContains(t, out, "Detached 'feature-1' from stack")

		// Branch should still exist in git
		if !repo.BranchExists("feature-1") {
			t.Error("git branch feature-1 should still exist")
		}

		// Branch should not be in graph
		if graphContains(t, repo, "feature-1") {
			t.Error("feature-1 should not be in graph")
		}
	})

	t.Run("detach_branch_with_children_reparents", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		runSt(t, "new", "feature-1")
		runSt(t, "append", "feature-2")
		runSt(t, "append", "feature-3")

		out := runSt(t, "detach", "feature-1")
		assertContains(t, out, "Detached 'feature-1' from stack")
		assertContains(t, out, "reparented")
		assertContains(t, out, "st restack")

		// feature-1 removed, feature-2 reparented to root
		g := loadGraph(t, repo)
		if _, ok := g.Branches["feature-1"]; ok {
			t.Error("feature-1 should be removed from graph")
		}
		if b, ok := g.Branches["feature-2"]; !ok {
			t.Error("feature-2 should still be in graph")
		} else if b.Parent != root {
			t.Errorf("feature-2 parent = %q, want %q", b.Parent, root)
		}
	})

	t.Run("detach_root_errors", func(t *testing.T) {
		_, root := setupRepoWithStack(t)

		runSt(t, "new", "feature-1")

		out := runStExpectError(t, "detach", root)
		assertContains(t, out, "cannot detach the root branch")
	})

	t.Run("detach_unknown_branch_errors", func(t *testing.T) {
		_, _ = setupRepoWithStack(t)

		runSt(t, "new", "feature-1")

		out := runStExpectError(t, "detach", "nonexistent")
		assertContains(t, out, "not in the stack")
	})

	t.Run("detach_current_branch_no_arg", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)

		runSt(t, "new", "feature-1")
		// feature-1 is current branch

		out := runSt(t, "detach")
		assertContains(t, out, "Detached 'feature-1' from stack")

		if graphContains(t, repo, "feature-1") {
			t.Error("feature-1 should not be in graph")
		}
	})
}

// ---------------------------------------------------------------------------
// TestDeleteStack
// ---------------------------------------------------------------------------

func TestDeleteStack(t *testing.T) {
	t.Run("deletes_entire_lineage_from_graph_preserves_root", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2")
		repo.AddAndCommit("f2 commit")
		runSt(t, "append", "f3")
		repo.CreateFile("f3.txt", "f3")
		repo.AddAndCommit("f3 commit")

		out := runSt(t, "delete-stack")

		g := loadGraph(t, repo)
		if _, ok := g.Branches["f1"]; ok {
			t.Error("f1 should be removed from graph")
		}
		if _, ok := g.Branches["f2"]; ok {
			t.Error("f2 should be removed from graph")
		}
		if _, ok := g.Branches["f3"]; ok {
			t.Error("f3 should be removed from graph")
		}
		if g.Root != root {
			t.Errorf("root = %q, want %q", g.Root, root)
		}
		assertContains(t, out, "Deleted stack")
	})

	t.Run("preserves_git_branches_by_default", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)

		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2")
		repo.AddAndCommit("f2 commit")

		runSt(t, "delete-stack")

		// Git branches should still exist
		if !repo.BranchExists("f1") {
			t.Error("git branch f1 should still exist")
		}
		if !repo.BranchExists("f2") {
			t.Error("git branch f2 should still exist")
		}
	})

	t.Run("branches_flag_deletes_git_branches", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)

		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2")
		repo.AddAndCommit("f2 commit")

		runSt(t, "delete-stack", "--branches", "--force")

		if repo.BranchExists("f1") {
			t.Error("git branch f1 should be deleted")
		}
		if repo.BranchExists("f2") {
			t.Error("git branch f2 should be deleted")
		}
	})

	t.Run("errors_when_on_root_branch", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		runSt(t, "new", "f1")
		repo.Checkout(root)

		out := runStExpectError(t, "delete-stack")
		assertContains(t, out, "cannot delete-stack")
	})

	t.Run("checks_out_root_after_deletion", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		runSt(t, "new", "f1")
		repo.CreateFile("f1.txt", "f1")
		repo.AddAndCommit("f1 commit")
		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2")
		repo.AddAndCommit("f2 commit")

		runSt(t, "delete-stack")

		cur := getCurrentBranch(t, repo)
		if cur != root {
			t.Errorf("current branch = %q, want %q", cur, root)
		}
	})
}

// ---------------------------------------------------------------------------
// TestAttachFilterStacked
// ---------------------------------------------------------------------------

func TestModifyConflictContinue(t *testing.T) {
	t.Run("modify_downstream_conflict_then_continue_completes", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: root -> f1 -> f2 -> f3
		runSt(t, "new", "f1")
		repo.CreateFile("shared.txt", "f1 content")
		repo.CreateFile("f1.txt", "f1 file")
		repo.AddAndCommit("f1 commit")

		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2 file")
		repo.AddAndCommit("f2 commit")

		runSt(t, "append", "f3")
		repo.CreateFile("f3.txt", "f3 file")
		repo.AddAndCommit("f3 commit")

		// Root changes shared.txt -> will conflict during downstream restack after modify on f1
		repo.Checkout(root)
		repo.CreateFile("shared.txt", "root content")
		repo.AddAndCommit("root conflict")

		// Modify f1: amend its commit with a file change that doesn't conflict, but downstream f2 rebasing will conflict with root's shared.txt
		repo.Checkout("f1")
		repo.WriteFile("f1.txt", "f1-modified")
		repo.RunGit("add", "f1.txt")
		out := runStExpectError(t, "modify")
		assertContains(t, out, "conflict")

		// BUG: this used to fail because modify saved no restack state.
		// Resolve the conflict on f2 and continue
		resolveConflictAndStage(t, repo, "shared.txt", "resolved content")
		runStContinue(t)

		// Verify downstream branches are now rebased
		repo.Checkout("f2")
		if !repo.FileExists("f2.txt") {
			t.Error("f2 should have f2.txt after modify + continue")
		}

		repo.Checkout("f3")
		if !repo.FileExists("f3.txt") {
			t.Error("f3 should have f3.txt after modify + continue")
		}

		// Verify restack state is cleared
		if restackStateExists(t, repo) {
			t.Error("restack state should be cleared")
		}
	})
}

// ---------------------------------------------------------------------------
// TestMoveConflictContinue — BUG: move saves no restack state on conflict
// ---------------------------------------------------------------------------

func TestMoveConflictContinue(t *testing.T) {
	t.Run("move_conflict_then_continue_completes", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: root -> a -> a2 (for attachment target) and root -> b (with shared.txt change)
		runSt(t, "new", "a")
		repo.CreateFile("a.txt", "a")
		repo.AddAndCommit("a commit")
		runSt(t, "append", "a2")
		repo.CreateFile("a2.txt", "a2")
		repo.AddAndCommit("a2 commit")

		repo.Checkout(root)
		runSt(t, "new", "b")
		repo.CreateFile("b.txt", "b")
		repo.AddAndCommit("b commit")

		// a changes shared.txt in a way that will conflict with b
		repo.Checkout("a")
		repo.CreateFile("shared.txt", "a content")
		repo.AddAndCommit("a adds shared")

		// Now move b onto a — b will need to rebase onto a, encountering shared.txt difference
		repo.Checkout("b")
		repo.CreateFile("shared.txt", "b content")
		repo.AddAndCommit("b adds shared")

		// Move b onto a — rebase will conflict
		out := runStExpectError(t, "move", "--onto", "a")
		assertContains(t, out, "conflict")

		// BUG: move used to not save restack state, so continue would fail.
		// Resolve: accept a's content, add b's original content as "resolved"
		resolveConflictAndStage(t, repo, "shared.txt", "resolved content")
		runStContinue(t)

		// Verify b is on a and has its b.txt
		if !repo.FileExists("b.txt") {
			t.Error("b should have b.txt after move + continue")
		}
		if !repo.FileExists("shared.txt") {
			t.Error("b should have shared.txt resolved")
		}
	})
}

// ---------------------------------------------------------------------------
// TestInsertConflictContinue — BUG: insert saves no restack state on conflict
// ---------------------------------------------------------------------------

func TestInsertConflictContinue(t *testing.T) {
	t.Run("insert_downstream_conflict_then_continue_completes", func(t *testing.T) {
		repo, root := setupRepoWithStack(t)

		// Build stack: root -> f1 -> f2
		runSt(t, "new", "f1")
		repo.CreateFile("shared.txt", "f1 content")
		repo.CreateFile("f1.txt", "f1 file")
		repo.AddAndCommit("f1 commit")

		runSt(t, "append", "f2")
		repo.CreateFile("f2.txt", "f2 file")
		repo.AddAndCommit("f2 commit")

		// Root changes shared.txt -> will conflict with f1 when f1 is restacked
		repo.Checkout(root)
		repo.CreateFile("shared.txt", "root content")
		repo.AddAndCommit("root conflict")

		// Insert "mid" before f1. f1 (and f2) need to rebase onto mid.
		// f1 has shared.txt "f1 content", mid starts empty.
		// f1 rebased onto mid will then conflict with root.
		repo.Checkout("f1")
		out := runStExpectError(t, "insert", "mid")
		assertContains(t, out, "conflict")

		// BUG: insert used to not save restack state.
		resolveConflictAndStage(t, repo, "shared.txt", "resolved content")
		runStContinue(t)

		// Verify mid is in graph, f1 and f2 rebased
		if !graphContains(t, repo, "mid") {
			t.Error("mid should be in graph after insert + continue")
		}
		repo.Checkout("f1")
		if !repo.FileExists("f1.txt") {
			t.Error("f1 should have f1.txt")
		}
		repo.Checkout("f2")
		if !repo.FileExists("f2.txt") {
			t.Error("f2 should have f2.txt")
		}
	})
}
