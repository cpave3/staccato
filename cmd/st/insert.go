package main

import (
	"fmt"

	"github.com/cpave3/staccato/pkg/backup"
	"github.com/cpave3/staccato/pkg/hooks"
	"github.com/cpave3/staccato/pkg/restack"
	"github.com/spf13/cobra"
)

func insertCmd() *cobra.Command {
	return withArgs(&cobra.Command{
		Use:   "insert <branch>",
		Short: "Insert an empty branch below the current branch",
		Long: `Creates <branch> on the current branch's parent, makes the current branch its
child, restacks the current branch and everything above it, and checks out the
new branch. Use it to put a change underneath work that already exists.

The new branch starts with no commits of its own. After committing to it, run
'st top' and 'st restack' to rebase the branches above onto it.

The affected branches are backed up first. If the restack stops on a conflict,
resolve it and run 'st continue'.`,
		Example: `st insert feature/schema`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branchName := args[0]

			g, git, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			if err := requireBranch(git); err != nil {
				return err
			}

			checkStaleness(g, git, printer)

			warnDirtyTree(git, printer)

			currentBranch, _ := git.GetCurrentBranch()

			// Get current branch's parent
			currentBranchInfo, exists := g.GetBranch(currentBranch)
			if !exists {
				return fmt.Errorf("current branch '%s' is not in the stack", currentBranch)
			}

			oldParent := currentBranchInfo.Parent

			// Create backup manager
			backupMgr := backup.NewManager(git, repoPath)

			// Create backups of all affected branches
			downstreamBranches := restack.GetDownstreamBranches(g, currentBranch)
			affectedBranches := append([]string{currentBranch}, downstreamBranches...)

			backups, err := backupMgr.CreateBackupsForStack(affectedBranches)
			if err != nil {
				return fmt.Errorf("failed to create backups: %w", err)
			}

			// Create new branch from old parent
			err = git.CheckoutBranch(oldParent)
			if err != nil {
				return fmt.Errorf("failed to checkout parent: %w", err)
			}

			err = git.CreateAndCheckoutBranch(branchName)
			if err != nil {
				return fmt.Errorf("failed to create branch: %w", err)
			}

			// Get SHAs
			baseSHA, _ := git.GetCommitSHA(oldParent)
			headSHA, _ := git.GetCommitSHA(branchName)

			// Add new branch to graph
			g.AddBranch(branchName, oldParent, baseSHA, headSHA)

			// Reparent current branch to new branch
			g.Branches[currentBranch].Parent = branchName

			// Save graph first
			if err := saveContext(g, repoPath, git); err != nil {
				return fmt.Errorf("failed to save graph: %w", err)
			}

			printer.BranchInserted(branchName, currentBranch)

			// Now restack downstream branches
			printer.Println("Restacking downstream branches...")

			engine := restack.NewEngine(git, backupMgr)
			result, err := engine.Restack(g, branchName)
			if err != nil {
				if result.Conflicts {
					// Save restack state so st continue knows which branches to continue
					restack.SaveRestackState(git, &restack.RestackState{
						Lineage: affectedBranches,
					})
					printer.ConflictDetected(result.ConflictsAt)
					return fmt.Errorf("conflict during restack at '%s' — resolve the conflicts and run 'st continue'", result.ConflictsAt)
				}

				// Restore backups on error
				printer.Error("Restack failed, restoring from backups...")
				backupMgr.RestoreStack(backups)
				return err
			}

			// Cleanup backups
			backupMgr.CleanupStackBackups(affectedBranches)

			printer.RestackComplete(len(result.Completed))

			// Checkout the newly inserted branch
			git.CheckoutBranch(branchName)

			hooks.NewRunner(repoPath).Fire(hooks.Context{
				Event:    hooks.PostBranchCreate,
				RepoPath: repoPath,
				Branch:   branchName,
				Data:     map[string]any{"parent": oldParent},
			})

			return nil
		},
	}, argDoc{"branch", "Name of the branch to create"})
}
