package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitRunner_Run(t *testing.T) {
	// Create a temp git repo
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	git := NewRunner(tmpDir)

	output, err := git.Run("rev-parse", "--git-dir")
	if err != nil {
		t.Fatalf("failed to run git command: %v", err)
	}

	if !strings.Contains(output, ".git") {
		t.Errorf("expected output to contain .git, got: %s", output)
	}
}

func TestGitRunner_GetCurrentBranch(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	git := NewRunner(tmpDir)

	branch, err := git.GetCurrentBranch()
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}

	// Should be on main or master depending on git version
	if branch != "main" && branch != "master" {
		t.Errorf("expected main or master, got: %s", branch)
	}
}

func TestGitRunner_CreateBranch(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	git := NewRunner(tmpDir)

	err := git.CreateBranch("feature-branch")
	if err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	// Verify branch exists
	output, _ := exec.Command("git", "-C", tmpDir, "branch", "--list", "feature-branch").Output()
	if !strings.Contains(string(output), "feature-branch") {
		t.Error("expected branch to be created")
	}
}

func TestGitRunner_GetCommitSHA(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	git := NewRunner(tmpDir)

	sha, err := git.GetCommitSHA("HEAD")
	if err != nil {
		t.Fatalf("failed to get commit SHA: %v", err)
	}

	if len(sha) != 40 {
		t.Errorf("expected 40 character SHA, got: %s (%d chars)", sha, len(sha))
	}
}

func TestGitRunner_EnableRerere(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	git := NewRunner(tmpDir)

	err := git.EnableRerere()
	if err != nil {
		t.Fatalf("failed to enable rerere: %v", err)
	}

	// Verify rerere is enabled
	output, _ := exec.Command("git", "-C", tmpDir, "config", "rerere.enabled").Output()
	if strings.TrimSpace(string(output)) != "true" {
		t.Errorf("expected rerere.enabled to be true, got: %s", string(output))
	}
}

func initRepoWithCommit(t *testing.T) (string, *Runner) {
	t.Helper()
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()
	return tmpDir, NewRunner(tmpDir)
}

func TestGitRunner_RefExists(t *testing.T) {
	_, git := initRepoWithCommit(t)

	if git.RefExists("refs/staccato/graph") {
		t.Error("ref should not exist initially")
	}
	if !git.RefExists("refs/heads/main") {
		t.Error("refs/heads/main should exist")
	}
}

func TestGitRunner_WriteBlobRef_ReadBlobRef(t *testing.T) {
	_, git := initRepoWithCommit(t)

	ref := "refs/staccato/graph"
	data := []byte(`{"version":1,"root":"main","branches":{}}`)

	if err := git.WriteBlobRef(ref, data); err != nil {
		t.Fatalf("WriteBlobRef failed: %v", err)
	}

	if !git.RefExists(ref) {
		t.Fatal("ref should exist after WriteBlobRef")
	}

	got, err := git.ReadBlobRef(ref)
	if err != nil {
		t.Fatalf("ReadBlobRef failed: %v", err)
	}

	if string(got) != string(data) {
		t.Errorf("ReadBlobRef = %q, want %q", got, data)
	}
}

func TestGitRunner_DeleteRef(t *testing.T) {
	_, git := initRepoWithCommit(t)

	ref := "refs/staccato/graph"
	git.WriteBlobRef(ref, []byte("test data"))

	if !git.RefExists(ref) {
		t.Fatal("ref should exist before delete")
	}

	if err := git.DeleteRef(ref); err != nil {
		t.Fatalf("DeleteRef failed: %v", err)
	}

	if git.RefExists(ref) {
		t.Error("ref should not exist after delete")
	}
}

func TestGitRunner_WriteBlobRef_Overwrite(t *testing.T) {
	_, git := initRepoWithCommit(t)

	ref := "refs/staccato/graph"
	git.WriteBlobRef(ref, []byte("first"))

	if err := git.WriteBlobRef(ref, []byte("second")); err != nil {
		t.Fatalf("second WriteBlobRef failed: %v", err)
	}

	got, _ := git.ReadBlobRef(ref)
	if string(got) != "second" {
		t.Errorf("ReadBlobRef = %q, want %q", got, "second")
	}
}

