package backup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cpave3/staccato/pkg/git"
)

// initTestRepo creates a temporary git repo with an initial commit and returns the path, runner, and manager.
func initTestRepo(t *testing.T) (string, *git.Runner, *Manager) {
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

	gitRunner := git.NewRunner(tmpDir)
	manager := NewManager(gitRunner, tmpDir)
	return tmpDir, gitRunner, manager
}

func TestBackupManager_CanCreateBackup(t *testing.T) {
	_, gitRunner, manager := initTestRepo(t)

	backupName, err := manager.CreateBackup("main")
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	if !strings.HasPrefix(backupName, "backup/auto/main/") {
		t.Errorf("expected backup name to start with backup/auto/main/, got: %s", backupName)
	}

	exists, _ := gitRunner.BranchExists(backupName)
	if !exists {
		t.Error("expected backup branch to exist")
	}
}

func TestBackupManager_CanRestoreBackup(t *testing.T) {
	tmpDir, gitRunner, manager := initTestRepo(t)

	backupName, _ := manager.CreateBackup("main")
	originalSHA, _ := gitRunner.GetBranchSHA("main")

	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("changed"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "change").Run()

	newSHA, _ := gitRunner.GetBranchSHA("main")
	if newSHA == originalSHA {
		t.Error("expected commit to change SHA")
	}

	err := manager.RestoreBackup("main", backupName)
	if err != nil {
		t.Fatalf("failed to restore backup: %v", err)
	}

	restoredSHA, _ := gitRunner.GetBranchSHA("main")
	if restoredSHA != originalSHA {
		t.Errorf("expected restored SHA to match original. Original: %s, Restored: %s", originalSHA, restoredSHA)
	}
}

func TestBackupManager_CanListBackups(t *testing.T) {
	_, _, manager := initTestRepo(t)

	manager.CreateBackup("main")
	time.Sleep(10 * time.Millisecond)
	manager.CreateBackup("main")

	backups, err := manager.ListBackups("main")
	if err != nil {
		t.Fatalf("failed to list backups: %v", err)
	}

	if len(backups) != 2 {
		t.Errorf("expected 2 backups, got: %d", len(backups))
	}
}

func TestBackupManager_CanDeleteBackup(t *testing.T) {
	_, gitRunner, manager := initTestRepo(t)

	backupName, _ := manager.CreateBackup("main")

	exists, _ := gitRunner.BranchExists(backupName)
	if !exists {
		t.Fatal("expected backup to exist before deletion")
	}

	err := manager.DeleteBackup(backupName)
	if err != nil {
		t.Fatalf("failed to delete backup: %v", err)
	}

	exists, _ = gitRunner.BranchExists(backupName)
	if exists {
		t.Error("expected backup to be deleted")
	}
}

func TestBackupManager_CleanupOldBackups(t *testing.T) {
	_, _, manager := initTestRepo(t)

	for i := 0; i < 5; i++ {
		manager.CreateBackup("main")
		time.Sleep(10 * time.Millisecond)
	}

	backups, _ := manager.ListBackups("main")
	if len(backups) != 5 {
		t.Fatalf("expected 5 backups, got: %d", len(backups))
	}

	err := manager.CleanupOldBackups("main", 3)
	if err != nil {
		t.Fatalf("failed to cleanup backups: %v", err)
	}

	backups, _ = manager.ListBackups("main")
	if len(backups) != 3 {
		t.Errorf("expected 3 backups after cleanup, got: %d", len(backups))
	}
}

func TestBackupManager_ListBackupsMatchesLegacy(t *testing.T) {
	tmpDir, gitRunner, manager := initTestRepo(t)

	// Create a legacy-format backup manually: backup/<branch>/<nano-ts>
	ts := time.Now().UnixNano()
	legacyName := fmt.Sprintf("backup/main/%d", ts)
	exec.Command("git", "-C", tmpDir, "branch", legacyName).Run()

	exists, _ := gitRunner.BranchExists(legacyName)
	if !exists {
		t.Fatal("failed to create legacy backup branch")
	}

	// Also create a new-format backup
	manager.CreateBackup("main")

	backups, err := manager.ListBackups("main")
	if err != nil {
		t.Fatalf("failed to list backups: %v", err)
	}

	if len(backups) != 2 {
		t.Errorf("expected 2 backups (legacy + new), got: %d", len(backups))
	}
}

