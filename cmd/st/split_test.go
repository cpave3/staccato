package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cpave3/staccato/internal/testutil"
	"github.com/cpave3/staccato/pkg/split"
)

// setupSplitFixture builds a tracked branch "feature" on top of root with three
// commits, so a split has commit, file and partial-file boundaries to cut on:
//
//	core: add types   core/types.txt (new), README.md (+core)
//	api: add handler  api/handler.txt (new), README.md (+api)
//	ui: add page      ui/page.txt (new), legacy.txt (deleted)
func setupSplitFixture(t *testing.T) (*testutil.GitRepo, string) {
	t.Helper()
	repo, root := setupRepoWithStack(t)
	repo.WriteFile("legacy.txt", "legacy\n")
	repo.AddAndCommit("add legacy")

	runSt(t, "new", "feature")

	repo.WriteFile("core/types.txt", "types\n")
	repo.WriteFile("README.md", "# Test Repo\ncore\n")
	repo.AddAndCommit("core: add types")

	repo.WriteFile("api/handler.txt", "handler\n")
	repo.WriteFile("README.md", "# Test Repo\ncore\napi\n")
	repo.AddAndCommit("api: add handler")

	repo.WriteFile("ui/page.txt", "page\n")
	if _, err := repo.RunGit("rm", "-q", "legacy.txt"); err != nil {
		t.Fatalf("git rm: %v", err)
	}
	repo.AddAndCommit("ui: add page")

	return repo, root
}

func describeSplit(t *testing.T, args ...string) split.Description {
	t.Helper()
	out := runSt(t, append([]string{"split", "describe"}, args...)...)
	var d split.Description
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("describe output is not JSON: %v\n%s", err, out)
	}
	return d
}

// applySplit writes plan to a file outside the repo and runs `st split apply` on it.
func applySplit(t *testing.T, plan string, args ...string) string {
	t.Helper()
	return runSt(t, append([]string{"split", "apply", writePlan(t, plan)}, args...)...)
}

func applySplitExpectError(t *testing.T, plan string, args ...string) string {
	t.Helper()
	return runStExpectError(t, append([]string{"split", "apply", writePlan(t, plan)}, args...)...)
}

func writePlan(t *testing.T, plan string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(path, []byte(plan), 0644); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	return path
}

func gitOut(t *testing.T, repo *testutil.GitRepo, args ...string) string {
	t.Helper()
	out, err := repo.RunGit(args...)
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return out
}

// changedFiles lists the paths that differ between two revisions.
func changedFiles(t *testing.T, repo *testutil.GitRepo, from, to string) []string {
	t.Helper()
	out := gitOut(t, repo, "diff", "--no-renames", "--name-only", from, to)
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

func assertChangedFiles(t *testing.T, repo *testutil.GitRepo, from, to string, want ...string) {
	t.Helper()
	got := changedFiles(t, repo, from, to)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("files changed %s..%s = %v, want %v", from, to, got, want)
	}
}

// assertStackChain checks that branches form a parent-to-child chain above
// root in both the graph and the commit history.
func assertStackChain(t *testing.T, repo *testutil.GitRepo, root string, branches ...string) {
	t.Helper()
	g := loadGraph(t, repo)
	parent := root
	for _, name := range branches {
		b, ok := g.Branches[name]
		if !ok {
			t.Fatalf("%s not in graph", name)
		}
		if b.Parent != parent {
			t.Errorf("%s parent = %q, want %q", name, b.Parent, parent)
		}
		parentTip := gitOut(t, repo, "rev-parse", parent)
		if b.BaseSHA != parentTip {
			t.Errorf("%s base_sha = %q, want tip of %s %q", name, b.BaseSHA, parent, parentTip)
		}
		if _, err := repo.RunGit("merge-base", "--is-ancestor", parent, name); err != nil {
			t.Errorf("%s is not built on %s", name, parent)
		}
		parent = name
	}
}

func TestSplitApply(t *testing.T) {
	t.Run("splits_by_paths_into_stacked_branches", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()

		applySplit(t, `{
			"slices": [
				{"name": "feature-core", "message": "core: add types", "paths": ["core/"]},
				{"name": "feature-api", "message": "api: add handler", "paths": ["api/"]},
				{"name": "feature", "message": "ui: add page", "rest": true}
			]
		}`)

		assertStackChain(t, repo, root, "feature-core", "feature-api", "feature")
		assertChangedFiles(t, repo, root, "feature-core", "core/types.txt")
		assertChangedFiles(t, repo, "feature-core", "feature-api", "api/handler.txt")
		assertChangedFiles(t, repo, "feature-api", "feature", "README.md", "legacy.txt", "ui/page.txt")

		if got, want := gitOut(t, repo, "rev-parse", "feature^{tree}"), gitOut(t, repo, "rev-parse", original+"^{tree}"); got != want {
			t.Errorf("top slice tree = %s, want the original tree %s", got, want)
		}
		if msg := gitOut(t, repo, "log", "-1", "--format=%s", "feature-api"); msg != "api: add handler" {
			t.Errorf("feature-api commit message = %q", msg)
		}
		if cur := getCurrentBranch(t, repo); cur != "feature" {
			t.Errorf("current branch = %q, want feature", cur)
		}
		if status := gitOut(t, repo, "status", "--porcelain"); status != "" {
			t.Errorf("working tree not clean after split:\n%s", status)
		}
	})
}

const pathsPlan = `{
	"slices": [
		{"name": "feature-core", "message": "core: add types", "paths": ["core/"]},
		{"name": "feature-api", "message": "api: add handler", "paths": ["api/"]},
		{"name": "feature", "message": "ui: add page", "rest": true}
	]
}`

func TestSplitApplyDryRun(t *testing.T) {
	t.Run("reports_the_slices_and_changes_nothing", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()

		refsBefore := gitOut(t, repo, "for-each-ref")

		out := applySplit(t, pathsPlan, "--dry-run", "--json")

		if refsAfter := gitOut(t, repo, "for-each-ref"); refsAfter != refsBefore {
			t.Errorf("dry run changed refs:\nbefore:\n%s\nafter:\n%s", refsBefore, refsAfter)
		}
		var r split.Result
		if err := json.Unmarshal([]byte(out), &r); err != nil {
			t.Fatalf("output is not JSON: %v\n%s", err, out)
		}
		if !r.DryRun || r.Applied {
			t.Errorf("dry_run/applied = %v/%v, want true/false", r.DryRun, r.Applied)
		}
		if r.Branch != "feature" || r.Parent != root || len(r.Slices) != 3 {
			t.Fatalf("result = %+v", r)
		}
		core, top := r.Slices[0], r.Slices[2]
		if core.Name != "feature-core" || core.Parent != root || core.Commits != 1 || core.Files != 1 || core.Insertions != 1 {
			t.Errorf("first slice = %+v", core)
		}
		if top.Name != "feature" || top.Parent != "feature-api" || top.Files != 3 || top.Deletions != 2 {
			t.Errorf("top slice = %+v", top)
		}
		// The slice commits exist so they can be inspected before applying.
		gitOut(t, repo, "cat-file", "-e", core.SHA+"^{commit}")
		assertChangedFiles(t, repo, root, core.SHA, "core/types.txt")

		assertSplitNotApplied(t, repo, root, original, "feature-core", "feature-api")
	})

	t.Run("prints_a_summary_table", func(t *testing.T) {
		setupSplitFixture(t)

		out := applySplit(t, pathsPlan, "--dry-run")

		assertContains(t, out, "Dry run")
		assertContains(t, out, "feature-core")
		assertContains(t, out, "+1/-0")
		assertContains(t, out, "Nothing was changed")
	})
}