func TestGitRunner_HasUncommittedChanges(t *testing.T) {
	tmpDir, git := initRepoWithCommit(t)

	// Clean state — no uncommitted changes
	has, err := git.HasUncommittedChanges()
	if err != nil {
		t.Fatalf("HasUncommittedChanges: %v", err)
	}
	if has {
		t.Error("expected no uncommitted changes on clean repo")
	}

	// Create an unstaged file
	os.WriteFile(filepath.Join(tmpDir, "dirty.txt"), []byte("dirty"), 0644)
	has, err = git.HasUncommittedChanges()
	if err != nil {
		t.Fatalf("HasUncommittedChanges: %v", err)
	}
	if !has {
		t.Error("expected uncommitted changes after creating untracked file")
	}
}

func TestGitRunner_HasUncommittedChanges_StagedFile(t *testing.T) {
	tmpDir, git := initRepoWithCommit(t)

	// Stage a new file (but don't commit)
	os.WriteFile(filepath.Join(tmpDir, "staged.txt"), []byte("staged"), 0644)
	exec.Command("git", "-C", tmpDir, "add", "staged.txt").Run()

	has, err := git.HasUncommittedChanges()
	if err != nil {
		t.Fatalf("HasUncommittedChanges: %v", err)
	}
	if !has {
		t.Error("expected uncommitted changes with staged file")
	}
}

func TestGitRunner_StashPush(t *testing.T) {
	tmpDir, git := initRepoWithCommit(t)

	// Create a dirty file
	os.WriteFile(filepath.Join(tmpDir, "dirty.txt"), []byte("dirty"), 0644)
	exec.Command("git", "-C", tmpDir, "add", "dirty.txt").Run()

	err := git.StashPush("test stash message")
	if err != nil {
		t.Fatalf("StashPush: %v", err)
	}

	// Working tree should be clean now
	has, _ := git.HasUncommittedChanges()
	if has {
		t.Error("expected clean working tree after stash push")
	}

	// Stash list should have our entry
	out, _ := git.Run("stash", "list")
	if !strings.Contains(out, "test stash message") {
		t.Errorf("stash list should contain our message, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// TestDiff
// ---------------------------------------------------------------------------

func TestGitRunner_Diff(t *testing.T) {
	t.Run("unstaged_no_paths", func(t *testing.T) {
		tmpDir, git := initRepoWithCommit(t)
		os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("changed"), 0644)
		out, err := git.Diff(false, nil)
		if err != nil {
			t.Fatalf("Diff: %v", err)
		}
		if !strings.Contains(out, "changed") {
			t.Error("expected diff to show change")
		}
	})

	t.Run("staged", func(t *testing.T) {
		tmpDir, git := initRepoWithCommit(t)
		os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("staged change"), 0644)
		exec.Command("git", "-C", tmpDir, "add", "test.txt").Run()
		out, err := git.Diff(true, nil)
		if err != nil {
			t.Fatalf("Diff staged: %v", err)
		}
		if !strings.Contains(out, "staged change") {
			t.Error("expected staged diff to show change")
		}
	})

	t.Run("with_paths", func(t *testing.T) {
		tmpDir, git := initRepoWithCommit(t)
		os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("changed"), 0644)
		os.WriteFile(filepath.Join(tmpDir, "other.txt"), []byte("other"), 0644)
		exec.Command("git", "-C", tmpDir, "add", "other.txt").Run()
		exec.Command("git", "-C", tmpDir, "commit", "-m", "add other").Run()
		os.WriteFile(filepath.Join(tmpDir, "other.txt"), []byte("other changed"), 0644)

		out, err := git.Diff(false, []string{"other.txt"})
		if err != nil {
			t.Fatalf("Diff with paths: %v", err)
		}
		if !strings.Contains(out, "other changed") {
			t.Error("expected diff filtered to other.txt")
		}
	})
}

// ---------------------------------------------------------------------------
// TestStashPop
// ---------------------------------------------------------------------------

