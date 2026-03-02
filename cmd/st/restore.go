package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/cpave3/staccato/pkg/attach"
	"github.com/cpave3/staccato/pkg/backup"
	"github.com/cpave3/staccato/pkg/restack"
)

func restoreCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "restore [branch-name]",
		Short: "Restore branch(es) from backup",
		Long: `Restores a branch or all branches from their backups.
Use this to recover from failed restack operations.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, git, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			backupMgr := backup.NewManager(git, repoPath)

			if all {
				// Abort any in-progress rebase first (git refuses checkout during rebase)
				inProgress, _ := git.IsRebaseInProgress()
				if inProgress {
					if err := git.RebaseAbort(); err != nil {
						printer.Error("Failed to abort in-progress rebase: %v", err)
						return fmt.Errorf("failed to abort rebase: %w", err)
					}
				}

				// Restore all branches in stack
				currentBranch, _ := git.GetCurrentBranch()
				attacher := attach.NewAttacher(git, printer)
				rootBranch := attacher.FindRoot(g, currentBranch)
				if rootBranch == "" {
					rootBranch = g.Root
				}

				branches := restack.GetStackBranches(g, rootBranch)
				for _, branch := range branches {
					backups, _ := backupMgr.ListBackups(branch)
					if len(backups) > 0 {
						err := backupMgr.RestoreBackup(branch, backups[0])
						if err != nil {
							printer.Error("Failed to restore %s: %v", branch, err)
						} else {
							printer.BackupRestored(branch)
						}
					}
				}

				// Update graph state to match restored branch SHAs
				for _, branch := range branches {
					if branch == g.Root {
						continue
					}
					b, exists := g.GetBranch(branch)
					if !exists {
						continue
					}
					newHeadSHA, err := git.GetCommitSHA(branch)
					if err != nil {
						continue
					}
					newBaseSHA, err := git.GetCommitSHA(b.Parent)
					if err != nil {
						continue
					}
					g.UpdateBranch(branch, newBaseSHA, newHeadSHA)
				}
				saveContext(g, repoPath, git)

				// Clean up restack state
				restack.ClearRestackState(repoPath)
			} else {
				// Restore specific branch
				var branchName string
				if len(args) > 0 {
					branchName = args[0]
				} else {
					branchName, _ = git.GetCurrentBranch()
				}

				backups, err := backupMgr.ListBackups(branchName)
				if err != nil || len(backups) == 0 {
					return fmt.Errorf("no backups found for branch '%s'", branchName)
				}

				err = backupMgr.RestoreBackup(branchName, backups[0])
				if err != nil {
					return fmt.Errorf("failed to restore backup: %w", err)
				}

				printer.BackupRestored(branchName)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Restore all branches in the stack")

	return cmd
}
