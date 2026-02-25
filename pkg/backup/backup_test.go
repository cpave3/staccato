package backup

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/user/st/pkg/git"
)

func TestBackupManager_CanCreateBackup(t *testing.T) {
	tmpDir := t.TempDir()

	// Init git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	gitRunner := git.NewRunner(tmpDir)
	manager := NewManager(gitRunner, tmpDir)

	backupName, err := manager.CreateBackup("main")
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	if !strings.HasPrefix(backupName, "backup/main/") {
		t.Errorf("expected backup name to start with backup/main/, got: %s", backupName)
	}

	// Verify backup branch exists
	exists, _ := gitRunner.BranchExists(backupName)
	if !exists {
		t.Error("expected backup branch to exist")
	}
}

func TestBackupManager_CanRestoreBackup(t *testing.T) {
	tmpDir := t.TempDir()

	// Init git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("original"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	gitRunner := git.NewRunner(tmpDir)
	manager := NewManager(gitRunner, tmpDir)

	// Create backup
	backupName, _ := manager.CreateBackup("main")
	originalSHA, _ := gitRunner.GetBranchSHA("main")

	// Make a new commit
	os.WriteFile(testFile, []byte("changed"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "change").Run()

	newSHA, _ := gitRunner.GetBranchSHA("main")
	if newSHA == originalSHA {
		t.Error("expected commit to change SHA")
	}

	// Restore backup
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
	tmpDir := t.TempDir()

	// Init git repo
	cmd := exec.Command("git", "init")
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

	// Create two backups
	manager.CreateBackup("main")
	time.Sleep(10 * time.Millisecond) // Ensure different timestamps
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
	tmpDir := t.TempDir()

	// Init git repo
	cmd := exec.Command("git", "init")
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

	backupName, _ := manager.CreateBackup("main")

	// Verify backup exists
	exists, _ := gitRunner.BranchExists(backupName)
	if !exists {
		t.Fatal("expected backup to exist before deletion")
	}

	// Delete backup
	err := manager.DeleteBackup(backupName)
	if err != nil {
		t.Fatalf("failed to delete backup: %v", err)
	}

	// Verify backup no longer exists
	exists, _ = gitRunner.BranchExists(backupName)
	if exists {
		t.Error("expected backup to be deleted")
	}
}

func TestBackupManager_CleanupOldBackups(t *testing.T) {
	tmpDir := t.TempDir()

	// Init git repo
	cmd := exec.Command("git", "init")
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

	// Create 5 backups
	for i := 0; i < 5; i++ {
		manager.CreateBackup("main")
		time.Sleep(10 * time.Millisecond)
	}

	backups, _ := manager.ListBackups("main")
	if len(backups) != 5 {
		t.Fatalf("expected 5 backups, got: %d", len(backups))
	}

	// Keep only 3 most recent
	err := manager.CleanupOldBackups("main", 3)
	if err != nil {
		t.Fatalf("failed to cleanup backups: %v", err)
	}

	backups, _ = manager.ListBackups("main")
	if len(backups) != 3 {
		t.Errorf("expected 3 backups after cleanup, got: %d", len(backups))
	}
}
