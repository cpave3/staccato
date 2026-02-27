package backup

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cpave3/staccato/pkg/git"
)

const (
	BackupPrefix      = "backup"
	AutoSubpath       = "auto"
	ManualSubpath     = "manual"
	LegacyManualPrefix = "backups"
)

// BackupKind distinguishes automatic from manual backups.
type BackupKind string

const (
	BackupAuto   BackupKind = "auto"
	BackupManual BackupKind = "manual"
)

// BackupInfo describes a single backup branch.
type BackupInfo struct {
	BranchRef    string     // Full git branch ref
	SourceBranch string     // Original branch name
	Kind         BackupKind
	Timestamp    time.Time
}

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

// CreateBackup creates an automatic backup of the specified branch.
// Returns the backup branch name.
func (m *Manager) CreateBackup(branchName string) (string, error) {
	timestamp := time.Now().UnixNano()
	backupName := fmt.Sprintf("%s/%s/%s/%d", BackupPrefix, AutoSubpath, branchName, timestamp)

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

// ListBackups returns all auto backups for a given branch (newest first).
// It matches both the new backup/auto/<branch>/ and legacy backup/<branch>/ patterns.
func (m *Manager) ListBackups(branchName string) ([]string, error) {
	newPattern := fmt.Sprintf("%s/%s/%s/", BackupPrefix, AutoSubpath, branchName)
	legacyPattern := fmt.Sprintf("%s/%s/", BackupPrefix, branchName)

	output, err := m.git.Run("branch", "-a")
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	var backups []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "* ")

		if strings.HasPrefix(line, newPattern) {
			backups = append(backups, line)
		} else if strings.HasPrefix(line, legacyPattern) {
			// Exclude new-format auto/manual subpaths from legacy match
			rest := strings.TrimPrefix(line, BackupPrefix+"/")
			if !strings.HasPrefix(rest, AutoSubpath+"/") && !strings.HasPrefix(rest, ManualSubpath+"/") {
				backups = append(backups, line)
			}
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
// Manual backups use backup/manual/<timestamp>/<branch> naming.
// Returns the timestamp label used in the backup branch names.
func (m *Manager) CreateManualBackup(branches []string) (string, error) {
	timestamp := time.Now().Format("2006-01-02_15-04-05")

	for _, branch := range branches {
		backupName := fmt.Sprintf("%s/%s/%s/%s", BackupPrefix, ManualSubpath, timestamp, branch)
		err := m.git.CopyBranch(branch, backupName)
		if err != nil {
			return timestamp, fmt.Errorf("failed to backup branch %s: %w", branch, err)
		}
	}

	return timestamp, nil
}

// parseBackupBranch parses a branch name into a BackupInfo.
// It recognizes all 4 formats:
//   - New auto:     backup/auto/<branch>/<nano-ts>
//   - New manual:   backup/manual/<YYYY-MM-DD_HH-MM-SS>/<branch>
//   - Legacy auto:  backup/<branch>/<nano-ts>
//   - Legacy manual: backups/<YYYY-MM-DD_HH-MM-SS>/<branch>
func parseBackupBranch(name string) (BackupInfo, bool) {
	// New auto: backup/auto/<branch...>/<nano-ts>
	if strings.HasPrefix(name, BackupPrefix+"/"+AutoSubpath+"/") {
		rest := strings.TrimPrefix(name, BackupPrefix+"/"+AutoSubpath+"/")
		// Timestamp is always the last segment
		lastSlash := strings.LastIndex(rest, "/")
		if lastSlash < 0 {
			return BackupInfo{}, false
		}
		branch := rest[:lastSlash]
		tsStr := rest[lastSlash+1:]
		nanos, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil || branch == "" {
			return BackupInfo{}, false
		}
		return BackupInfo{
			BranchRef:    name,
			SourceBranch: branch,
			Kind:         BackupAuto,
			Timestamp:    time.Unix(0, nanos),
		}, true
	}

	// New manual: backup/manual/<YYYY-MM-DD_HH-MM-SS>/<branch>
	if strings.HasPrefix(name, BackupPrefix+"/"+ManualSubpath+"/") {
		rest := strings.TrimPrefix(name, BackupPrefix+"/"+ManualSubpath+"/")
		// Timestamp is a fixed-width 19-char segment: YYYY-MM-DD_HH-MM-SS
		if len(rest) < 20 || rest[19] != '/' {
			return BackupInfo{}, false
		}
		tsStr := rest[:19]
		branch := rest[20:]
		t, err := time.Parse("2006-01-02_15-04-05", tsStr)
		if err != nil || branch == "" {
			return BackupInfo{}, false
		}
		return BackupInfo{
			BranchRef:    name,
			SourceBranch: branch,
			Kind:         BackupManual,
			Timestamp:    t,
		}, true
	}

	// Legacy manual: backups/<YYYY-MM-DD_HH-MM-SS>/<branch>
	if strings.HasPrefix(name, LegacyManualPrefix+"/") {
		rest := strings.TrimPrefix(name, LegacyManualPrefix+"/")
		if len(rest) < 20 || rest[19] != '/' {
			return BackupInfo{}, false
		}
		tsStr := rest[:19]
		branch := rest[20:]
		t, err := time.Parse("2006-01-02_15-04-05", tsStr)
		if err != nil || branch == "" {
			return BackupInfo{}, false
		}
		return BackupInfo{
			BranchRef:    name,
			SourceBranch: branch,
			Kind:         BackupManual,
			Timestamp:    t,
		}, true
	}

	// Legacy auto: backup/<branch>/<nano-ts>  (excluding auto/manual subpaths)
	if strings.HasPrefix(name, BackupPrefix+"/") {
		rest := strings.TrimPrefix(name, BackupPrefix+"/")
		if strings.HasPrefix(rest, AutoSubpath+"/") || strings.HasPrefix(rest, ManualSubpath+"/") {
			return BackupInfo{}, false
		}
		lastSlash := strings.LastIndex(rest, "/")
		if lastSlash < 0 {
			return BackupInfo{}, false
		}
		branch := rest[:lastSlash]
		tsStr := rest[lastSlash+1:]
		nanos, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil || branch == "" {
			return BackupInfo{}, false
		}
		return BackupInfo{
			BranchRef:    name,
			SourceBranch: branch,
			Kind:         BackupAuto,
			Timestamp:    time.Unix(0, nanos),
		}, true
	}

	return BackupInfo{}, false
}

// ListAllBackups returns all backup branches (auto and manual, old and new),
// sorted newest-first.
func (m *Manager) ListAllBackups() ([]BackupInfo, error) {
	output, err := m.git.Run("branch", "-a")
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	var backups []BackupInfo
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "* ")
		if line == "" {
			continue
		}

		if info, ok := parseBackupBranch(line); ok {
			backups = append(backups, info)
		}
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	return backups, nil
}

// DeleteBackups deletes the given backup branches and returns the number deleted.
func (m *Manager) DeleteBackups(backups []BackupInfo) (int, error) {
	deleted := 0
	for _, b := range backups {
		err := m.git.DeleteBranch(b.BranchRef, true)
		if err != nil {
			return deleted, fmt.Errorf("failed to delete %s: %w", b.BranchRef, err)
		}
		deleted++
	}
	return deleted, nil
}

// GetBackupPath returns the default path for storing backup metadata
func GetBackupPath(repoPath string) string {
	return filepath.Join(repoPath, ".git", "stack", "backups")
}
