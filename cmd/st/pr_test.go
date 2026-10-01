package main

import (
	"testing"
)

// ---------------------------------------------------------------------------
// TestPR
// ---------------------------------------------------------------------------

func TestPR(t *testing.T) {
	t.Run("make_errors_when_branch_not_in_stack", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		repo.RunGit("remote", "add", "origin", "https://github.com/user/repo.git")
		repo.CreateBranch("untracked")
		out := runStExpectError(t, "pr", "make")
		assertContains(t, out, "not in the stack")
	})

	t.Run("make_accepts_title_and_description_flags", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		repo.RunGit("remote", "add", "origin", "https://github.com/user/repo.git")
		repo.CreateBranch("untracked")
		// Should still error for branch not in stack, proving flags parse
		out := runStExpectError(t, "pr", "make", "--title", "My PR", "--description", "Details here")
		assertContains(t, out, "not in the stack")
	})

	t.Run("make_accepts_short_flags", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		repo.RunGit("remote", "add", "origin", "https://github.com/user/repo.git")
		repo.CreateBranch("untracked")
		out := runStExpectError(t, "pr", "make", "-t", "Short Title", "-d", "Short desc")
		assertContains(t, out, "not in the stack")
	})

	t.Run("make_errors_when_no_remote", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")
		out := runStExpectError(t, "pr", "make")
		assertContains(t, out, "failed to get remote URL")
	})

	t.Run("make_errors_for_unsupported_forge", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.RunGit("remote", "add", "origin", "https://gitlab.com/user/repo.git")
		out := runStExpectError(t, "pr", "make")
		assertContains(t, out, "forge not supported")
	})

	t.Run("view_errors_when_no_remote", func(t *testing.T) {
		setupRepoWithStack(t)
		out := runStExpectError(t, "pr", "view")
		assertContains(t, out, "failed to get remote URL")
	})

	t.Run("view_errors_for_unsupported_forge", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		repo.RunGit("remote", "add", "origin", "https://gitlab.com/user/repo.git")
		out := runStExpectError(t, "pr", "view")
		assertContains(t, out, "forge not supported")
	})
}
