package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

// writeScript creates an executable script in the given directory.
func writeScript(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeNonExecutable creates a non-executable file in the given directory.
func writeNonExecutable(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDiscoverInDir_ExecutableOnly(t *testing.T) {
	dir := t.TempDir()

	writeScript(t, dir, "hook.sh", "#!/bin/sh\nexit 0\n")
	writeNonExecutable(t, dir, "README.md", "not a hook")
	writeScript(t, dir, "another.sh", "#!/bin/sh\nexit 0\n")

	scripts := discoverInDir(dir)
	if len(scripts) != 2 {
		t.Fatalf("expected 2 scripts, got %d: %v", len(scripts), scripts)
	}
}

func TestDiscoverInDir_MissingDir(t *testing.T) {
	scripts := discoverInDir("/nonexistent/path")
	if len(scripts) != 0 {
		t.Fatalf("expected 0 scripts for missing dir, got %d", len(scripts))
	}
}

func TestDiscover_TwoLevelResolution(t *testing.T) {
	projectDir := t.TempDir()
	globalDir := t.TempDir()

	// Create a runner that uses our temp dirs
	r := &Runner{repoPath: projectDir}

	// Override hookDirs for testing by creating the expected directory structure
	eventDir := filepath.Join(projectDir, ".staccato", "hooks", string(PostBranchCreate))
	writeScript(t, eventDir, "project.sh", "#!/bin/sh\nexit 0\n")

	// Test project-level discovery
	scripts := r.discover(PostBranchCreate)
	if len(scripts) != 1 {
		t.Fatalf("expected 1 project script, got %d: %v", len(scripts), scripts)
	}

	_ = globalDir // global dir tested in integration tests (requires HOME override)
}

func TestFire_SuccessfulHook(t *testing.T) {
	projectDir := t.TempDir()

	eventDir := filepath.Join(projectDir, ".staccato", "hooks", string(PostBranchCreate))
	markerFile := filepath.Join(projectDir, "hook-ran")

	writeScript(t, eventDir, "hook.sh", "#!/bin/sh\ntouch "+markerFile+"\n")

	r := NewRunner(projectDir)
	err := r.Fire(Context{
		Event:    PostBranchCreate,
		RepoPath: projectDir,
		Branch:   "feature-1",
		Data:     map[string]any{"parent": "main"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(markerFile); os.IsNotExist(err) {
		t.Fatal("hook script was not executed")
	}
}

func TestFire_PreHookBlocks(t *testing.T) {
	projectDir := t.TempDir()

	eventDir := filepath.Join(projectDir, ".staccato", "hooks", string(PreSync))
	writeScript(t, eventDir, "block.sh", "#!/bin/sh\necho 'uncommitted changes' >&2\nexit 2\n")

	r := NewRunner(projectDir)
	err := r.Fire(Context{
		Event:    PreSync,
		RepoPath: projectDir,
		Branch:   "feature-1",
	})
	if err == nil {
		t.Fatal("expected error from blocking pre-hook, got nil")
	}
	if err.Error() != "uncommitted changes" {
		t.Fatalf("expected 'uncommitted changes', got '%s'", err.Error())
	}
}

func TestFire_PostHookExit2IsWarning(t *testing.T) {
	projectDir := t.TempDir()

	eventDir := filepath.Join(projectDir, ".staccato", "hooks", string(PostSync))
	writeScript(t, eventDir, "warn.sh", "#!/bin/sh\necho 'warning message' >&2\nexit 2\n")

	r := NewRunner(projectDir)
	err := r.Fire(Context{
		Event:    PostSync,
		RepoPath: projectDir,
		Branch:   "feature-1",
	})
	if err != nil {
		t.Fatalf("post-hook exit 2 should not return error, got: %v", err)
	}
}

func TestFire_NonZeroNon2ExitContinues(t *testing.T) {
	projectDir := t.TempDir()

	eventDir := filepath.Join(projectDir, ".staccato", "hooks", string(PostBranchCreate))
	markerFile := filepath.Join(projectDir, "second-ran")

	writeScript(t, eventDir, "01-fail.sh", "#!/bin/sh\nexit 1\n")
	writeScript(t, eventDir, "02-ok.sh", "#!/bin/sh\ntouch "+markerFile+"\n")

	r := NewRunner(projectDir)
	err := r.Fire(Context{
		Event:    PostBranchCreate,
		RepoPath: projectDir,
		Branch:   "feature-1",
	})
	if err != nil {
		t.Fatalf("non-zero non-2 exit should not return error, got: %v", err)
	}

	if _, err := os.Stat(markerFile); os.IsNotExist(err) {
		t.Fatal("second hook script should have run after first failed with exit 1")
	}
}

func TestFire_ReceivesJSON(t *testing.T) {
	projectDir := t.TempDir()

	eventDir := filepath.Join(projectDir, ".staccato", "hooks", string(PostPRCreate))
	outputFile := filepath.Join(projectDir, "stdin.json")

	writeScript(t, eventDir, "capture.sh", "#!/bin/sh\ncat > "+outputFile+"\n")

	r := NewRunner(projectDir)
	err := r.Fire(Context{
		Event:    PostPRCreate,
		RepoPath: projectDir,
		Branch:   "feature-1",
		Data:     map[string]any{"base": "main", "web": true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read captured stdin: %v", err)
	}

	// Verify JSON contains expected fields
	content := string(data)
	for _, expected := range []string{`"event":"post-pr-create"`, `"branch":"feature-1"`, `"base":"main"`} {
		if !contains(content, expected) {
			t.Errorf("expected stdin JSON to contain %s, got: %s", expected, content)
		}
	}
}

func TestFire_ReceivesEnvVars(t *testing.T) {
	projectDir := t.TempDir()

	eventDir := filepath.Join(projectDir, ".staccato", "hooks", string(PostBranchCreate))
	outputFile := filepath.Join(projectDir, "env.txt")

	writeScript(t, eventDir, "env.sh", "#!/bin/sh\necho \"$ST_EVENT|$ST_REPO_PATH|$ST_BRANCH\" > "+outputFile+"\n")

	r := NewRunner(projectDir)
	err := r.Fire(Context{
		Event:    PostBranchCreate,
		RepoPath: projectDir,
		Branch:   "test-branch",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read env output: %v", err)
	}

	expected := "post-branch-create|" + projectDir + "|test-branch"
	if got := trimNewline(string(data)); got != expected {
		t.Errorf("expected env vars '%s', got '%s'", expected, got)
	}
}

func TestFire_NoHooksIsNoOp(t *testing.T) {
	projectDir := t.TempDir()

	r := NewRunner(projectDir)
	err := r.Fire(Context{
		Event:    PostSync,
		RepoPath: projectDir,
		Branch:   "feature-1",
	})
	if err != nil {
		t.Fatalf("no hooks should succeed silently, got: %v", err)
	}
}

func TestFire_GlobalBeforeProject(t *testing.T) {
	projectDir := t.TempDir()

	// Use a temp dir as fake HOME
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	orderFile := filepath.Join(projectDir, "order.txt")

	globalEventDir := filepath.Join(fakeHome, ".config", "staccato", "hooks", string(PostSync))
	writeScript(t, globalEventDir, "global.sh", "#!/bin/sh\necho global >> "+orderFile+"\n")

	projectEventDir := filepath.Join(projectDir, ".staccato", "hooks", string(PostSync))
	writeScript(t, projectEventDir, "project.sh", "#!/bin/sh\necho project >> "+orderFile+"\n")

	r := NewRunner(projectDir)
	err := r.Fire(Context{
		Event:    PostSync,
		RepoPath: projectDir,
		Branch:   "main",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(orderFile)
	if err != nil {
		t.Fatalf("failed to read order file: %v", err)
	}

	expected := "global\nproject\n"
	if string(data) != expected {
		t.Errorf("expected order '%s', got '%s'", expected, string(data))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
