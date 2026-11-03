package main

import (
	"fmt"
	"os"
	"path/filepath"
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

// fakeGH puts a gh script first on PATH that answers 'gh pr list --head
// <branch> ... --json ...' with the JSON in prs[branch], or [] for a branch
// not in prs, so pull request commands can be tested without the network.
func fakeGH(t *testing.T, prs map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for branch, js := range prs {
		if err := os.WriteFile(filepath.Join(dir, branch+".json"), []byte(js), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	script := `#!/bin/sh
while [ $# -gt 0 ]; do
  if [ "$1" = "--head" ]; then branch="$2"; fi
  shift
done
if [ -f "` + dir + `/$branch.json" ]; then cat "` + dir + `/$branch.json"; else echo '[]'; fi
`
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// ghPR is the JSON gh prints for a branch with one pull request.
func ghPR(number int, branch, state string) string {
	return fmt.Sprintf(`[{"number":%d,"headRefName":%q,"title":"t","state":%q,"isDraft":false,"reviewDecision":"","url":"https://github.com/user/repo/pull/%d","statusCheckRollup":[]}]`,
		number, branch, state, number)
}

func TestPRList(t *testing.T) {
	t.Run("prints_each_branch_with_its_pr_link_or_no_pr", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		repo.RunGit("remote", "add", "origin", "https://github.com/user/repo.git")
		runSt(t, "new", "f1")
		runSt(t, "append", "f2")
		fakeGH(t, map[string]string{"f1": ghPR(12, "f1", "OPEN")})

		out := runSt(t, "pr", "list")
		assertContains(t, out, "f1  https://github.com/user/repo/pull/12\n")
		assertContains(t, out, "f2  No PR\n")
	})

	t.Run("marks_merged_and_closed_prs_with_their_state", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		repo.RunGit("remote", "add", "origin", "https://github.com/user/repo.git")
		runSt(t, "new", "f1")
		runSt(t, "append", "f2")
		fakeGH(t, map[string]string{
			"f1": ghPR(12, "f1", "MERGED"),
			"f2": ghPR(13, "f2", "CLOSED"),
		})

		out := runSt(t, "pr", "list")
		assertContains(t, out, "f1  https://github.com/user/repo/pull/12 (merged)\n")
		assertContains(t, out, "f2  https://github.com/user/repo/pull/13 (closed)\n")
	})

	t.Run("lists_the_current_lineage_bottom_up_in_aligned_columns", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		repo.RunGit("remote", "add", "origin", "https://github.com/user/repo.git")
		runSt(t, "new", "other")
		runSt(t, "new", "a")
		runSt(t, "append", "a-long")
		fakeGH(t, map[string]string{"a-long": ghPR(7, "a-long", "OPEN")})

		out := runSt(t, "pr", "list")
		want := "a       No PR\n" +
			"a-long  https://github.com/user/repo/pull/7\n"
		if out != want {
			t.Errorf("expected:\n%s\ngot:\n%s", want, out)
		}
	})

	t.Run("errors_when_branch_not_in_stack", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		repo.RunGit("remote", "add", "origin", "https://github.com/user/repo.git")
		runSt(t, "new", "f1")
		repo.CreateBranch("untracked")
		fakeGH(t, nil)

		out := runStExpectError(t, "pr", "list")
		assertContains(t, out, "not in the stack")
	})

	t.Run("says_so_when_the_stack_has_no_branches", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		repo.RunGit("remote", "add", "origin", "https://github.com/user/repo.git")
		fakeGH(t, nil)

		out := runSt(t, "pr", "list")
		assertContains(t, out, "no branches in the stack")
	})
}