// addDescendants stacks child and grandchild on feature and returns to feature.
func addDescendants(t *testing.T, repo *testutil.GitRepo) {
	t.Helper()
	runSt(t, "append", "child")
	repo.WriteFile("child.txt", "child\n")
	repo.AddAndCommit("child work")
	runSt(t, "append", "grandchild")
	repo.WriteFile("grandchild.txt", "grandchild\n")
	repo.AddAndCommit("grandchild work")
	repo.Checkout("feature")
}

func TestSplitApplyDescendants(t *testing.T) {
	t.Run("follow_the_top_slice_and_keep_their_own_changes", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		addDescendants(t, repo)

		applySplit(t, pathsPlan)

		assertStackChain(t, repo, root, "feature-core", "feature-api", "feature", "child", "grandchild")
		assertChangedFiles(t, repo, "feature", "child", "child.txt")
		assertChangedFiles(t, repo, "child", "grandchild", "grandchild.txt")
		if got := gitOut(t, repo, "log", "--format=%s|%an", "feature..grandchild"); got != "grandchild work|Test User\nchild work|Test User" {
			t.Errorf("descendant commits = %q, want their messages and author kept", got)
		}
		if cur := getCurrentBranch(t, repo); cur != "feature" {
			t.Errorf("current branch = %q, want feature", cur)
		}
		if leftover := gitOut(t, repo, "branch", "--list", "backup/auto/*"); leftover != "" {
			t.Errorf("automatic backups left behind:\n%s", leftover)
		}
	})

	t.Run("move_without_a_checkout_when_split_from_a_descendant", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		addDescendants(t, repo)
		repo.Checkout("grandchild")
		repo.WriteFile("grandchild.txt", "grandchild\nwork in progress\n")

		applySplit(t, `{
			"branch": "feature",
			"slices": [
				{"name": "feature-core", "message": "core", "paths": ["core/"]},
				{"name": "feature", "message": "rest", "rest": true}
			]
		}`)

		assertStackChain(t, repo, root, "feature-core", "feature", "child", "grandchild")
		if cur := getCurrentBranch(t, repo); cur != "grandchild" {
			t.Errorf("current branch = %q, want grandchild", cur)
		}
		if status := gitOut(t, repo, "status", "--porcelain"); status != "M grandchild.txt" && status != " M grandchild.txt" {
			t.Errorf("uncommitted changes after split:\n%s", status)
		}
	})

	t.Run("skip_a_graph_entry_whose_branch_is_gone", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		addDescendants(t, repo)
		// The graph still lists grandchild, but its branch was deleted by hand.
		gitOut(t, repo, "branch", "-q", "-D", "grandchild")

		out := applySplit(t, pathsPlan)

		assertContains(t, out, "grandchild")
		assertStackChain(t, repo, root, "feature-core", "feature-api", "feature", "child")
	})

	t.Run("are_not_rewritten_when_the_top_commit_is_kept", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		commits := describeSplit(t).Commits
		addDescendants(t, repo)
		childBefore := gitOut(t, repo, "rev-parse", "child")

		applySplit(t, fmt.Sprintf(`{
			"slices": [
				{"name": "feature-core", "commits": [%q, %q]},
				{"name": "feature", "commits": [%q]}
			]
		}`, commits[0].SHA, commits[1].SHA, commits[2].SHA))

		assertStackChain(t, repo, root, "feature-core", "feature", "child", "grandchild")
		if got := gitOut(t, repo, "rev-parse", "child"); got != childBefore {
			t.Errorf("child was rewritten: %s, want %s", got, childBefore)
		}
		if backups := gitOut(t, repo, "branch", "--list", "backup/*"); backups != "" {
			t.Errorf("backups created although no commit was rewritten:\n%s", backups)
		}
	})
}

func TestSplitApplyExec(t *testing.T) {
	// present prints which of the fixture's files exist in the checked-out slice.
	const present = `echo "$ST_SPLIT_SLICE core=$(test -f core/types.txt && echo y || echo n) api=$(test -f api/handler.txt && echo y || echo n) ui=$(test -f ui/page.txt && echo y || echo n)"`

	t.Run("runs_the_command_on_each_slice_before_applying", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		log := filepath.Join(t.TempDir(), "seen.log")

		applySplit(t, pathsPlan, "--exec", present+" >> "+log)

		seen, err := os.ReadFile(log)
		if err != nil {
			t.Fatalf("read log: %v", err)
		}
		want := "feature-core core=y api=n ui=n\nfeature-api core=y api=y ui=n\nfeature core=y api=y ui=y\n"
		if string(seen) != want {
			t.Errorf("command saw:\n%s\nwant:\n%s", seen, want)
		}
		assertStackChain(t, repo, root, "feature-core", "feature-api", "feature")
		if cur := getCurrentBranch(t, repo); cur != "feature" {
			t.Errorf("current branch = %q, want feature", cur)
		}
		if status := gitOut(t, repo, "status", "--porcelain"); status != "" {
			t.Errorf("working tree not clean after split:\n%s", status)
		}
	})

	t.Run("applies_nothing_when_a_slice_fails", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()

		// The first slice has no api/ yet, so the command fails there.
		out := applySplitExpectError(t, pathsPlan, "--exec", `echo "checking $ST_SPLIT_SLICE"; test -f api/handler.txt`)

		assertContains(t, out, "checking feature-core")
		assertContains(t, out, `slice "feature-core" failed`)
		assertContains(t, out, "nothing was applied")
		assertSplitNotApplied(t, repo, root, original, "feature-core", "feature-api")
		if status := gitOut(t, repo, "status", "--porcelain"); status != "" {
			t.Errorf("working tree not restored after failed verification:\n%s", status)
		}
	})

	t.Run("verifies_without_applying_on_dry_run", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()
		log := filepath.Join(t.TempDir(), "seen.log")

		applySplit(t, pathsPlan, "--dry-run", "--exec", present+" >> "+log)

		if seen, _ := os.ReadFile(log); strings.Count(string(seen), "\n") != 3 {
			t.Errorf("command ran on %q, want all three slices", seen)
		}
		assertSplitNotApplied(t, repo, root, original, "feature-core", "feature-api")
	})

	t.Run("refuses_with_uncommitted_changes", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()
		repo.WriteFile("ui/page.txt", "page\nwork in progress\n")

		out := applySplitExpectError(t, pathsPlan, "--exec", "true")

		assertContains(t, out, "uncommitted changes")
		assertSplitNotApplied(t, repo, root, original, "feature-core", "feature-api")
	})

	t.Run("stashes_what_the_command_changed_and_applies_nothing", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()

		// A formatter or lockfile update would leave tracked files modified.
		out := applySplitExpectError(t, pathsPlan, "--exec", "echo generated >> core/types.txt")

		assertContains(t, out, "changed tracked files")
		assertContains(t, out, "core/types.txt")
		assertContains(t, out, "git stash")
		assertSplitNotApplied(t, repo, root, original, "feature-core", "feature-api")
		if status := gitOut(t, repo, "status", "--porcelain"); status != "" {
			t.Errorf("working tree not clean after failed verification:\n%s", status)
		}
		if stashes := gitOut(t, repo, "stash", "list"); strings.Count(stashes, "st split") != 1 {
			t.Errorf("stash list = %q, want the command's changes kept in one stash", stashes)
		}
	})

	t.Run("returns_to_the_branch_when_interrupted", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()

		cmd := exec.Command(stBinary, "split", "apply", writePlan(t, pathsPlan), "--exec", "sleep 30")
		setCoverEnv(cmd)
		if err := cmd.Start(); err != nil {
			t.Fatalf("start: %v", err)
		}
		// Wait until verification has checked out the first slice.
		deadline := time.Now().Add(10 * time.Second)
		for getCurrentBranch(t, repo) != "HEAD" {
			if time.Now().After(deadline) {
				cmd.Process.Kill()
				t.Fatal("verification never checked out a slice")
			}
			time.Sleep(20 * time.Millisecond)
		}
		started := time.Now()
		cmd.Process.Signal(os.Interrupt)
		if err := cmd.Wait(); err == nil {
			t.Error("interrupted split exited successfully")
		}
		if waited := time.Since(started); waited > 10*time.Second {
			t.Errorf("st took %s to stop after the interrupt", waited)
		}

		assertSplitNotApplied(t, repo, root, original, "feature-core", "feature-api")
	})
}

