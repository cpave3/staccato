package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/cpave3/staccato/pkg/backup"
	"github.com/cpave3/staccato/pkg/restack"
)

func continueCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "continue",
		Short: "Resume restack after conflict resolution",
		Long:  "Continues a restack operation that was paused due to conflicts.",
		RunE: func(cmd *cobra.Command, args []string) error {
			g, git, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			// Check if rebase is in progress
			inProgress, err := git.IsRebaseInProgress()
			if err != nil {
				return fmt.Errorf("failed to check rebase status: %w", err)
			}

			if !inProgress {
				return fmt.Errorf("no rebase in progress — nothing to continue")
			}

			backupMgr := backup.NewManager(git, repoPath)
			engine := restack.NewEngine(git, backupMgr)

			// Load restack state to get lineage info
			var lineage []string
			state, stateErr := restack.LoadRestackState(repoPath)
			if stateErr != nil || state == nil {
				// Rebase in progress but no st restack state — likely a manual rebase
				return fmt.Errorf("no st restack in progress — did you mean 'git rebase --continue'?")
			}
			lineage = state.Lineage

			// Continue the restack (uses lineage if available)
			result, err := engine.Continue(g, lineage)

			// Save graph state
			saveContext(g, repoPath, git)

			if err != nil {
				if result.Conflicts {
					printer.ConflictDetected(result.ConflictsAt)
					return fmt.Errorf("still have conflicts to resolve")
				}
				return err
			}

			// Clean up restack state and backups on success
			restack.ClearRestackState(repoPath)
			if lineage != nil {
				backupMgr.CleanupStackBackups(lineage)
			}

			printer.RestackComplete(len(result.Completed))

			return nil
		},
	}
}