func TestGitRunner_StashPop(t *testing.T) {
	tmpDir, git := initRepoWithCommit(t)

	// Stage and stash a change
	os.WriteFile(filepath.Join(tmpDir, "stashed.txt"), []byte("stash me"), 0644)
	exec.Command("git", "-C", tmpDir, "add", "stashed.txt").Run()
	git.StashPush("test stash")

	// Working tree should be clean
	has, _ := git.HasUncommittedChanges()
	if has {
		t.Fatal("expected clean after stash push")
	}

	// Pop the stash
	err := git.StashPop()
	if err != nil {
		t.Fatalf("StashPop: %v", err)
	}

	// File should be back
	has, _ = git.HasUncommittedChanges()
	if !has {
		t.Error("expected uncommitted changes after stash pop")
	}
}

func TestGitRunner_GetAllBranches(t *testing.T) {
	tmpDir, git := initRepoWithCommit(t)

	exec.Command("git", "-C", tmpDir, "branch", "feature-1").Run()
	exec.Command("git", "-C", tmpDir, "branch", "feature-2").Run()

	branches, err := git.GetAllBranches()
	if err != nil {
		t.Fatalf("GetAllBranches: %v", err)
	}

	found := map[string]bool{}
	for _, b := range branches {
		found[b] = true
	}
	for _, expected := range []string{"main", "feature-1", "feature-2"} {
		if !found[expected] {
			t.Errorf("expected branch %s in list, got: %v", expected, branches)
		}
	}
}

// initRepoWithRemote creates a local repo with a bare "origin" remote and an initial commit.
func initRepoWithRemote(t *testing.T) (string, *Runner) {
	t.Helper()

	// Create a bare repo to act as origin
	bareDir := t.TempDir()
	if err := exec.Command("git", "init", "--bare", "-b", "main", bareDir).Run(); err != nil {
		t.Fatalf("failed to init bare repo: %v", err)
	}

	// Create a local repo with an initial commit
	tmpDir, git := initRepoWithCommit(t)

	// Add the bare repo as origin and push main
	exec.Command("git", "-C", tmpDir, "remote", "add", "origin", bareDir).Run()
	exec.Command("git", "-C", tmpDir, "push", "-u", "origin", "main").Run()

	return tmpDir, git
}