func TestSplitApplyHooksAndSummary(t *testing.T) {
	repo, _ := setupSplitFixture(t)
	addDescendants(t, repo)
	log := filepath.Join(t.TempDir(), "hooks.log")
	for _, event := range []string{"post-branch-create", "post-branch-delete"} {
		dir := filepath.Join(repo.Dir, ".staccato", "hooks", event)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		script := fmt.Sprintf("#!/bin/sh\necho \"$ST_EVENT $ST_BRANCH\" >> %s\n", log)
		if err := os.WriteFile(filepath.Join(dir, "log"), []byte(script), 0755); err != nil {
			t.Fatal(err)
		}
	}

	out := applySplit(t, `{
		"drop_original": true,
		"slices": [
			{"name": "feature-core", "message": "core", "paths": ["core/"]},
			{"name": "feature-rest", "message": "rest", "rest": true}
		]
	}`)

	fired, _ := os.ReadFile(log)
	want := "post-branch-create feature-core\npost-branch-create feature-rest\npost-branch-delete feature\n"
	if string(fired) != want {
		t.Errorf("hooks fired:\n%s\nwant:\n%s", fired, want)
	}

	assertContains(t, out, `Split "feature" into 2 branches`)
	assertContains(t, out, "Moved 2 descendant branch(es) onto the split: child, grandchild")
	assertContains(t, out, `Removed branch "feature"`)
	assertContains(t, out, "backup/manual/")
}

func TestSplitApplyOutOfDateDescendant(t *testing.T) {
	repo, root := setupSplitFixture(t)
	runSt(t, "append", "child")
	repo.WriteFile("core/types.txt", "types\nchild\n")
	repo.AddAndCommit("child edits types")
	childBefore := repo.HeadSHA()
	// feature moves on without child being restacked, in a way that conflicts.
	repo.Checkout("feature")
	repo.WriteFile("core/types.txt", "types\nfeature\n")
	repo.AddAndCommit("core: more types")

	out := applySplit(t, pathsPlan)

	// The split is not held up by a descendant that was already behind: it is
	// left exactly as it was and reported.
	assertContains(t, out, "child")
	assertContains(t, out, "st restack")
	assertStackChain(t, repo, root, "feature-core", "feature-api", "feature")
	if got := gitOut(t, repo, "rev-parse", "child"); got != childBefore {
		t.Errorf("out-of-date child was rewritten: %s, want %s", got, childBefore)
	}
	if g := loadGraph(t, repo); g.Branches["child"].Parent != "feature" {
		t.Errorf("child parent = %q, want feature", g.Branches["child"].Parent)
	}
	if rebaseInProgress(t, repo) {
		t.Error("split left a rebase in progress")
	}

	// It can still be restacked afterwards, with the conflict it always had.
	repo.Checkout("child")
	runStExpectError(t, "restack")
	resolveConflictAndStage(t, repo, "core/types.txt", "types\nfeature\nchild\n")
	runStContinue(t)
	assertStackChain(t, repo, root, "feature-core", "feature-api", "feature", "child")
	assertChangedFiles(t, repo, "feature", "child", "core/types.txt")
}

func TestSplitApplyUncommittedChanges(t *testing.T) {
	t.Run("are_kept_when_nothing_needs_restacking", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		repo.WriteFile("ui/page.txt", "page\nwork in progress\n")
		repo.WriteFile("scratch.txt", "untracked\n")

		applySplit(t, pathsPlan)

		assertStackChain(t, repo, root, "feature-core", "feature-api", "feature")
		if status := gitOut(t, repo, "status", "--porcelain"); status != "M ui/page.txt\n?? scratch.txt" && status != " M ui/page.txt\n?? scratch.txt" {
			t.Errorf("uncommitted changes after split:\n%s", status)
		}
	})

	t.Run("are_kept_when_descendants_move_too", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		addDescendants(t, repo)
		repo.WriteFile("ui/page.txt", "page\nwork in progress\n")

		applySplit(t, pathsPlan)

		assertStackChain(t, repo, root, "feature-core", "feature-api", "feature", "child", "grandchild")
		if status := gitOut(t, repo, "status", "--porcelain"); status != "M ui/page.txt" && status != " M ui/page.txt" {
			t.Errorf("uncommitted changes after split:\n%s", status)
		}
	})
}

