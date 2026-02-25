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