func TestParseBackupBranch_NewAuto(t *testing.T) {
	name := "backup/auto/feat/mcp/1709000000000000000"
	info, ok := parseBackupBranch(name)
	if !ok {
		t.Fatal("expected parse to succeed")
	}
	if info.Kind != BackupAuto {
		t.Errorf("expected auto, got %s", info.Kind)
	}
	if info.SourceBranch != "feat/mcp" {
		t.Errorf("expected feat/mcp, got %s", info.SourceBranch)
	}
	if info.BranchRef != name {
		t.Errorf("expected BranchRef to be %s, got %s", name, info.BranchRef)
	}
}

func TestParseBackupBranch_NewManual(t *testing.T) {
	name := "backup/manual/2026-02-27_16-32-07/feat/mcp"
	info, ok := parseBackupBranch(name)
	if !ok {
		t.Fatal("expected parse to succeed")
	}
	if info.Kind != BackupManual {
		t.Errorf("expected manual, got %s", info.Kind)
	}
	if info.SourceBranch != "feat/mcp" {
		t.Errorf("expected feat/mcp, got %s", info.SourceBranch)
	}
	expected := time.Date(2026, 2, 27, 16, 32, 7, 0, time.UTC)
	if !info.Timestamp.Equal(expected) {
		t.Errorf("expected timestamp %v, got %v", expected, info.Timestamp)
	}
}

func TestParseBackupBranch_LegacyAuto(t *testing.T) {
	name := "backup/feat/mcp/1709000000000000000"
	info, ok := parseBackupBranch(name)
	if !ok {
		t.Fatal("expected parse to succeed")
	}
	if info.Kind != BackupAuto {
		t.Errorf("expected auto, got %s", info.Kind)
	}
	if info.SourceBranch != "feat/mcp" {
		t.Errorf("expected feat/mcp, got %s", info.SourceBranch)
	}
}

func TestParseBackupBranch_LegacyManual(t *testing.T) {
	name := "backups/2026-02-27_16-32-07/feat/mcp"
	info, ok := parseBackupBranch(name)
	if !ok {
		t.Fatal("expected parse to succeed")
	}
	if info.Kind != BackupManual {
		t.Errorf("expected manual, got %s", info.Kind)
	}
	if info.SourceBranch != "feat/mcp" {
		t.Errorf("expected feat/mcp, got %s", info.SourceBranch)
	}
}

func TestParseBackupBranch_SimpleBranch(t *testing.T) {
	name := "backup/auto/main/1709000000000000000"
	info, ok := parseBackupBranch(name)
	if !ok {
		t.Fatal("expected parse to succeed")
	}
	if info.SourceBranch != "main" {
		t.Errorf("expected main, got %s", info.SourceBranch)
	}
}

func TestParseBackupBranch_Invalid(t *testing.T) {
	cases := []string{
		"main",
		"backup",
		"backup/auto/",
		"backup/manual/",
		"backup/auto/main/notanumber",
		"backup/manual/bad-format/main",
		"other/prefix/main/123",
	}
	for _, c := range cases {
		_, ok := parseBackupBranch(c)
		if ok {
			t.Errorf("expected parse to fail for %q", c)
		}
	}
}

func TestBackupManager_ListAllBackups(t *testing.T) {
	tmpDir, _, manager := initTestRepo(t)

	// Create a new auto backup
	manager.CreateBackup("main")

	// Create a legacy auto backup
	legacyTs := time.Now().Add(-time.Hour).UnixNano()
	legacyName := fmt.Sprintf("backup/main/%d", legacyTs)
	exec.Command("git", "-C", tmpDir, "branch", legacyName).Run()

	// Create a new manual backup
	manager.CreateManualBackup([]string{"main"})

	// Create a legacy manual backup
	legacyManual := "backups/2025-01-01_12-00-00/main"
	exec.Command("git", "-C", tmpDir, "branch", legacyManual).Run()

	all, err := manager.ListAllBackups()
	if err != nil {
		t.Fatalf("failed to list all backups: %v", err)
	}

	if len(all) != 4 {
		t.Errorf("expected 4 backups, got %d", len(all))
		for _, b := range all {
			t.Logf("  %s (%s) %s", b.BranchRef, b.Kind, b.Timestamp)
		}
	}

	// Verify sorted newest-first
	for i := 1; i < len(all); i++ {
		if all[i].Timestamp.After(all[i-1].Timestamp) {
			t.Errorf("expected sorted newest-first, but index %d (%v) is after index %d (%v)",
				i, all[i].Timestamp, i-1, all[i-1].Timestamp)
		}
	}
}