func TestSplitApplyWorktrees(t *testing.T) {
	addWorktree := func(t *testing.T, repo *testutil.GitRepo, branch string) string {
		t.Helper()
		dir := filepath.Join(t.TempDir(), "wt")
		gitOut(t, repo, "worktree", "add", "--quiet", dir, branch)
		return dir
	}

	t.Run("runs_inside_a_linked_worktree", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		repo.Checkout(root)
		wt := addWorktree(t, repo, "feature")
		if err := os.Chdir(wt); err != nil {
			t.Fatal(err)
		}

		applySplit(t, pathsPlan)

		assertStackChain(t, repo, root, "feature-core", "feature-api", "feature")
		if cur := runGitInDir(t, wt, "rev-parse", "--abbrev-ref", "HEAD"); cur != "feature" {
			t.Errorf("worktree branch = %q, want feature", cur)
		}
		if status := runGitInDir(t, wt, "status", "--porcelain"); status != "" {
			t.Errorf("worktree not clean after split:\n%s", status)
		}
	})

	t.Run("splits_a_branch_checked_out_elsewhere_when_the_top_slice_keeps_its_name", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		repo.Checkout(root)
		wt := addWorktree(t, repo, "feature")

		applySplit(t, `{
			"branch": "feature",
			"slices": [
				{"name": "feature-core", "message": "core", "paths": ["core/"]},
				{"name": "feature", "message": "rest", "rest": true}
			]
		}`)

		assertStackChain(t, repo, root, "feature-core", "feature")
		if status := runGitInDir(t, wt, "status", "--porcelain"); status != "" {
			t.Errorf("other worktree not clean after split:\n%s", status)
		}
		if cur := getCurrentBranch(t, repo); cur != root {
			t.Errorf("current branch = %q, want %q", cur, root)
		}
	})

	t.Run("refuses_to_rename_a_branch_checked_out_elsewhere", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()
		repo.Checkout(root)
		addWorktree(t, repo, "feature")

		out := applySplitExpectError(t, `{
			"branch": "feature",
			"drop_original": true,
			"slices": [
				{"name": "feature-core", "message": "core", "paths": ["core/"]},
				{"name": "feature-rest", "message": "rest", "rest": true}
			]
		}`)

		assertContains(t, out, "checked out in another worktree")
		if got := gitOut(t, repo, "rev-parse", "feature"); got != original {
			t.Errorf("feature moved to %s", got)
		}
		if repo.BranchExists("feature-core") {
			t.Error("feature-core was created by a rejected plan")
		}
	})

	t.Run("moves_a_descendant_checked_out_elsewhere", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		addDescendants(t, repo)
		wt := addWorktree(t, repo, "child")

		applySplit(t, pathsPlan)

		assertStackChain(t, repo, root, "feature-core", "feature-api", "feature", "child", "grandchild")
		if status := runGitInDir(t, wt, "status", "--porcelain"); status != "" {
			t.Errorf("other worktree not clean after split:\n%s", status)
		}
		if got, want := runGitInDir(t, wt, "rev-parse", "HEAD"), gitOut(t, repo, "rev-parse", "child"); got != want {
			t.Errorf("other worktree HEAD = %s, want the moved child %s", got, want)
		}
	})
}

func TestSplitApplyBranchNames(t *testing.T) {
	t.Run("original_name_dropped_when_no_slice_keeps_it", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()
		addDescendants(t, repo)

		applySplit(t, `{
			"drop_original": true,
			"slices": [
				{"name": "feature-core", "message": "core", "paths": ["core/"]},
				{"name": "feature-rest", "message": "rest", "rest": true}
			]
		}`)

		assertStackChain(t, repo, root, "feature-core", "feature-rest", "child", "grandchild")
		if repo.BranchExists("feature") {
			t.Error("branch feature still exists")
		}
		if graphContains(t, repo, "feature") {
			t.Error("feature still in graph")
		}
		if cur := getCurrentBranch(t, repo); cur != "feature-rest" {
			t.Errorf("current branch = %q, want the top slice feature-rest", cur)
		}
		if status := gitOut(t, repo, "status", "--porcelain"); status != "" {
			t.Errorf("working tree not clean after split:\n%s", status)
		}
		// The original commits stay reachable through a manual backup.
		backup := gitOut(t, repo, "branch", "--list", "--format=%(refname:short)", "backup/manual/*/feature")
		if backup == "" || gitOut(t, repo, "rev-parse", backup) != original {
			t.Errorf("backup of the original branch = %q, want one at %s", backup, original)
		}
	})

	t.Run("original_name_can_go_to_a_lower_slice", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()

		applySplit(t, `{
			"slices": [
				{"name": "feature", "message": "core", "paths": ["core/"]},
				{"name": "feature-rest", "message": "rest", "rest": true}
			]
		}`)

		assertStackChain(t, repo, root, "feature", "feature-rest")
		assertChangedFiles(t, repo, root, "feature", "core/types.txt")
		if cur := getCurrentBranch(t, repo); cur != "feature-rest" {
			t.Errorf("current branch = %q, want the top slice feature-rest", cur)
		}
		if status := gitOut(t, repo, "status", "--porcelain"); status != "" {
			t.Errorf("working tree not clean after split:\n%s", status)
		}
		if got, want := gitOut(t, repo, "rev-parse", "feature-rest^{tree}"), gitOut(t, repo, "rev-parse", original+"^{tree}"); got != want {
			t.Errorf("top slice tree = %s, want the original tree %s", got, want)
		}
	})
}

func TestSplitApplyFromStdin(t *testing.T) {
	repo, root := setupSplitFixture(t)

	cmd := exec.Command(stBinary, "split", "apply", "-")
	cmd.Stdin = strings.NewReader(pathsPlan)
	setCoverEnv(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("st split apply - failed: %v\n%s", err, out)
	}

	assertStackChain(t, repo, root, "feature-core", "feature-api", "feature")
}

