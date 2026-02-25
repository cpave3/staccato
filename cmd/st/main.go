package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/user/st/pkg/attach"
	"github.com/user/st/pkg/backup"
	"github.com/user/st/pkg/git"
	"github.com/user/st/pkg/graph"
	"github.com/user/st/pkg/output"
	"github.com/user/st/pkg/restack"
)

var (
	verbose bool
	rootCmd *cobra.Command
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd = &cobra.Command{
		Use:   "st",
		Short: "A deterministic, offline-first Git stack management CLI",
		Long: `st is a Git stack management tool inspired by Graphite and Git Town.
		
It provides branch-level stacking with deterministic restacking, automatic backups,
and lazy attachment for retrofitting existing branches.`,
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	// Add subcommands
	rootCmd.AddCommand(newCmd())
	rootCmd.AddCommand(appendCmd())
	rootCmd.AddCommand(insertCmd())
	rootCmd.AddCommand(restackCmd())
	rootCmd.AddCommand(continueCmd())
	rootCmd.AddCommand(attachCmd())
	rootCmd.AddCommand(restoreCmd())
	rootCmd.AddCommand(syncCmd())
	rootCmd.AddCommand(logCmd())
	rootCmd.AddCommand(switchCmd())
}

// getContext loads the graph and git runner for commands
func getContext() (*graph.Graph, *git.Runner, *output.Printer, string, error) {
	printer := output.NewPrinter(verbose)

	// Find git repository root
	gitRunner := git.NewRunner("")
	repoPath, err := gitRunner.Run("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("not a git repository")
	}

	gitRunner = git.NewRunner(repoPath)

	// Load or create graph
	graphPath := filepath.Join(repoPath, graph.GetDefaultGraphPath())
	g, err := graph.LoadGraph(graphPath)
	if err != nil {
		// Graph doesn't exist yet, try to create with current branch as root
		currentBranch, err := gitRunner.GetCurrentBranch()
		if err != nil {
			return nil, nil, nil, "", fmt.Errorf("failed to get current branch: %w", err)
		}
		g = graph.NewGraph(currentBranch)
	}

	return g, gitRunner, printer, repoPath, nil
}

// saveContext saves the graph
func saveContext(g *graph.Graph, repoPath string) error {
	graphPath := filepath.Join(repoPath, graph.GetDefaultGraphPath())
	return g.Save(graphPath)
}

func newCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <branch-name>",
		Short: "Create a new branch from the current root/trunk",
		Long:  "Creates a new branch from the current root branch and adds it to the stack graph.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branchName := args[0]

			g, git, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			// Create branch from root
			err = git.CreateAndCheckoutBranch(branchName)
			if err != nil {
				return fmt.Errorf("failed to create branch: %w", err)
			}

			// Get SHAs
			baseSHA, _ := git.GetCommitSHA(g.Root)
			headSHA, _ := git.GetCommitSHA(branchName)

			// Add to graph
			g.AddBranch(branchName, g.Root, baseSHA, headSHA)

			// Save graph
			if err := saveContext(g, repoPath); err != nil {
				return fmt.Errorf("failed to save graph: %w", err)
			}

			printer.BranchCreated(branchName, g.Root)

			return nil
		},
	}
}

func appendCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "append <branch-name>",
		Short: "Create a child branch from the current branch",
		Long:  "Creates a new branch from the current branch and adds it as a child in the stack.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branchName := args[0]

			g, git, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			parentBranch, _ := git.GetCurrentBranch()

			// If current branch is not in graph and not root, we need to attach it first
			if parentBranch != g.Root {
				if _, exists := g.GetBranch(parentBranch); !exists {
					return fmt.Errorf("current branch '%s' is not in the stack. Run 'st attach' first", parentBranch)
				}
			}

			// Create branch
			err = git.CreateAndCheckoutBranch(branchName)
			if err != nil {
				return fmt.Errorf("failed to create branch: %w", err)
			}

			// Get SHAs
			baseSHA, _ := git.GetCommitSHA(parentBranch)
			headSHA, _ := git.GetCommitSHA(branchName)

			// Add to graph
			g.AddBranch(branchName, parentBranch, baseSHA, headSHA)

			// Save graph
			if err := saveContext(g, repoPath); err != nil {
				return fmt.Errorf("failed to save graph: %w", err)
			}

			printer.BranchCreated(branchName, parentBranch)

			return nil
		},
	}
}

func insertCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "insert <branch-name>",
		Short: "Insert a branch before the current branch",
		Long: `Inserts a new branch before the current branch in the stack.
The current branch and all downstream branches will be reparented and restacked.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branchName := args[0]

			g, git, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

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
			if err := saveContext(g, repoPath); err != nil {
				return fmt.Errorf("failed to save graph: %w", err)
			}

			printer.BranchInserted(branchName, currentBranch)

			// Now restack downstream branches
			printer.Println("Restacking downstream branches...")

			engine := restack.NewEngine(git, backupMgr)
			result, err := engine.Restack(g, branchName)

			if err != nil {
				if result.Conflicts {
					printer.ConflictDetected(result.ConflictsAt)
					return fmt.Errorf("conflict during restack")
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

			return nil
		},
	}
}

func restackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restack",
		Short: "Restack the entire stack",
		Long: `Rebases all branches in the stack onto their parents in topological order.
Creates backups before any destructive operations. Stops on first conflict.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, git, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			currentBranch, _ := git.GetCurrentBranch()

			// Check if current branch is in the stack
			if currentBranch != g.Root {
				if _, exists := g.GetBranch(currentBranch); !exists {
					return fmt.Errorf("current branch '%s' is not in the stack", currentBranch)
				}
			}

			// Find root of current stack
			attacher := attach.NewAttacher(git, printer)
			rootBranch := attacher.FindRoot(g, currentBranch)
			if rootBranch == "" {
				rootBranch = g.Root
			}

			printer.RestackStart(rootBranch)

			// Create backup manager
			backupMgr := backup.NewManager(git, repoPath)

			// Perform restack
			engine := restack.NewEngine(git, backupMgr)
			result, err := engine.Restack(g, rootBranch)

			// Save graph state (even if there was an error)
			saveContext(g, repoPath)

			if err != nil {
				if result.Conflicts {
					printer.ConflictDetected(result.ConflictsAt)
					return fmt.Errorf("conflict during restack - resolve and run 'st continue'")
				}

				// Check if we should restore
				if len(result.Backups) > 0 {
					printer.Error("Restack failed, run 'st restore' to recover")
				}
				return err
			}

			// Cleanup backups on success
			if len(result.Backups) > 0 {
				stackBranches := restack.GetStackBranches(g, rootBranch)
				backupMgr.CleanupStackBackups(stackBranches)
			}

			printer.RestackComplete(len(result.Completed))

			return nil
		},
	}
}

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
				return fmt.Errorf("no rebase in progress - nothing to continue")
			}

			backupMgr := backup.NewManager(git, repoPath)
			engine := restack.NewEngine(git, backupMgr)

			currentBranch, _ := git.GetCurrentBranch()
			attacher := attach.NewAttacher(git, printer)
			rootBranch := attacher.FindRoot(g, currentBranch)
			if rootBranch == "" {
				rootBranch = g.Root
			}

			// Continue the restack
			result, err := engine.Continue(g, rootBranch, nil)

			// Save graph state
			saveContext(g, repoPath)

			if err != nil {
				if result.Conflicts {
					printer.ConflictDetected(result.ConflictsAt)
					return fmt.Errorf("still have conflicts to resolve")
				}
				return err
			}

			printer.RestackComplete(len(result.Completed))

			return nil
		},
	}
}

