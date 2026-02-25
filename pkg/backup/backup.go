package backup

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/user/st/pkg/git"
)

const (
	BackupPrefix = "backup"
)

// Manager handles creation, restoration, and cleanup of branch backups
type Manager struct {
	git      *git.Runner
	repoPath string
}

// NewManager creates a new backup manager
func NewManager(git *git.Runner, repoPath string) *Manager {
	return &Manager{
		git:      git,
		repoPath: repoPath,
	}
}

// CreateBackup creates a backup of the specified branch
// Returns the backup branch name
func (m *Manager) CreateBackup(branchName string) (string, error) {
	timestamp := time.Now().UnixNano()
	backupName := fmt.Sprintf("%s/%s/%d", BackupPrefix, branchName, timestamp)

	// Create backup branch
	err := m.git.CopyBranch(branchName, backupName)
	if err != nil {
		return "", fmt.Errorf("failed to create backup branch: %w", err)
	}

	return backupName, nil
}

// RestoreBackup restores a branch from a backup
func (m *Manager) RestoreBackup(branchName, backupName string) error {
	// Check if backup exists
	exists, err := m.git.BranchExists(backupName)
	if err != nil {
		return fmt.Errorf("failed to check backup existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("backup %s does not exist", backupName)
	}

	// Get current branch
	currentBranch, err := m.git.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// If we're on the branch to restore, switch away first
	if currentBranch == branchName {
		// Try to switch to the backup branch temporarily
		err = m.git.CheckoutBranch(backupName)
		if err != nil {
			return fmt.Errorf("failed to checkout backup branch: %w", err)
		}
	}

	// Delete the original branch
	err = m.git.DeleteBranch(branchName, true)
	if err != nil {
		// Try to restore original state
		m.git.CheckoutBranch(currentBranch)
		return fmt.Errorf("failed to delete original branch: %w", err)
	}

	// Rename backup to original name
	err = m.git.CopyBranch(backupName, branchName)
	if err != nil {
		return fmt.Errorf("failed to rename backup: %w", err)
	}

	// Switch back to the restored branch
	err = m.git.CheckoutBranch(branchName)
	if err != nil {
		return fmt.Errorf("failed to checkout restored branch: %w", err)
	}

	// Delete the backup branch (now that we've copied it)
	m.git.DeleteBranch(backupName, true)

	return nil
}

// ListBackups returns all backups for a given branch
func (m *Manager) ListBackups(branchName string) ([]string, error) {
	pattern := fmt.Sprintf("%s/%s/", BackupPrefix, branchName)

	// Get all branches
	output, err := m.git.Run("branch", "-a")
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	var backups []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		// Remove leading * if present (current branch marker)
		line = strings.TrimPrefix(line, "* ")

		if strings.HasPrefix(line, pattern) {
			backups = append(backups, line)
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(backups, func(i, j int) bool {
		tsi := m.extractTimestamp(backups[i])
		tsj := m.extractTimestamp(backups[j])
		return tsi > tsj
	})

	return backups, nil
}

// DeleteBackup removes a specific backup
func (m *Manager) DeleteBackup(backupName string) error {
	exists, err := m.git.BranchExists(backupName)
	if err != nil {
		return fmt.Errorf("failed to check backup existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("backup %s does not exist", backupName)
	}

	err = m.git.DeleteBranch(backupName, true)
	if err != nil {
		return fmt.Errorf("failed to delete backup: %w", err)
	}

	return nil
}

// CleanupOldBackups removes old backups keeping only the specified number of most recent
func (m *Manager) CleanupOldBackups(branchName string, keep int) error {
	backups, err := m.ListBackups(branchName)
	if err != nil {
		return err
	}

	if len(backups) <= keep {
		return nil
	}

	// Delete oldest backups
	for i := keep; i < len(backups); i++ {
		err := m.DeleteBackup(backups[i])
		if err != nil {
			return fmt.Errorf("failed to delete backup %s: %w", backups[i], err)
		}
	}

	return nil
}

// CreateBackupsForStack creates backups for all branches in a stack
func (m *Manager) CreateBackupsForStack(branches []string) (map[string]string, error) {
	backups := make(map[string]string)

	for _, branch := range branches {
		backupName, err := m.CreateBackup(branch)
		if err != nil {
			return backups, fmt.Errorf("failed to backup branch %s: %w", branch, err)
		}
		backups[branch] = backupName
	}

	return backups, nil
}

// RestoreStack restores all branches in a stack from their backups
func (m *Manager) RestoreStack(backups map[string]string) error {
	for branch, backup := range backups {
		err := m.RestoreBackup(branch, backup)
		if err != nil {
			return fmt.Errorf("failed to restore branch %s: %w", branch, err)
		}
	}
	return nil
}

// CleanupStackBackups removes all backups for branches in a stack
func (m *Manager) CleanupStackBackups(branches []string) error {
	for _, branch := range branches {
		backups, err := m.ListBackups(branch)
		if err != nil {
			return fmt.Errorf("failed to list backups for %s: %w", branch, err)
		}

		for _, backup := range backups {
			err := m.DeleteBackup(backup)
			if err != nil {
				return fmt.Errorf("failed to delete backup %s: %w", backup, err)
			}
		}
	}
	return nil
}

// extractTimestamp extracts the timestamp from a backup name
func (m *Manager) extractTimestamp(backupName string) int64 {
	parts := strings.Split(backupName, "/")
	if len(parts) < 3 {
		return 0
	}

	ts, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	if err != nil {
		return 0
	}
	return ts
}

// CreateManualBackup creates a manual snapshot of the given branches.
// Unlike automatic backups, manual backups use the "backups/" prefix (plural)
// and are not auto-deleted on success.
// Returns the timestamp label used in the backup branch names.
func (m *Manager) CreateManualBackup(branches []string) (string, error) {
	timestamp := time.Now().Format("2006-01-02_15-04-05")

	for _, branch := range branches {
		backupName := fmt.Sprintf("backups/%s/%s", timestamp, branch)
		err := m.git.CopyBranch(branch, backupName)
		if err != nil {
			return timestamp, fmt.Errorf("failed to backup branch %s: %w", branch, err)
		}
	}

	return timestamp, nil
}

// GetBackupPath returns the default path for storing backup metadata
func GetBackupPath(repoPath string) string {
	return filepath.Join(repoPath, ".git", "stack", "backups")
}