func TestSplitApplyOverrides(t *testing.T) {
	t.Run("gives_a_shared_file_intermediate_content", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()

		// The override source sits next to the plan and is named relative to it.
		plan := writePlan(t, `{
			"slices": [
				{"name": "feature-core", "message": "core", "paths": ["core/"],
				 "overrides": {"README.md": "README.core.md"}},
				{"name": "feature-api", "message": "api", "paths": ["api/", "README.md"]},
				{"name": "feature", "message": "ui", "rest": true}
			]
		}`)
		if err := os.WriteFile(filepath.Join(filepath.Dir(plan), "README.core.md"), []byte("# Test Repo\ncore\n"), 0644); err != nil {
			t.Fatal(err)
		}
		runSt(t, "split", "apply", plan)

		assertStackChain(t, repo, root, "feature-core", "feature-api", "feature")
		if got := gitOut(t, repo, "show", "feature-core:README.md"); got != "# Test Repo\ncore" {
			t.Errorf("README.md at feature-core = %q, want the override content", got)
		}
		if got := gitOut(t, repo, "show", "feature-api:README.md"); got != "# Test Repo\ncore\napi" {
			t.Errorf("README.md at feature-api = %q, want the final content", got)
		}
		assertChangedFiles(t, repo, root, "feature-core", "README.md", "core/types.txt")
		assertChangedFiles(t, repo, "feature-core", "feature-api", "README.md", "api/handler.txt")
		assertChangedFiles(t, repo, "feature-api", "feature", "legacy.txt", "ui/page.txt")
		if got, want := gitOut(t, repo, "rev-parse", "feature^{tree}"), gitOut(t, repo, "rev-parse", original+"^{tree}"); got != want {
			t.Errorf("top slice tree = %s, want the original tree %s", got, want)
		}
	})

	t.Run("rejects_override_that_is_never_brought_to_final_content", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()

		plan := writePlan(t, `{
			"slices": [
				{"name": "feature-core", "message": "core", "paths": ["core/", "api/", "ui/", "legacy.txt"]},
				{"name": "feature", "message": "docs", "overrides": {"README.md": "README.partial.md"}}
			]
		}`)
		if err := os.WriteFile(filepath.Join(filepath.Dir(plan), "README.partial.md"), []byte("# Test Repo\ncore\n"), 0644); err != nil {
			t.Fatal(err)
		}
		out := runStExpectError(t, "split", "apply", plan)

		assertContains(t, out, `slice "feature" overrides "README.md" but no later slice finishes it`)
		assertSplitNotApplied(t, repo, root, original, "feature-core")
	})

	t.Run("rejects_override_of_a_symlink", func(t *testing.T) {
		repo, _ := setupSplitFixture(t)
		if err := os.Symlink("core/types.txt", filepath.Join(repo.Dir, "link")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		repo.AddAndCommit("add link")
		original := repo.HeadSHA()

		plan := writePlan(t, `{
			"slices": [
				{"name": "feature-core", "message": "core", "paths": ["core/"],
				 "overrides": {"link": "content.txt"}},
				{"name": "feature", "message": "rest", "rest": true}
			]
		}`)
		if err := os.WriteFile(filepath.Join(filepath.Dir(plan), "content.txt"), []byte("x\n"), 0644); err != nil {
			t.Fatal(err)
		}
		out := runStExpectError(t, "split", "apply", plan)

		assertContains(t, out, `override "link"`)
		assertContains(t, out, "regular file")
		if got := gitOut(t, repo, "rev-parse", "feature"); got != original {
			t.Errorf("feature moved to %s", got)
		}
	})

	t.Run("rejects_override_of_a_file_the_branch_does_not_change", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()

		plan := writePlan(t, `{
			"slices": [
				{"name": "feature-core", "message": "core", "paths": ["core/"],
				 "overrides": {"unrelated.txt": "content.txt"}},
				{"name": "feature", "message": "rest", "rest": true}
			]
		}`)
		if err := os.WriteFile(filepath.Join(filepath.Dir(plan), "content.txt"), []byte("x\n"), 0644); err != nil {
			t.Fatal(err)
		}
		out := runStExpectError(t, "split", "apply", plan)

		assertContains(t, out, `"unrelated.txt"`)
		assertContains(t, out, "not changed on this branch")
		assertSplitNotApplied(t, repo, root, original, "feature-core")
	})

	t.Run("rejects_missing_override_source", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()

		out := applySplitExpectError(t, `{
			"slices": [
				{"name": "feature-core", "message": "core", "paths": ["core/"],
				 "overrides": {"README.md": "nope.md"}},
				{"name": "feature", "message": "rest", "rest": true}
			]
		}`)

		assertContains(t, out, "nope.md")
		assertSplitNotApplied(t, repo, root, original, "feature-core")
	})
}

func TestSplitApplyCommits(t *testing.T) {
	t.Run("cuts_at_commit_boundaries_without_rewriting", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		commits := describeSplit(t).Commits

		applySplit(t, fmt.Sprintf(`{
			"slices": [
				{"name": "feature-core", "commits": [%q]},
				{"name": "feature-api", "commits": [%q]},
				{"name": "feature", "commits": [%q]}
			]
		}`, commits[0].SHA, commits[1].SHA, commits[2].SHA))

		assertStackChain(t, repo, root, "feature-core", "feature-api", "feature")
		for i, name := range []string{"feature-core", "feature-api", "feature"} {
			if got := gitOut(t, repo, "rev-parse", name); !strings.HasPrefix(got, commits[i].SHA) {
				t.Errorf("%s = %s, want the existing commit %s", name, got, commits[i].SHA)
			}
		}
	})

	t.Run("reorders_commits_keeping_message_and_author", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()
		commits := describeSplit(t).Commits
		authored := gitOut(t, repo, "log", "-1", "--format=%aI", "feature")
		gitOut(t, repo, "config", "user.name", "Someone Else")

		// Short SHAs are accepted, and the ui commit moves below the other two.
		applySplit(t, fmt.Sprintf(`{
			"slices": [
				{"name": "feature-ui", "commits": [%q]},
				{"name": "feature", "commits": [%q, %q]}
			]
		}`, commits[2].SHA[:8], commits[0].SHA, commits[1].SHA))

		assertStackChain(t, repo, root, "feature-ui", "feature")
		assertChangedFiles(t, repo, root, "feature-ui", "legacy.txt", "ui/page.txt")
		if got := gitOut(t, repo, "log", "-1", "--format=%s|%an|%aI|%cn", "feature-ui"); got != "ui: add page|Test User|"+authored+"|Someone Else" {
			t.Errorf("feature-ui commit = %q, want the original message, author and author date, committed by the current user", got)
		}
		if got := gitOut(t, repo, "log", "--format=%s", "feature-ui..feature"); got != "api: add handler\ncore: add types" {
			t.Errorf("feature commits = %q", got)
		}
		if got, want := gitOut(t, repo, "rev-parse", "feature^{tree}"), gitOut(t, repo, "rev-parse", original+"^{tree}"); got != want {
			t.Errorf("top slice tree = %s, want the original tree %s", got, want)
		}
	})
}

func TestSplitApplyMixedSources(t *testing.T) {
	t.Run("leaves_out_a_commit_whose_changes_an_earlier_slice_took_by_path", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()
		commits := describeSplit(t).Commits

		out := applySplit(t, fmt.Sprintf(`{
			"slices": [
				{"name": "feature-ui", "message": "ui: add page", "paths": ["ui/", "legacy.txt"]},
				{"name": "feature", "commits": [%q, %q, %q]}
			]
		}`, commits[0].SHA, commits[1].SHA, commits[2].SHA), "--json")

		var r split.Result
		if err := json.Unmarshal([]byte(out), &r); err != nil {
			t.Fatalf("output is not JSON: %v\n%s", err, out)
		}
		if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0], "ui: add page") {
			t.Errorf("warnings = %v, want one about the ui commit", r.Warnings)
		}
		assertStackChain(t, repo, root, "feature-ui", "feature")
		if got := gitOut(t, repo, "log", "--format=%s", "feature-ui..feature"); got != "api: add handler\ncore: add types" {
			t.Errorf("feature commits = %q", got)
		}
		if got, want := gitOut(t, repo, "rev-parse", "feature^{tree}"), gitOut(t, repo, "rev-parse", original+"^{tree}"); got != want {
			t.Errorf("top slice tree = %s, want the original tree %s", got, want)
		}
	})
}