func attachCmd() *cobra.Command {
	var autoSelect bool

	cmd := &cobra.Command{
		Use:   "attach [branch-name]",
		Short: "Adopt an unknown branch into the stack",
		Long: `Attaches a branch that was created outside of st to the stack graph.
If no branch is specified, uses the current branch.
Prompts for parent branch selection unless --auto is used.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, git, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			var branchName string
			if len(args) > 0 {
				branchName = args[0]
			} else {
				branchName, _ = git.GetCurrentBranch()
			}

			attacher := attach.NewAttacher(git, printer)

			// Check if already attached
			if attacher.IsBranchInGraph(g, branchName) {
				printer.Info("Branch '%s' is already in the stack", branchName)
				return nil
			}

			// Try to auto-attach
			err = attacher.AutoAttach(g, branchName, autoSelect)
			if err != nil {
				// Manual selection needed
				candidates, _ := attacher.SuggestParents(g, branchName)
				printer.AttachPrompt(branchName, candidates)
				return fmt.Errorf("manual parent selection required")
			}

			// Save graph
			if err := saveContext(g, repoPath); err != nil {
				return fmt.Errorf("failed to save graph: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&autoSelect, "auto", false, "Automatically select the best parent candidate")

	return cmd
}

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

func syncCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Push branches to remote",
		Long: `Explicitly pushes all branches in the stack to the remote.
This is an offline-first tool, so push is always explicit.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, git, printer, _, err := getContext()
			if err != nil {
				return err
			}

			if dryRun {
				printer.DryRunNotice()
			}

			// Check if we have a remote
			hasRemote, _ := git.HasRemote()
			if !hasRemote {
				return fmt.Errorf("no remote configured")
			}

			currentBranch, _ := git.GetCurrentBranch()
			attacher := attach.NewAttacher(git, printer)
			rootBranch := attacher.FindRoot(g, currentBranch)
			if rootBranch == "" {
				rootBranch = g.Root
			}

			// Get all branches in stack (excluding root)
			branches := restack.GetStackBranches(g, rootBranch)

			// Push each branch
			pushedCount := 0
			for _, branch := range branches {
				if branch == g.Root {
					continue
				}

				if dryRun {
					printer.Info("Would push: %s", branch)
				} else {
					err := git.Push(branch, false)
					if err != nil {
						printer.Error("Failed to push %s: %v", branch, err)
					} else {
						printer.Info("Pushed: %s", branch)
						pushedCount++
					}
				}
			}

			printer.SyncComplete(pushedCount, dryRun)

			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be pushed without pushing")

	return cmd
}

func logCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "log",
		Short: "Display stack hierarchy",
		Long:  "Shows the current stack structure as a tree.",
		RunE: func(cmd *cobra.Command, args []string) error {
			g, git, printer, _, err := getContext()
			if err != nil {
				return err
			}

			currentBranch, _ := git.GetCurrentBranch()

			printer.StackLog(g, currentBranch)

			return nil
		},
	}
}

func switchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "switch",
		Short: "Interactively switch to a branch in the stack",
		Long: `Displays the stack hierarchy with numbered options and lets you
select a branch to checkout interactively.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, git, printer, _, err := getContext()
			if err != nil {
				return err
			}

			currentBranch, _ := git.GetCurrentBranch()
			attacher := attach.NewAttacher(git, printer)
			rootBranch := attacher.FindRoot(g, currentBranch)
			if rootBranch == "" {
				rootBranch = g.Root
			}

			// Collect all branches in stack with their display info
			type branchOption struct {
				num    int
				name   string
				indent int
			}
			var options []branchOption
			num := 1

			var collectBranches func(branch string, depth int)
			collectBranches = func(branch string, depth int) {
				options = append(options, branchOption{num: num, name: branch, indent: depth})
				num++

				children := g.GetChildren(branch)
				for _, child := range children {
					collectBranches(child.Name, depth+1)
				}
			}

			collectBranches(rootBranch, 0)

			if len(options) == 0 {
				return fmt.Errorf("no branches in stack")
			}

			// Display interactive menu
			printer.Println("")
			printer.Println("Select a branch to switch to:")
			printer.Println("")

			for _, opt := range options {
				indent := ""
				for i := 0; i < opt.indent; i++ {
					indent += "  "
				}

				marker := "○"
				if opt.name == currentBranch {
					marker = "●"
				}

				printer.Println("  [%d] %s%s %s", opt.num, indent, marker, opt.name)
			}

			printer.Println("")
			printer.Print("Enter branch number: ")

			// Read user input
			var choice int
			_, err = fmt.Scanf("%d", &choice)
			if err != nil {
				return fmt.Errorf("invalid input: %w", err)
			}

			// Validate choice
			if choice < 1 || choice > len(options) {
				return fmt.Errorf("invalid selection: %d", choice)
			}

			selectedBranch := options[choice-1].name

			// Checkout the selected branch
			err = git.CheckoutBranch(selectedBranch)
			if err != nil {
				return fmt.Errorf("failed to checkout %s: %w", selectedBranch, err)
			}

			if selectedBranch == currentBranch {
				printer.Info("Already on '%s'", selectedBranch)
			} else {
				printer.Success("Switched to '%s'", selectedBranch)
			}

			return nil
		},
	}
}
