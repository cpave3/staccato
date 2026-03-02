package mcp

import (
	"context"
	"fmt"

	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/cpave3/staccato/pkg/attach"
	"github.com/cpave3/staccato/pkg/backup"
	"github.com/cpave3/staccato/pkg/restack"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerManagementTools(s *server.MCPServer, sc *stcontext.StaccatoContext) {
	// st_attach
	s.AddTool(
		mcp.NewTool("st_attach",
			mcp.WithDescription("Attach a branch to the stack under a specified parent."),
			mcp.WithToolAnnotation(mutating()),
			mcp.WithString("branch_name", mcp.Required(), mcp.Description("Branch to attach")),
			mcp.WithString("parent", mcp.Required(), mcp.Description("Parent branch in the stack")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			branchName, err := req.RequireString("branch_name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			parent, err := req.RequireString("parent")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Validate parent is in the graph or is root
			if parent != sc.Graph.Root {
				if _, exists := sc.Graph.GetBranch(parent); !exists {
					if stcontext.IsTrunkBranch(parent) {
						exists, err := sc.Git.BranchExists(parent)
						if err == nil && exists {
							sc.Graph.Root = parent
						} else {
							return mcp.NewToolResultError(fmt.Sprintf("parent '%s' is not in the stack", parent)), nil
						}
					} else {
						return mcp.NewToolResultError(fmt.Sprintf("parent '%s' is not in the stack", parent)), nil
					}
				}
			}

			attacher := attach.NewAttacher(sc.Git, nil)
			if err := attacher.AttachBranch(sc.Graph, branchName, parent); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to attach: %v", err)), nil
			}

			if err := sc.Save(); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to save graph: %v", err)), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf("Attached '%s' as child of '%s'", branchName, parent)), nil
		},
	)

	// st_restack
	s.AddTool(
		mcp.NewTool("st_restack",
			mcp.WithDescription("Restack branches in the current lineage."),
			mcp.WithToolAnnotation(mutating()),
			mcp.WithBoolean("to_current", mcp.Description("Only restack up to the current branch")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			toCurrent := req.GetBool("to_current", false)

			currentBranch, _ := sc.Git.GetCurrentBranch()
			if currentBranch != sc.Graph.Root {
				if _, exists := sc.Graph.GetBranch(currentBranch); !exists {
					return mcp.NewToolResultError(fmt.Sprintf("current branch '%s' is not in the stack", currentBranch)), nil
				}
			}

			lineageBranches := restack.GetLineage(sc.Graph, currentBranch)
			if !restack.IsBranchAtTip(sc.Graph, currentBranch) {
				if !toCurrent {
					return mcp.NewToolResultError("not at tip of lineage — set to_current=true to restack only up to current branch"), nil
				}
				lineageBranches = restack.GetAncestors(sc.Graph, currentBranch)
			}

			backupMgr := backup.NewManager(sc.Git, sc.RepoPath)
			engine := restack.NewEngine(sc.Git, backupMgr)
			result, err := engine.RestackLineage(sc.Graph, currentBranch, lineageBranches)

			sc.Save()

			if err != nil {
				if result != nil && result.Conflicts {
					return mcp.NewToolResultError(fmt.Sprintf("conflict at '%s' — resolve and run st_continue", result.ConflictsAt)), nil
				}
				return mcp.NewToolResultError(fmt.Sprintf("restack failed: %v", err)), nil
			}

			if result != nil && len(result.Backups) > 0 {
				backupMgr.CleanupStackBackups(lineageBranches)
			}

			completed := 0
			if result != nil {
				completed = len(result.Completed)
			}
			return mcp.NewToolResultText(fmt.Sprintf("Restacked %d branches", completed)), nil
		},
	)

	// st_continue
	s.AddTool(
		mcp.NewTool("st_continue",
			mcp.WithDescription("Continue a restack after conflict resolution."),
			mcp.WithToolAnnotation(mutating()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			inProgress, err := sc.Git.IsRebaseInProgress()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to check rebase status: %v", err)), nil
			}
			if !inProgress {
				return mcp.NewToolResultError("no rebase in progress — nothing to continue"), nil
			}

			backupMgr := backup.NewManager(sc.Git, sc.RepoPath)
			engine := restack.NewEngine(sc.Git, backupMgr)

			// Load restack state to get lineage info
			var lineage []string
			state, stateErr := restack.LoadRestackState(sc.RepoPath)
			if stateErr == nil && state != nil {
				lineage = state.Lineage
			}

			result, err := engine.Continue(sc.Graph, lineage)
			restack.ClearRestackState(sc.RepoPath)
			sc.Save()

			if err != nil {
				if result != nil && result.Conflicts {
					return mcp.NewToolResultError(fmt.Sprintf("still have conflicts at '%s'", result.ConflictsAt)), nil
				}
				return mcp.NewToolResultError(err.Error()), nil
			}

			completed := 0
			if result != nil {
				completed = len(result.Completed)
			}
			return mcp.NewToolResultText(fmt.Sprintf("Continue complete, restacked %d branches", completed)), nil
		},
	)
}
