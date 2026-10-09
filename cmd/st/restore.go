package main

import (
	"fmt"

	"github.com/cpave3/staccato/pkg/attach"
	"github.com/cpave3/staccato/pkg/backup"
	"github.com/cpave3/staccato/pkg/restack"
	"github.com/spf13/cobra"
)

func restoreCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "restore [branch]",
		Short: "Put a branch back from its automatic backup",
		Long: `Commands that restack back up the branches they rewrite and remove the backups
when they succeed, so a backup is left only when one failed or was
interrupted. This resets the branch to its most recent such backup and checks
it out; without [branch], the current branch.

--all restores every branch of the current stack that has a backup, after
aborting a rebase that is still in progress.

For a restack that stopped on a conflict, 'st abort' is the usual way back.
Snapshots made by 'st backup' are not used here: see 'st backup --help'.`,
		Example: `st restore
st restore feature/api
st restore --all`,
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
					base, head, err := repairedBranch(git, branch, b)
					if err != nil {
						// Full-stack backups may predate a parent rewrite; after restoring
						// every branch, their shared history is the available fork point.
						base, err = git.GetMergeBase(branch, b.Parent)
						if err != nil {
							return err
						}
						head, err = git.GetCommitSHA(branch)
						if err != nil {
							return err
						}
					}
					g.UpdateBranch(branch, base, head)
				}
				if err := saveContext(g, repoPath, git); err != nil {
					return err
				}

				// Clean up restack state
				restack.ClearRestackState(git)
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
				if b, exists := g.GetBranch(branchName); exists {
					base, head, err := repairedBranch(git, branchName, b)
					if err != nil {
						return err
					}
					g.UpdateBranch(branchName, base, head)
					if err := saveContext(g, repoPath, git); err != nil {
						return err
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Restore every branch of the current stack that has a backup")

	return withArgs(cmd, argDoc{"branch", "Branch to restore (default: the current branch)"})
}