func TestSplitApplyPathspecs(t *testing.T) {
	repo, root := setupSplitFixture(t)

	// A glob, then pathspec magic that takes everything left outside ui/.
	applySplit(t, `{
		"slices": [
			{"name": "feature-types", "message": "types", "paths": ["*/types.txt"]},
			{"name": "feature-mid", "message": "everything but ui", "paths": [":(exclude)ui/"]},
			{"name": "feature", "message": "ui", "rest": true}
		]
	}`)

	assertStackChain(t, repo, root, "feature-types", "feature-mid", "feature")
	assertChangedFiles(t, repo, root, "feature-types", "core/types.txt")
	assertChangedFiles(t, repo, "feature-types", "feature-mid", "README.md", "api/handler.txt", "legacy.txt")
	assertChangedFiles(t, repo, "feature-mid", "feature", "ui/page.txt")
}

func TestSplitApplyUnusualPaths(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	runSt(t, "new", "feature")
	symlinked := os.Symlink("plain.txt", filepath.Join(repo.Dir, "link")) == nil
	repo.WriteFile(" lead.txt", "leading space\n")
	repo.WriteFile("sp ace/tab\there.txt", "tab\n")
	repo.WriteFile("plain.txt", "plain\n")
	if err := os.Chmod(filepath.Join(repo.Dir, "plain.txt"), 0755); err != nil {
		t.Fatal(err)
	}
	repo.AddAndCommit("odd names")

	applySplit(t, `{
		"slices": [
			{"name": "feature-odd", "message": "odd names", "paths": [" lead.txt", "sp ace/"]},
			{"name": "feature", "message": "plain", "rest": true}
		]
	}`)

	if symlinked {
		if entry := gitOut(t, repo, "ls-tree", "feature", "link"); !strings.HasPrefix(entry, "120000 ") {
			t.Errorf("link = %q, want it kept as a symlink", entry)
		}
	}

	assertStackChain(t, repo, root, "feature-odd", "feature")
	for _, path := range []string{" lead.txt", "sp ace/tab\there.txt"} {
		if _, err := repo.RunGit("cat-file", "-e", "feature-odd:"+path); err != nil {
			t.Errorf("%q is missing from feature-odd", path)
		}
	}
	if _, err := repo.RunGit("cat-file", "-e", "feature-odd:plain.txt"); err == nil {
		t.Error("plain.txt is in feature-odd, want it only in the top slice")
	}
	if mode := gitOut(t, repo, "ls-tree", "feature", "plain.txt"); !strings.HasPrefix(mode, "100755 ") {
		t.Errorf("plain.txt = %q, want its executable bit kept", mode)
	}
}

// assertSplitNotApplied checks that a rejected plan left the branch, the
// graph and the checkout exactly as they were.
func assertSplitNotApplied(t *testing.T, repo *testutil.GitRepo, root, original string, absent ...string) {
	t.Helper()
	if got := gitOut(t, repo, "rev-parse", "feature"); got != original {
		t.Errorf("feature moved to %s, want it left at %s", got, original)
	}
	g := loadGraph(t, repo)
	if b, ok := g.Branches["feature"]; !ok || b.Parent != root {
		t.Errorf("feature graph entry changed: %+v", b)
	}
	for _, name := range absent {
		if repo.BranchExists(name) {
			t.Errorf("branch %s was created by a rejected plan", name)
		}
		if _, ok := g.Branches[name]; ok {
			t.Errorf("%s was added to the graph by a rejected plan", name)
		}
	}
	if cur := getCurrentBranch(t, repo); cur != "feature" {
		t.Errorf("current branch = %q, want feature", cur)
	}
}