func TestGitRunner_Push_SetsUpstream(t *testing.T) {
	tmpDir, git := initRepoWithRemote(t)

	// Create a new branch and commit
	exec.Command("git", "-C", tmpDir, "checkout", "-b", "feature").Run()
	os.WriteFile(filepath.Join(tmpDir, "feat.txt"), []byte("feat"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "feature commit").Run()

	// Push via Runner — should set upstream
	if err := git.Push("feature", false); err != nil {
		t.Fatalf("Push: %v", err)
	}

	// Verify upstream is set
	out, err := exec.Command("git", "-C", tmpDir, "config", "branch.feature.remote").Output()
	if err != nil {
		t.Fatalf("upstream remote not set: %v", err)
	}
	if strings.TrimSpace(string(out)) != "origin" {
		t.Errorf("expected upstream remote 'origin', got %q", strings.TrimSpace(string(out)))
	}

	mergeRef, err := exec.Command("git", "-C", tmpDir, "config", "branch.feature.merge").Output()
	if err != nil {
		t.Fatalf("upstream merge ref not set: %v", err)
	}
	if strings.TrimSpace(string(mergeRef)) != "refs/heads/feature" {
		t.Errorf("expected upstream merge 'refs/heads/feature', got %q", strings.TrimSpace(string(mergeRef)))
	}
}

func TestGitRunner_PushAll_SetsUpstream(t *testing.T) {
	tmpDir, git := initRepoWithRemote(t)

	// Create two branches with commits
	for _, branch := range []string{"feat-a", "feat-b"} {
		exec.Command("git", "-C", tmpDir, "checkout", "main").Run()
		exec.Command("git", "-C", tmpDir, "checkout", "-b", branch).Run()
		os.WriteFile(filepath.Join(tmpDir, branch+".txt"), []byte(branch), 0644)
		exec.Command("git", "-C", tmpDir, "add", ".").Run()
		exec.Command("git", "-C", tmpDir, "commit", "-m", branch+" commit").Run()
	}

	if err := git.PushAll([]string{"feat-a", "feat-b"}, false); err != nil {
		t.Fatalf("PushAll: %v", err)
	}

	// Verify upstream is set for both
	for _, branch := range []string{"feat-a", "feat-b"} {
		out, err := exec.Command("git", "-C", tmpDir, "config", "branch."+branch+".remote").Output()
		if err != nil {
			t.Fatalf("upstream remote not set for %s: %v", branch, err)
		}
		if strings.TrimSpace(string(out)) != "origin" {
			t.Errorf("branch %s: expected upstream remote 'origin', got %q", branch, strings.TrimSpace(string(out)))
		}
	}
}

func TestGitRunner_GetMergeBase(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	// Create initial commit on main
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	// Get base SHA
	baseOutput, _ := exec.Command("git", "-C", tmpDir, "rev-parse", "HEAD").Output()
	baseSHA := strings.TrimSpace(string(baseOutput))

	// Create feature branch with a commit
	exec.Command("git", "-C", tmpDir, "checkout", "-b", "feature").Run()
	os.WriteFile(testFile, []byte("feature change"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "feature commit").Run()

	git := NewRunner(tmpDir)

	mergeBase, err := git.GetMergeBase("feature", "main")
	if err != nil {
		t.Fatalf("failed to get merge base: %v", err)
	}

	if mergeBase != baseSHA {
		t.Errorf("expected merge base %s, got: %s", baseSHA, mergeBase)
	}
}

// commitFile writes content to name and commits it, returning the new commit.
func commitFile(t *testing.T, dir, name, content, message string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-q", "-m", message}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func initCommitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test User"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestGitRunner_Output(t *testing.T) {
	dir := initCommitRepo(t)
	commitFile(t, dir, " lead.txt", "x\n", "odd name")
	git := NewRunner(dir)

	t.Run("keeps_output_untrimmed", func(t *testing.T) {
		out, err := git.Output(nil, "", "ls-files", "-z")
		if err != nil {
			t.Fatalf("Output: %v", err)
		}
		if out != " lead.txt\x00" {
			t.Errorf("Output = %q, want the leading space and NUL terminator kept", out)
		}
	})

	t.Run("passes_environment_and_stdin", func(t *testing.T) {
		index := filepath.Join(t.TempDir(), "index")
		env := []string{"GIT_INDEX_FILE=" + index}
		blob, err := git.RunWith(nil, "content\n", "hash-object", "-w", "--stdin")
		if err != nil {
			t.Fatalf("hash-object: %v", err)
		}
		if _, err := git.Output(env, "100644 "+blob+"\tnew.txt\x00", "update-index", "-z", "--index-info"); err != nil {
			t.Fatalf("update-index: %v", err)
		}
		scratch, _ := git.Output(env, "", "ls-files")
		real, _ := git.Output(nil, "", "ls-files")
		if scratch != "new.txt\n" || real != " lead.txt\n" {
			t.Errorf("scratch index = %q, real index = %q", scratch, real)
		}
	})

	t.Run("reports_stderr_only_in_the_error", func(t *testing.T) {
		_, err := git.Output(nil, "", "rev-parse", "--verify", "no-such-ref")
		if err == nil || !strings.Contains(err.Error(), "no-such-ref") {
			t.Errorf("err = %v, want it to carry git's message", err)
		}
	})
}

func TestGitRunner_CherryPickTree(t *testing.T) {
	dir := initCommitRepo(t)
	base := commitFile(t, dir, "a.txt", "one\n", "base")
	second := commitFile(t, dir, "a.txt", "one\ntwo\n", "extend a")
	third := commitFile(t, dir, "b.txt", "b\n", "add b")
	git := NewRunner(dir)

	t.Run("applies_a_commit_without_touching_the_checkout", func(t *testing.T) {
		pick, err := git.CherryPickTree(base, third)
		if err != nil {
			t.Fatalf("CherryPickTree: %v", err)
		}
		if pick.Conflicted {
			t.Fatalf("unexpected conflict in %v", pick.Paths)
		}
		files, err := git.Run("ls-tree", "--name-only", pick.Tree)
		if err != nil {
			t.Fatal(err)
		}
		if files != "a.txt\nb.txt" {
			t.Errorf("picked tree holds %q, want a.txt and b.txt", files)
		}
		// b.txt is applied on base, where a.txt still has one line.
		if content, _ := git.Run("show", pick.Tree+":a.txt"); content != "one" {
			t.Errorf("a.txt = %q, want it as it is on base", content)
		}
		if head, _ := git.Run("rev-parse", "HEAD"); head != third {
			t.Errorf("HEAD moved to %s", head)
		}
	})

	t.Run("reports_conflicted_paths", func(t *testing.T) {
		// A commit that rewrites the same line differently from second.
		exec.Command("git", "-C", dir, "checkout", "-q", "-b", "other", base).Run()
		other := commitFile(t, dir, "a.txt", "one\nother\n", "diverge")

		pick, err := git.CherryPickTree(other, second)
		if err != nil {
			t.Fatalf("CherryPickTree: %v", err)
		}
		if !pick.Conflicted || len(pick.Paths) != 1 || pick.Paths[0] != "a.txt" {
			t.Errorf("pick = %+v, want a conflict in a.txt", pick)
		}
		if pick.Tree != "" {
			t.Errorf("conflicted pick returned tree %q", pick.Tree)
		}
	})
}

// rejectPushesOf installs a pre-receive hook in bareDir that refuses any push
// to the named branch, saying so on standard error.
func rejectPushesOf(t *testing.T, bareDir, branch string) {
	t.Helper()
	hook := filepath.Join(bareDir, "hooks", "pre-receive")
	script := "#!/bin/sh\nwhile read old new ref; do\n  if [ \"$ref\" = \"refs/heads/" + branch + "\" ]; then\n    echo \"held for approval\" >&2\n    exit 1\n  fi\ndone\n"
	if err := os.WriteFile(hook, []byte(script), 0755); err != nil {
		t.Fatalf("write hook: %v", err)
	}
}

func TestGitRunner_Push_RefusedPushNamesGitsReason(t *testing.T) {
	tmpDir, runner := initRepoWithRemote(t)
	bareDir, _ := exec.Command("git", "-C", tmpDir, "remote", "get-url", "origin").Output()
	rejectPushesOf(t, strings.TrimSpace(string(bareDir)), "feature")

	exec.Command("git", "-C", tmpDir, "checkout", "-b", "feature").Run()
	os.WriteFile(filepath.Join(tmpDir, "feat.txt"), []byte("feat"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "feature commit").Run()

	err := runner.Push("feature", true)

	var pushErr *PushError
	if !errors.As(err, &pushErr) {
		t.Fatalf("Push error = %v, want a *PushError", err)
	}
	if pushErr.Ref != "feature" {
		t.Errorf("Ref = %q, want feature", pushErr.Ref)
	}
	if !strings.HasPrefix(pushErr.Reason, "! [remote rejected]") || !strings.Contains(pushErr.Reason, "feature") {
		t.Errorf("Reason = %q, want git's [remote rejected] line for feature", pushErr.Reason)
	}
}

func TestPushReason(t *testing.T) {
	cases := []struct {
		name, stderr, want string
	}{
		{
			"ref status line wins over the generic error",
			"remote: held for approval\nTo /tmp/origin\n ! [remote rejected] x -> x (pending ap-1)\nerror: failed to push some refs to '/tmp/origin'\n",
			"! [remote rejected] x -> x (pending ap-1)",
		},
		{
			"stale lease with hints",
			"To /tmp/origin\n ! [rejected]        x -> x (stale info)\nerror: failed to push some refs to '/tmp/origin'\nhint: Updates were rejected\nhint: more advice\n",
			"! [rejected]        x -> x (stale info)",
		},
		{
			"fatal line when no ref was reported",
			"fatal: unable to access 'https://example.com/': Could not resolve host\n",
			"fatal: unable to access 'https://example.com/': Could not resolve host",
		},
		{
			"last remote line when nothing better",
			"remote: first\nremote: proxy refused this push\nerror: failed to push some refs to 'x'\n",
			"remote: proxy refused this push",
		},
		{
			"last non-empty line otherwise",
			"something odd\n\n",
			"something odd",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := PushReason(c.stderr); got != c.want {
				t.Errorf("PushReason = %q, want %q", got, c.want)
			}
		})
	}
}