func TestBackupManager_DeleteBackups(t *testing.T) {
	_, gitRunner, manager := initTestRepo(t)

	manager.CreateBackup("main")
	time.Sleep(10 * time.Millisecond)
	manager.CreateBackup("main")

	all, _ := manager.ListAllBackups()
	if len(all) != 2 {
		t.Fatalf("expected 2 backups, got %d", len(all))
	}

	deleted, err := manager.DeleteBackups(all)
	if err != nil {
		t.Fatalf("failed to delete backups: %v", err)
	}
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}

	// Verify they're gone
	for _, b := range all {
		exists, _ := gitRunner.BranchExists(b.BranchRef)
		if exists {
			t.Errorf("expected %s to be deleted", b.BranchRef)
		}
	}
}

func TestRestoreStack(t *testing.T) {
	tmpDir, gitRunner, manager := initTestRepo(t)

	// Create two feature branches
	exec.Command("git", "-C", tmpDir, "checkout", "-b", "f1").Run()
	os.WriteFile(filepath.Join(tmpDir, "f1.txt"), []byte("f1"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "f1 commit").Run()

	exec.Command("git", "-C", tmpDir, "checkout", "main").Run()
	exec.Command("git", "-C", tmpDir, "checkout", "-b", "f2").Run()
	os.WriteFile(filepath.Join(tmpDir, "f2.txt"), []byte("f2"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "f2 commit").Run()

	// Record original SHAs
	origF1, _ := gitRunner.GetBranchSHA("f1")
	origF2, _ := gitRunner.GetBranchSHA("f2")

	// Create backups for both
	backups, err := manager.CreateBackupsForStack([]string{"f1", "f2"})
	if err != nil {
		t.Fatalf("CreateBackupsForStack failed: %v", err)
	}

	// Modify both branches
	exec.Command("git", "-C", tmpDir, "checkout", "f1").Run()
	os.WriteFile(filepath.Join(tmpDir, "f1.txt"), []byte("f1 modified"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "f1 modify").Run()

	exec.Command("git", "-C", tmpDir, "checkout", "f2").Run()
	os.WriteFile(filepath.Join(tmpDir, "f2.txt"), []byte("f2 modified"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "f2 modify").Run()

	// Restore stack
	err = manager.RestoreStack(backups)
	if err != nil {
		t.Fatalf("RestoreStack failed: %v", err)
	}

	// Verify both branches are restored
	restoredF1, _ := gitRunner.GetBranchSHA("f1")
	restoredF2, _ := gitRunner.GetBranchSHA("f2")
	if restoredF1 != origF1 {
		t.Errorf("f1 SHA: expected %s, got %s", origF1, restoredF1)
	}
	if restoredF2 != origF2 {
		t.Errorf("f2 SHA: expected %s, got %s", origF2, restoredF2)
	}
}

func TestGetBackupPath(t *testing.T) {
	path := GetBackupPath("/repo")
	expected := filepath.Join("/repo", ".git", "stack", "backups")
	if path != expected {
		t.Errorf("GetBackupPath = %q, want %q", path, expected)
	}
}

func TestBackupManager_CreateManualBackup_NewNaming(t *testing.T) {
	_, gitRunner, manager := initTestRepo(t)

	ts, err := manager.CreateManualBackup([]string{"main"})
	if err != nil {
		t.Fatalf("failed to create manual backup: %v", err)
	}

	expectedPrefix := fmt.Sprintf("backup/manual/%s/main", ts)
	exists, _ := gitRunner.BranchExists(expectedPrefix)
	if !exists {
		t.Errorf("expected manual backup branch %s to exist", expectedPrefix)
	}
}