func TestSplitApplyRejects(t *testing.T) {
	t.Run("plan_that_leaves_changes_out", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()

		out := applySplitExpectError(t, `{
			"slices": [
				{"name": "feature-core", "message": "core", "paths": ["core/"]},
				{"name": "feature", "message": "api", "paths": ["api/"]}
			]
		}`)

		for _, missing := range []string{"README.md", "legacy.txt", "ui/page.txt"} {
			assertContains(t, out, missing)
		}
		assertContains(t, out, `"rest": true`)
		assertSplitNotApplied(t, repo, root, original, "feature-core")
	})

	t.Run("path_that_matches_no_changed_file", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()

		out := applySplitExpectError(t, `{
			"slices": [
				{"name": "feature-core", "message": "core", "paths": ["core/", "nope/"]},
				{"name": "feature", "message": "rest", "rest": true}
			]
		}`)

		assertContains(t, out, `"nope/"`)
		assertContains(t, out, "matches no file changed")
		assertSplitNotApplied(t, repo, root, original, "feature-core")
	})

	t.Run("path_already_placed_by_an_earlier_slice", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()

		// The agent expects core/types.txt in the second slice, but the first
		// slice's directory path already took it.
		out := applySplitExpectError(t, `{
			"slices": [
				{"name": "feature-core", "message": "core", "paths": ["core/"]},
				{"name": "feature-api", "message": "api", "paths": ["api/", "core/types.txt"]},
				{"name": "feature", "message": "rest", "rest": true}
			]
		}`)

		assertContains(t, out, `slice "feature-api": path "core/types.txt"`)
		assertContains(t, out, `already placed by slice "feature-core"`)
		assertSplitNotApplied(t, repo, root, original, "feature-core", "feature-api")
	})

	t.Run("original_name_dropped_without_saying_so", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()

		out := applySplitExpectError(t, `{
			"slices": [
				{"name": "feature-core", "message": "core", "paths": ["core/"]},
				{"name": "feature-rest", "message": "rest", "rest": true}
			]
		}`)

		assertContains(t, out, `no slice is named "feature"`)
		assertContains(t, out, `"drop_original": true`)
		assertSplitNotApplied(t, repo, root, original, "feature-core", "feature-rest")
	})

	t.Run("drop_original_set_while_a_slice_keeps_the_name", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()

		out := applySplitExpectError(t, `{
			"drop_original": true,
			"slices": [
				{"name": "feature-core", "message": "core", "paths": ["core/"]},
				{"name": "feature", "message": "rest", "rest": true}
			]
		}`)

		assertContains(t, out, `"drop_original"`)
		assertSplitNotApplied(t, repo, root, original, "feature-core")
	})

	t.Run("slice_name_already_taken", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()
		gitOut(t, repo, "branch", "taken", root)

		out := applySplitExpectError(t, `{
			"slices": [
				{"name": "taken", "message": "core", "paths": ["core/"]},
				{"name": "feature", "message": "rest", "rest": true}
			]
		}`)

		assertContains(t, out, `branch "taken" already exists`)
		assertSplitNotApplied(t, repo, root, original)
		if got := gitOut(t, repo, "rev-parse", "taken"); got != gitOut(t, repo, "rev-parse", root) {
			t.Errorf("existing branch taken was moved to %s", got)
		}
	})

	t.Run("invalid_slice_name", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()

		out := applySplitExpectError(t, `{
			"slices": [
				{"name": "bad..name", "message": "core", "paths": ["core/"]},
				{"name": "feature", "message": "rest", "rest": true}
			]
		}`)

		assertContains(t, out, `"bad..name" is not a valid branch name`)
		assertSplitNotApplied(t, repo, root, original)
	})

	t.Run("slice_name_nested_under_a_branch_that_stays", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()

		out := applySplitExpectError(t, `{
			"slices": [
				{"name": "feature/core", "message": "core", "paths": ["core/"]},
				{"name": "feature", "message": "rest", "rest": true}
			]
		}`)

		assertContains(t, out, `"feature/core" cannot exist alongside branch "feature"`)
		assertSplitNotApplied(t, repo, root, original)
	})

	t.Run("branch_not_in_the_stack", func(t *testing.T) {
		repo, _ := setupSplitFixture(t)
		repo.CreateBranch("loose")

		out := applySplitExpectError(t, `{
			"slices": [
				{"name": "loose-core", "message": "core", "paths": ["core/"]},
				{"name": "loose", "message": "rest", "rest": true}
			]
		}`)

		assertContains(t, out, `branch "loose" is not in the stack`)
	})

	t.Run("root_branch", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()

		out := applySplitExpectError(t, fmt.Sprintf(`{
			"branch": %q,
			"slices": [
				{"name": "a", "message": "core", "paths": ["core/"]},
				{"name": "b", "message": "rest", "rest": true}
			]
		}`, root))

		assertContains(t, out, "cannot split the root branch")
		assertSplitNotApplied(t, repo, root, original, "a", "b")
	})

	t.Run("branch_behind_its_parent", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()
		repo.Checkout(root)
		repo.WriteFile("trunk.txt", "trunk\n")
		repo.AddAndCommit("trunk moves on")
		repo.Checkout("feature")

		out := applySplitExpectError(t, `{
			"slices": [
				{"name": "feature-core", "message": "core", "paths": ["core/"]},
				{"name": "feature", "message": "rest", "rest": true}
			]
		}`)

		assertContains(t, out, "st restack")
		assertSplitNotApplied(t, repo, root, original, "feature-core")
	})

	t.Run("branch_moved_since_the_plan_was_written", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		head := describeSplit(t).Head
		repo.WriteFile("late.txt", "late\n")
		repo.AddAndCommit("late addition")
		original := repo.HeadSHA()

		out := applySplitExpectError(t, fmt.Sprintf(`{
			"head": %q,
			"slices": [
				{"name": "feature-core", "message": "core", "paths": ["core/"]},
				{"name": "feature", "message": "rest", "rest": true}
			]
		}`, head))

		assertContains(t, out, "has moved")
		assertContains(t, out, "st split describe")
		assertSplitNotApplied(t, repo, root, original, "feature-core")
	})

	t.Run("commit_placed_before_the_commit_it_builds_on", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()
		commits := describeSplit(t).Commits

		// The api commit edits a README line that the core commit introduces.
		out := applySplitExpectError(t, fmt.Sprintf(`{
			"slices": [
				{"name": "feature-api", "commits": [%q]},
				{"name": "feature", "commits": [%q, %q]}
			]
		}`, commits[1].SHA, commits[0].SHA, commits[2].SHA))

		assertContains(t, out, "api: add handler")
		assertContains(t, out, "README.md")
		assertSplitNotApplied(t, repo, root, original, "feature-api")
	})

	t.Run("rest_slice_with_nothing_left", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()

		out := applySplitExpectError(t, `{
			"slices": [
				{"name": "feature-all", "message": "everything", "paths": ["core/", "api/", "ui/", "legacy.txt", "README.md"]},
				{"name": "feature", "message": "rest", "rest": true}
			]
		}`)

		assertContains(t, out, `slice "feature" is empty`)
		assertSplitNotApplied(t, repo, root, original, "feature-all")
	})

	t.Run("merge_commit_placed_by_commit", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		gitOut(t, repo, "checkout", "-q", "-b", "side", "feature~1")
		repo.WriteFile("side.txt", "side\n")
		repo.AddAndCommit("side work")
		repo.Checkout("feature")
		gitOut(t, repo, "merge", "-q", "--no-ff", "-m", "merge side", "side")
		gitOut(t, repo, "branch", "-q", "-D", "side")
		original := repo.HeadSHA()

		d := describeSplit(t)
		var merge string
		for _, c := range d.Commits {
			if c.Merge {
				merge = c.SHA
			}
		}
		if merge == "" {
			t.Fatalf("describe did not flag the merge commit: %+v", d.Commits)
		}

		out := applySplitExpectError(t, fmt.Sprintf(`{
			"slices": [
				{"name": "feature-core", "commits": [%q]},
				{"name": "feature", "commits": [%q]}
			]
		}`, d.Commits[0].SHA, merge))

		assertContains(t, out, "merge commit")
		assertSplitNotApplied(t, repo, root, original, "feature-core")

		// The same branch still splits by paths.
		applySplit(t, pathsPlan)
		assertStackChain(t, repo, root, "feature-core", "feature-api", "feature")
	})

	t.Run("commit_from_outside_the_branch", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()
		commits := describeSplit(t).Commits

		out := applySplitExpectError(t, fmt.Sprintf(`{
			"slices": [
				{"name": "feature-core", "commits": [%q]},
				{"name": "feature", "commits": [%q]}
			]
		}`, root, commits[0].SHA))

		assertContains(t, out, "not one of this branch's commits")
		assertSplitNotApplied(t, repo, root, original, "feature-core")
	})

	t.Run("commit_listed_twice", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()
		commits := describeSplit(t).Commits

		out := applySplitExpectError(t, fmt.Sprintf(`{
			"slices": [
				{"name": "feature-core", "commits": [%q]},
				{"name": "feature", "commits": [%q, %q, %q]}
			]
		}`, commits[0].SHA, commits[0].SHA, commits[1].SHA, commits[2].SHA))

		assertContains(t, out, "listed twice")
		assertSplitNotApplied(t, repo, root, original, "feature-core")
	})

	t.Run("commits_left_out", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		original := repo.HeadSHA()
		commits := describeSplit(t).Commits

		out := applySplitExpectError(t, fmt.Sprintf(`{
			"slices": [
				{"name": "feature-core", "commits": [%q]},
				{"name": "feature", "commits": [%q]}
			]
		}`, commits[0].SHA, commits[1].SHA))

		assertContains(t, out, "ui/page.txt")
		assertContains(t, out, "not assigned to a slice")
		assertContains(t, out, "ui: add page")
		assertSplitNotApplied(t, repo, root, original, "feature-core")
	})
}

