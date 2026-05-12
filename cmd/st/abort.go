package main

import (
	"fmt"

	"github.com/cpave3/staccato/pkg/backup"
	"github.com/cpave3/staccato/pkg/restack"
	"github.com/spf13/cobra"
)

func abortCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "abort",
		Short: "Abort an in-progress rebase",
		Long:  "Cancels the current rebase operation, clears restack state, and restores from backups if available.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, gitRunner, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			inProgress, err := gitRunner.IsRebaseInProgress()
			if err != nil {
				return fmt.Errorf("failed to check rebase status: %w", err)
			}
			if !inProgress {
				return fmt.Errorf("no rebase in progress — nothing to abort")
			}

			// Abort the rebase
			if err := gitRunner.RebaseAbort(); err != nil {
				return fmt.Errorf("failed to abort rebase: %w", err)
			}

			// Try to restore branches from backups that were created during the restack
			backupMgr := backup.NewManager(gitRunner, repoPath)
			state, stateErr := restack.LoadRestackState(repoPath)
			if stateErr == nil && state != nil && len(state.Lineage) > 0 {
				for _, branch := range state.Lineage {
					if branch == g.Root {
						continue
					}
					backups, _ := backupMgr.ListBackups(branch)
					if len(backups) > 0 {
						if err := backupMgr.RestoreBackup(branch, backups[0]); err != nil {
							printer.Warning("failed to restore %s from backup: %v", branch, err)
						}
					}
					if b, ok := g.GetBranch(branch); ok {
						headSHA, _ := gitRunner.GetCommitSHA(branch)
						baseSHA, _ := gitRunner.GetCommitSHA(b.Parent)
						g.UpdateBranch(branch, baseSHA, headSHA)
					}
				}
			}

			// Clear restack state
			restack.ClearRestackState(repoPath)

			// Save graph
			if err := saveContext(g, repoPath, gitRunner); err != nil {
				return fmt.Errorf("failed to save graph: %w", err)
			}

			printer.Success("Rebase aborted")
			return nil
		},
	}
}
