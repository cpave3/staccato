package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cpave3/staccato/internal/testutil"
)

// setupRefusingOrigin makes a stack root -> f1 -> f2 with f2 checked out and an
// origin whose pre-receive hook refuses pushes to f1, as a credential proxy
// holding a push for approval would.
func setupRefusingOrigin(t *testing.T) *testutil.GitRepo {
	t.Helper()
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)

	runSt(t, "new", "f1")
	repo.CreateFile("f1.txt", "f1")
	repo.AddAndCommit("f1 commit")
	runSt(t, "append", "f2")
	repo.CreateFile("f2.txt", "f2")
	repo.AddAndCommit("f2 commit")

	hook := "#!/bin/sh\nwhile read old new ref; do\n  if [ \"$ref\" = \"refs/heads/f1\" ]; then\n    echo \"held for approval\" >&2\n    exit 1\n  fi\ndone\n"
	if err := os.WriteFile(filepath.Join(repo.OriginDir(), "hooks", "pre-receive"), []byte(hook), 0755); err != nil {
		t.Fatalf("write hook: %v", err)
	}
	return repo
}

// runStStreams runs st and returns its standard output, standard error and exit error.
func runStStreams(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(stBinary, args...)
	setCoverEnv(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func remoteHasBranch(repo *testutil.GitRepo, branch string) bool {
	out, _ := repo.RunGit("ls-remote", "--heads", "origin", branch)
	return strings.TrimSpace(out) != ""
}

func TestSyncReportsARefusedPush(t *testing.T) {
	repo := setupRefusingOrigin(t)

	out := runStExpectError(t, "sync")

	summary := strings.Index(out, "Sync complete")
	if summary < 0 {
		t.Fatalf("no summary in output:\n%s", out)
	}
	// The warning comes after the summary, naming the branch and git's reason.
	assertContains(t, out[summary:], "failed to push f1: ! [remote rejected] f1 -> f1")
	if !remoteHasBranch(repo, "f2") {
		t.Error("f2 was not pushed after f1 was refused")
	}
	if remoteHasBranch(repo, "f1") {
		t.Error("f1 is on the remote although its push was refused")
	}
}

type syncJSON struct {
	TrunkUpdated   *bool    `json:"trunk_updated"`
	MergedBranches []string `json:"merged_branches"`
	RestackedCount *int     `json:"restacked_count"`
	PushedBranches []string `json:"pushed_branches"`
	FailedPushes   []struct {
		Branch string `json:"branch"`
		Reason string `json:"reason"`
	} `json:"failed_pushes"`
	ConflictAt *string `json:"conflict_at"`
}

func TestSyncJSONListsRefusedPushes(t *testing.T) {
	setupRefusingOrigin(t)

	stdout, stderr, err := runStStreams(t, "sync", "--json")

	if err == nil {
		t.Fatalf("st sync --json exited 0 with a refused push\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	var got syncJSON
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%s", err, stdout)
	}
	if len(got.FailedPushes) != 1 || got.FailedPushes[0].Branch != "f1" || !strings.Contains(got.FailedPushes[0].Reason, "[remote rejected]") {
		t.Errorf("failed_pushes = %+v, want f1 with git's reason", got.FailedPushes)
	}
	if len(got.PushedBranches) != 1 || got.PushedBranches[0] != "f2" {
		t.Errorf("pushed_branches = %v, want [f2]", got.PushedBranches)
	}
}

func TestSyncJSONWhenEverythingPushed(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)
	runSt(t, "new", "f1")
	repo.CreateFile("f1.txt", "f1")
	repo.AddAndCommit("f1 commit")

	stdout, stderr, err := runStStreams(t, "sync", "--json")

	if err != nil {
		t.Fatalf("st sync --json failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%s", err, stdout)
	}
	for _, key := range []string{"trunk_updated", "merged_branches", "restacked_count", "pushed_branches", "failed_pushes", "conflict_at"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("JSON has no %q: %s", key, stdout)
		}
	}
	for _, key := range []string{"merged_branches", "failed_pushes"} {
		if string(raw[key]) != "[]" {
			t.Errorf("%s = %s, want []", key, raw[key])
		}
	}
	var pushed []string
	json.Unmarshal(raw["pushed_branches"], &pushed)
	if len(pushed) != 1 || pushed[0] != "f1" {
		t.Errorf("pushed_branches = %s, want [\"f1\"]", raw["pushed_branches"])
	}
}

func TestSyncDryRunJSON(t *testing.T) {
	repo, root := setupRepoWithStack(t)
	if err := repo.AddRemote(); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo.RunGit("push", "-u", "origin", root)
	runSt(t, "new", "f1")
	repo.CreateFile("f1.txt", "f1")
	repo.AddAndCommit("f1 commit")

	stdout, stderr, err := runStStreams(t, "sync", "--dry-run", "--json")

	if err != nil {
		t.Fatalf("st sync --dry-run --json failed: %v\nstderr: %s", err, stderr)
	}
	var got syncJSON
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%s", err, stdout)
	}
	if len(got.PushedBranches) != 1 || got.PushedBranches[0] != "f1" {
		t.Errorf("pushed_branches = %v, want the branch that would be pushed", got.PushedBranches)
	}
	if remoteHasBranch(repo, "f1") {
		t.Error("dry run pushed f1")
	}
}
