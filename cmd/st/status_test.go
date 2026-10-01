package main

import (
	"testing"
)

// ---------------------------------------------------------------------------
// TestStatus
// ---------------------------------------------------------------------------

func TestStatus(t *testing.T) {
	t.Run("errors_when_no_remote", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")
		out := runStExpectError(t, "status")
		assertContains(t, out, "failed to get remote URL")
	})

	t.Run("errors_for_unsupported_forge", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.RunGit("remote", "add", "origin", "https://gitlab.com/user/repo.git")
		out := runStExpectError(t, "status")
		assertContains(t, out, "forge not supported")
	})
}