func TestSplitDescribe(t *testing.T) {
	t.Run("reports_commits_and_files_of_current_branch", func(t *testing.T) {
		repo, root := setupSplitFixture(t)

		d := describeSplit(t)

		if d.Branch != "feature" || d.Parent != root {
			t.Errorf("branch/parent = %q/%q, want feature/%q", d.Branch, d.Parent, root)
		}
		if head := repo.HeadSHA(); d.Head != head {
			t.Errorf("head = %q, want %q", d.Head, head)
		}

		var subjects []string
		for _, c := range d.Commits {
			subjects = append(subjects, c.Subject)
		}
		want := []string{"core: add types", "api: add handler", "ui: add page"}
		if len(subjects) != len(want) {
			t.Fatalf("commits = %v, want %v", subjects, want)
		}
		for i := range want {
			if subjects[i] != want[i] {
				t.Errorf("commit %d = %q, want %q", i, subjects[i], want[i])
			}
		}

		files := map[string]split.FileInfo{}
		for _, f := range d.Files {
			files[f.Path] = f
		}
		if len(files) != 5 {
			t.Errorf("files = %d, want 5: %+v", len(files), d.Files)
		}
		if f := files["core/types.txt"]; f.Status != "added" || f.Insertions != 1 {
			t.Errorf("core/types.txt = %+v, want added +1", f)
		}
		if f := files["legacy.txt"]; f.Status != "deleted" || f.Deletions != 1 {
			t.Errorf("legacy.txt = %+v, want deleted -1", f)
		}
		readme := files["README.md"]
		if readme.Status != "modified" {
			t.Errorf("README.md status = %q, want modified", readme.Status)
		}
		if len(readme.Commits) != 2 || readme.Commits[0] != d.Commits[0].SHA || readme.Commits[1] != d.Commits[1].SHA {
			t.Errorf("README.md commits = %v, want the first two commits", readme.Commits)
		}

		// README.md counts +3/-1: the root's first line gains its trailing newline.
		if d.Totals.Files != 5 || d.Totals.Insertions != 6 || d.Totals.Deletions != 2 {
			t.Errorf("totals = %+v, want 5 files +6/-2", d.Totals)
		}
		if len(d.Commits[0].SHA) != 12 || !strings.HasPrefix(gitOut(t, repo, "rev-parse", "feature~2"), d.Commits[0].SHA) {
			t.Errorf("first commit sha = %q, want a 12-character abbreviation", d.Commits[0].SHA)
		}
		if want, _ := filepath.EvalSymlinks(repo.Dir); d.Worktree != want {
			t.Errorf("worktree = %q, want %q", d.Worktree, want)
		}
	})

	t.Run("reports_rework_and_renames", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		repo.WriteFile("legacy.txt", "one\ntwo\nthree\n")
		repo.AddAndCommit("add legacy")
		runSt(t, "new", "feature")
		gitOut(t, repo, "mv", "legacy.txt", "archive.txt")
		repo.AddAndCommit("move legacy")
		repo.WriteFile("notes.txt", "draft\n")
		repo.AddAndCommit("draft notes")
		repo.WriteFile("notes.txt", "final\n")
		repo.AddAndCommit("rewrite notes")

		d := describeSplit(t)

		files := map[string]split.FileInfo{}
		for _, f := range d.Files {
			files[f.Path] = f
		}
		// A rename is two placeable paths, linked so they can be kept together.
		if f := files["archive.txt"]; f.Status != "added" || f.RenamedFrom != "legacy.txt" {
			t.Errorf("archive.txt = %+v, want added and renamed from legacy.txt", f)
		}
		if f := files["legacy.txt"]; f.Status != "deleted" {
			t.Errorf("legacy.txt = %+v, want deleted", f)
		}
		// The net change is +4/-3, but the commits wrote +5/-4 to get there:
		// notes.txt was written and then rewritten.
		if d.Totals.Insertions != 4 || d.Totals.Deletions != 3 {
			t.Errorf("totals = %+v, want +4/-3", d.Totals)
		}
		if d.Churn.Insertions != 5 || d.Churn.Deletions != 4 {
			t.Errorf("churn = %+v, want +5/-4", d.Churn)
		}
	})

	t.Run("describes_a_named_branch_from_elsewhere", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		runSt(t, "append", "child")
		repo.Checkout(root)

		d := describeSplit(t, "feature")

		if d.Branch != "feature" || len(d.Commits) != 3 {
			t.Errorf("described %q with %d commits, want feature with 3", d.Branch, len(d.Commits))
		}
		if len(d.Children) != 1 || d.Children[0] != "child" {
			t.Errorf("children = %v, want [child]", d.Children)
		}
	})

	t.Run("flags_a_branch_that_is_behind_its_parent", func(t *testing.T) {
		repo, root := setupSplitFixture(t)
		if describeSplit(t).NeedsRestack {
			t.Error("needs_restack set on an up-to-date branch")
		}
		repo.Checkout(root)
		repo.WriteFile("trunk.txt", "trunk\n")
		repo.AddAndCommit("trunk moves on")
		repo.Checkout("feature")

		d := describeSplit(t)

		if !d.NeedsRestack {
			t.Error("needs_restack not set on a branch behind its parent")
		}
		// The inventory still covers only the branch's own changes.
		if len(d.Commits) != 3 || d.Totals.Files != 5 {
			t.Errorf("described %d commits and %d files, want 3 and 5", len(d.Commits), d.Totals.Files)
		}
	})

	t.Run("rejects_a_branch_outside_the_stack", func(t *testing.T) {
		repo, _ := setupSplitFixture(t)
		repo.CreateBranch("loose")

		out := runStExpectError(t, "split", "describe")

		assertContains(t, out, `branch "loose" is not in the stack`)
	})
}

func TestSplitApplySignsCommitsWhenConfigured(t *testing.T) {
	repo, _ := setupSplitFixture(t)
	addDescendants(t, repo)
	// A stand-in for gpg: git only needs a signature on stdout and a status line.
	signer := filepath.Join(t.TempDir(), "fake-gpg")
	script := "#!/bin/sh\ncat > /dev/null\nprintf -- '-----BEGIN PGP SIGNATURE-----\\n\\nfake\\n-----END PGP SIGNATURE-----\\n'\necho '[GNUPG:] SIG_CREATED ' >&2\n"
	if err := os.WriteFile(signer, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	gitOut(t, repo, "config", "gpg.program", signer)
	gitOut(t, repo, "config", "commit.gpgsign", "true")

	applySplit(t, pathsPlan)

	for _, branch := range []string{"feature-core", "feature", "child"} {
		if raw := gitOut(t, repo, "cat-file", "commit", branch); !strings.Contains(raw, "gpgsig ") {
			t.Errorf("commit at %s is not signed:\n%s", branch, raw)
		}
	}
}
