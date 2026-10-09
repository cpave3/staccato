package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/cpave3/staccato/internal/testutil"
	"github.com/cpave3/staccato/pkg/stackexec"
)

// execPresent prints the branch the command runs for and which of the
// fixture's files exist in its checkout.
const execPresent = `echo "$ST_EXEC_BRANCH one=$(test -f one.txt && echo y || echo n) two=$(test -f two.txt && echo y || echo n) three=$(test -f three.txt && echo y || echo n)"`

// setupExecFixture builds the stack root → one → two → three, where each branch
// adds a file named after itself, and leaves "two" checked out.
func setupExecFixture(t *testing.T) (*testutil.GitRepo, string) {
	t.Helper()
	repo, root := setupRepoWithStack(t)
	for i, name := range []string{"one", "two", "three"} {
		if i == 0 {
			runSt(t, "new", name)
		} else {
			runSt(t, "append", name)
		}
		repo.WriteFile(name+".txt", name+"\n")
		if err := repo.AddAndCommit("add " + name); err != nil {
			t.Fatalf("commit %s: %v", name, err)
		}
	}
	if err := repo.Checkout("two"); err != nil {
		t.Fatalf("checkout two: %v", err)
	}
	return repo, root
}

// runStExec runs `st exec` and returns its report (stdout), the progress and
// command output (stderr), and the exit error.
func runStExec(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(stBinary, append([]string{"exec"}, args...)...)
	setCoverEnv(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// assertExecReport checks that the report has a line giving branch that status.
func assertExecReport(t *testing.T, report, branch, status string) {
	t.Helper()
	line := regexp.MustCompile(`(?m)^\s*\S+\s+` + regexp.QuoteMeta(branch) + `\s+` + status + `\b`)
	if !line.MatchString(report) {
		t.Errorf("report does not show %s as %s:\n%s", branch, status, report)
	}
}

func TestExec(t *testing.T) {
	t.Run("runs_the_command_on_every_branch_of_the_lineage", func(t *testing.T) {
		repo, _ := setupExecFixture(t)
		log := filepath.Join(t.TempDir(), "seen.log")

		out := runSt(t, "exec", execPresent+" >> "+log)

		seen, err := os.ReadFile(log)
		if err != nil {
			t.Fatalf("read log: %v", err)
		}
		want := "one one=y two=n three=n\ntwo one=y two=y three=n\nthree one=y two=y three=y\n"
		if string(seen) != want {
			t.Errorf("command saw:\n%s\nwant:\n%s", seen, want)
		}
		if cur := getCurrentBranch(t, repo); cur != "two" {
			t.Errorf("current branch = %q, want to be back on two", cur)
		}
		if status := gitOut(t, repo, "status", "--porcelain"); status != "" {
			t.Errorf("working tree not clean after exec:\n%s", status)
		}
		assertContains(t, out, "3 passed")
	})

	t.Run("keeps_going_after_a_failure_and_reports_each_branch", func(t *testing.T) {
		repo, _ := setupExecFixture(t)

		report, progress, err := runStExec(t, `echo "building $ST_EXEC_BRANCH"; test "$ST_EXEC_BRANCH" != two`)

		if err == nil {
			t.Error("st exec exited successfully although the command failed on a branch")
		}
		assertExecReport(t, report, "one", "passed")
		assertExecReport(t, report, "two", "failed")
		assertExecReport(t, report, "three", "passed")
		assertContains(t, report, "2 passed, 1 failed")
		assertContains(t, report, "exit 1")
		// The command's own output is streamed, apart from the report.
		assertContains(t, progress, "building three")
		assertContains(t, progress, "1 of 3 branches failed")
		if cur := getCurrentBranch(t, repo); cur != "two" {
			t.Errorf("current branch = %q, want to be back on two", cur)
		}
	})

	t.Run("stops_at_the_first_failure_with_fail_fast", func(t *testing.T) {
		repo, _ := setupExecFixture(t)

		report, progress, err := runStExec(t, "--fail-fast", `echo "building $ST_EXEC_BRANCH"; test "$ST_EXEC_BRANCH" != two`)

		if err == nil {
			t.Error("st exec exited successfully although the command failed on a branch")
		}
		if strings.Contains(progress, "building three") {
			t.Errorf("the command ran on three after two failed:\n%s", progress)
		}
		assertExecReport(t, report, "one", "passed")
		assertExecReport(t, report, "two", "failed")
		assertExecReport(t, report, "three", "skipped")
		assertContains(t, report, "1 passed, 1 failed, 1 skipped")
		if cur := getCurrentBranch(t, repo); cur != "two" {
			t.Errorf("current branch = %q, want to be back on two", cur)
		}
	})

	t.Run("refuses_with_uncommitted_changes", func(t *testing.T) {
		repo, _ := setupExecFixture(t)
		repo.WriteFile("two.txt", "two\nwork in progress\n")
		log := filepath.Join(t.TempDir(), "seen.log")

		out := runStExpectError(t, "exec", "echo ran >> "+log)

		assertContains(t, out, "uncommitted changes")
		if _, err := os.Stat(log); err == nil {
			t.Error("the command ran although the working tree had uncommitted changes")
		}
		if cur := getCurrentBranch(t, repo); cur != "two" {
			t.Errorf("current branch = %q, want two", cur)
		}
		if status := gitOut(t, repo, "status", "--porcelain"); !strings.Contains(status, "two.txt") {
			t.Errorf("the uncommitted change was lost, status:\n%s", status)
		}
	})

	t.Run("stashes_tracked_changes_the_command_makes", func(t *testing.T) {
		repo, _ := setupExecFixture(t)
		log := filepath.Join(t.TempDir(), "seen.log")

		// A formatter or lockfile update would leave tracked files modified.
		report, _, err := runStExec(t, `cat one.txt >> `+log+`; test "$ST_EXEC_BRANCH" != one || echo generated >> one.txt`)

		if err != nil {
			t.Errorf("st exec failed although the command passed on every branch: %v", err)
		}
		// Later branches see their own content, not what the command left behind.
		if seen, _ := os.ReadFile(log); string(seen) != "one\none\none\n" {
			t.Errorf("one.txt as the command saw it on each branch = %q, want it unchanged every time", seen)
		}
		assertExecReport(t, report, "one", "passed")
		assertContains(t, report, "3 passed")
		assertContains(t, report, "git stash")
		if status := gitOut(t, repo, "status", "--porcelain"); status != "" {
			t.Errorf("working tree not clean after exec:\n%s", status)
		}
		if stashes := gitOut(t, repo, "stash", "list"); strings.Count(stashes, "st exec") != 1 || !strings.Contains(stashes, "one") {
			t.Errorf("stash list = %q, want the command's changes on one kept in one stash", stashes)
		}
		if cur := getCurrentBranch(t, repo); cur != "two" {
			t.Errorf("current branch = %q, want to be back on two", cur)
		}
	})

	t.Run("reports_a_branch_that_cannot_be_checked_out_and_keeps_going", func(t *testing.T) {
		repo, _ := setupExecFixture(t)
		// Untracked here, tracked on three: checking three out would overwrite it.
		repo.WriteFile("three.txt", "mine\n")
		gitOut(t, repo, "checkout", "-q", "one")

		report, _, err := runStExec(t, "true")

		if err == nil {
			t.Error("st exec exited successfully although a branch could not be checked out")
		}
		assertExecReport(t, report, "one", "passed")
		assertExecReport(t, report, "two", "passed")
		assertExecReport(t, report, "three", "failed")
		assertContains(t, report, "could not be checked out")
		assertContains(t, report, "2 passed, 1 failed")
		if content, _ := os.ReadFile(filepath.Join(repo.Dir, "three.txt")); string(content) != "mine\n" {
			t.Errorf("the untracked three.txt = %q, want it left as it was", content)
		}
		if cur := getCurrentBranch(t, repo); cur != "one" {
			t.Errorf("current branch = %q, want to be back on one", cur)
		}
	})

	t.Run("returns_to_the_branch_when_interrupted", func(t *testing.T) {
		repo, _ := setupExecFixture(t)

		cmd := exec.Command(stBinary, "exec", "sleep 30")
		setCoverEnv(cmd)
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		if err := cmd.Start(); err != nil {
			t.Fatalf("start: %v", err)
		}
		// Wait until the first branch has been checked out.
		deadline := time.Now().Add(10 * time.Second)
		for getCurrentBranch(t, repo) != "HEAD" {
			if time.Now().After(deadline) {
				cmd.Process.Kill()
				t.Fatal("st exec never checked out a branch")
			}
			time.Sleep(20 * time.Millisecond)
		}
		started := time.Now()
		cmd.Process.Signal(os.Interrupt)
		if err := cmd.Wait(); err == nil {
			t.Error("interrupted st exec exited successfully")
		}
		if waited := time.Since(started); waited > 10*time.Second {
			t.Errorf("st took %s to stop after the interrupt", waited)
		}

		if cur := getCurrentBranch(t, repo); cur != "two" {
			t.Errorf("current branch = %q, want to be back on two", cur)
		}
		// The branches that never ran are still accounted for.
		report := stdout.String()
		assertExecReport(t, report, "one", "failed")
		assertExecReport(t, report, "two", "skipped")
		assertExecReport(t, report, "three", "skipped")
		assertContains(t, report, "interrupted")
	})

	t.Run("reports_as_json", func(t *testing.T) {
		repo, root := setupExecFixture(t)

		// The command prints to stdout, which must not end up in the JSON.
		stdout, _, err := runStExec(t, "--json", `echo "building $ST_EXEC_BRANCH"; test "$ST_EXEC_BRANCH" != two || exit 3`)

		if err == nil {
			t.Error("st exec --json exited successfully although the command failed on a branch")
		}
		var result stackexec.Result
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
		}
		if result.Passed != 2 || result.Failed != 1 || result.Skipped != 0 {
			t.Errorf("passed/failed/skipped = %d/%d/%d, want 2/1/0", result.Passed, result.Failed, result.Skipped)
		}
		if len(result.Branches) != 3 {
			t.Fatalf("branches = %+v, want three", result.Branches)
		}
		one, two := result.Branches[0], result.Branches[1]
		if one.Branch != "one" || one.Parent != root || one.Status != "passed" || one.ExitCode == nil || *one.ExitCode != 0 {
			t.Errorf("first branch = %+v, want one on %s, passed with exit code 0", one, root)
		}
		if one.SHA != gitOut(t, repo, "rev-parse", "one") {
			t.Errorf("sha of one = %q, want its tip", one.SHA)
		}
		if two.Branch != "two" || two.Parent != "one" || two.Status != "failed" || two.ExitCode == nil || *two.ExitCode != 3 {
			t.Errorf("second branch = %+v, want two on one, failed with exit code 3", two)
		}
	})

	t.Run("reports_what_ran_when_the_run_cannot_go_on", func(t *testing.T) {
		setupExecFixture(t)

		// On two the command changes a tracked file and leaves the index
		// locked, so its change cannot be stashed and no branch can be checked
		// out after it.
		report, progress, err := runStExec(t, `test "$ST_EXEC_BRANCH" != two || { echo generated >> one.txt; touch .git/index.lock; }`)

		if err == nil {
			t.Error("st exec exited successfully although it could not finish")
		}
		assertContains(t, progress, "could not be stashed")
		assertExecReport(t, report, "one", "passed")
		assertExecReport(t, report, "two", "passed")
		assertExecReport(t, report, "three", "skipped")
	})

	t.Run("tells_the_command_which_branch_it_is_on", func(t *testing.T) {
		repo, root := setupExecFixture(t)
		log := filepath.Join(t.TempDir(), "seen.log")

		runSt(t, "exec", `echo "$ST_EXEC_INDEX/$ST_EXEC_TOTAL $ST_EXEC_BRANCH on $ST_EXEC_PARENT at $ST_EXEC_SHA" >> `+log)

		seen, _ := os.ReadFile(log)
		want := fmt.Sprintf("1/3 one on %s at %s\n2/3 two on one at %s\n3/3 three on two at %s\n",
			root, gitOut(t, repo, "rev-parse", "one"), gitOut(t, repo, "rev-parse", "two"), gitOut(t, repo, "rev-parse", "three"))
		if string(seen) != want {
			t.Errorf("command saw:\n%s\nwant:\n%s", seen, want)
		}
	})

	t.Run("takes_the_command_as_separate_words", func(t *testing.T) {
		setupExecFixture(t)

		// -f belongs to test, not to st.
		report, _, _ := runStExec(t, "test", "-f", "two.txt")

		assertExecReport(t, report, "one", "failed")
		assertExecReport(t, report, "two", "passed")
		assertExecReport(t, report, "three", "passed")
		assertContains(t, report, "branch(es): test -f two.txt\n")
	})

	t.Run("keeps_each_separate_word_as_one_argument", func(t *testing.T) {
		setupExecFixture(t)

		// The script is a single argument to sh, spaces and all.
		report, _, _ := runStExec(t, "--fail-fast", "sh", "-c", `test "$ST_EXEC_BRANCH" != two`)

		assertExecReport(t, report, "one", "passed")
		assertExecReport(t, report, "two", "failed")
		assertExecReport(t, report, "three", "skipped")
	})

	t.Run("refuses_a_quoted_command_followed_by_more_words", func(t *testing.T) {
		setupExecFixture(t)
		log := filepath.Join(t.TempDir(), "seen.log")

		// Read as words, this is a program called "echo ran >> log" given the
		// argument --fail-fast, which would fail on every branch.
		out := runStExpectError(t, "exec", "echo ran >> "+log, "--fail-fast")

		assertContains(t, out, "before the command")
		assertContains(t, out, "one quoted argument")
		if _, err := os.Stat(log); err == nil {
			t.Error("the command ran")
		}
	})

	t.Run("needs_a_command", func(t *testing.T) {
		setupExecFixture(t)

		out := runStExpectError(t, "exec")

		assertContains(t, out, "needs a command")
		assertContains(t, out, "st exec --help")
	})

	t.Run("refuses_on_a_branch_that_is_not_in_the_stack", func(t *testing.T) {
		repo, _ := setupExecFixture(t)
		gitOut(t, repo, "checkout", "-q", "-b", "untracked")
		log := filepath.Join(t.TempDir(), "seen.log")

		out := runStExpectError(t, "exec", "echo ran >> "+log)

		assertContains(t, out, "not in the stack")
		assertContains(t, out, "st attach")
		if _, err := os.Stat(log); err == nil {
			t.Error("the command ran although the current branch is not in the stack")
		}
	})

	t.Run("refuses_on_a_detached_head", func(t *testing.T) {
		repo, _ := setupExecFixture(t)
		gitOut(t, repo, "checkout", "-q", "--detach")

		out := runStExpectError(t, "exec", "true")

		assertContains(t, out, "HEAD is detached")
	})

	t.Run("refuses_when_the_stack_has_no_branches", func(t *testing.T) {
		setupRepoWithStack(t)

		out := runStExpectError(t, "exec", "true")

		assertContains(t, out, "no branches")
	})

	t.Run("leaves_other_stacks_alone", func(t *testing.T) {
		repo, root := setupExecFixture(t)
		// A second stack on the trunk, beside one → two → three.
		gitOut(t, repo, "checkout", "-q", root)
		runSt(t, "new", "other")
		repo.WriteFile("other.txt", "other\n")
		if err := repo.AddAndCommit("add other"); err != nil {
			t.Fatal(err)
		}
		gitOut(t, repo, "checkout", "-q", "three")

		report, _, err := runStExec(t, "true")

		if err != nil {
			t.Fatalf("st exec failed: %v\n%s", err, report)
		}
		assertContains(t, report, "3 passed")
		if strings.Contains(report, "other") {
			t.Errorf("the command ran on a branch outside the current lineage:\n%s", report)
		}
	})

	t.Run("visits_a_forked_stack_in_the_same_order_every_time", func(t *testing.T) {
		repo, _ := setupExecFixture(t)
		// Four more branches on one, beside two.
		for _, name := range []string{"fork-d", "fork-b", "fork-a", "fork-c"} {
			gitOut(t, repo, "checkout", "-q", "one")
			runSt(t, "append", name)
		}
		gitOut(t, repo, "checkout", "-q", "one")

		stdout, _, err := runStExec(t, "--json", "true")

		if err != nil {
			t.Fatalf("st exec failed: %v\n%s", err, stdout)
		}
		var result stackexec.Result
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
		}
		var order []string
		for _, b := range result.Branches {
			order = append(order, b.Branch)
		}
		// A parent before its children, and siblings by name.
		if got, want := strings.Join(order, " "), "one fork-a fork-b fork-c fork-d two three"; got != want {
			t.Errorf("visited %q, want %q", got, want)
		}
	})

	t.Run("runs_on_every_tracked_branch_from_the_trunk", func(t *testing.T) {
		repo, root := setupExecFixture(t)
		gitOut(t, repo, "checkout", "-q", root)
		runSt(t, "new", "other")
		gitOut(t, repo, "checkout", "-q", root)

		report, _, err := runStExec(t, "true")

		if err != nil {
			t.Fatalf("st exec failed: %v\n%s", err, report)
		}
		for _, branch := range []string{"one", "two", "three", "other"} {
			assertExecReport(t, report, branch, "passed")
		}
		if strings.Contains(report, " "+root+" ") {
			t.Errorf("the command ran on the trunk itself:\n%s", report)
		}
		if cur := getCurrentBranch(t, repo); cur != root {
			t.Errorf("current branch = %q, want to be back on %s", cur, root)
		}
	})
}
