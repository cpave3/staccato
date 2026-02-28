package mcp

import (
	"context"
	"fmt"

	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/cpave3/staccato/pkg/backup"
	"github.com/cpave3/staccato/pkg/restack"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerBranchTools(s *server.MCPServer, sc *stcontext.StaccatoContext) {
	// st_new
	s.AddTool(
		mcp.NewTool("st_new",
			mcp.WithDescription("Create a new branch from the trunk/root."),
			mcp.WithToolAnnotation(mutating()),
			mcp.WithString("branch_name", mcp.Required(), mcp.Description("Name for the new branch")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := req.RequireString("branch_name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			if err := sc.Git.CreateAndCheckoutBranch(name); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to create branch: %v", err)), nil
			}

			baseSHA, _ := sc.Git.GetCommitSHA(sc.Graph.Root)
			headSHA, _ := sc.Git.GetCommitSHA(name)
			sc.Graph.AddBranch(name, sc.Graph.Root, baseSHA, headSHA)

			if err := sc.Save(); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to save graph: %v", err)), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf("Created branch '%s' from '%s'", name, sc.Graph.Root)), nil
		},
	)

	// st_append
	s.AddTool(
		mcp.NewTool("st_append",
			mcp.WithDescription("Create a child branch from the current branch."),
			mcp.WithToolAnnotation(mutating()),
			mcp.WithString("branch_name", mcp.Required(), mcp.Description("Name for the new branch")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := req.RequireString("branch_name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			parentBranch, _ := sc.Git.GetCurrentBranch()
			if parentBranch != sc.Graph.Root {
				if _, exists := sc.Graph.GetBranch(parentBranch); !exists {
					return mcp.NewToolResultError(fmt.Sprintf("current branch '%s' is not in the stack", parentBranch)), nil
				}
			}

			if err := sc.Git.CreateAndCheckoutBranch(name); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to create branch: %v", err)), nil
			}

			baseSHA, _ := sc.Git.GetCommitSHA(parentBranch)
			headSHA, _ := sc.Git.GetCommitSHA(name)
			sc.Graph.AddBranch(name, parentBranch, baseSHA, headSHA)

			if err := sc.Save(); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to save graph: %v", err)), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf("Created branch '%s' as child of '%s'", name, parentBranch)), nil
		},
	)

	// st_insert
	s.AddTool(
		mcp.NewTool("st_insert",
			mcp.WithDescription("Insert a new branch before the current branch, reparenting downstream."),
			mcp.WithToolAnnotation(mutating()),
			mcp.WithString("branch_name", mcp.Required(), mcp.Description("Name for the new branch")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := req.RequireString("branch_name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			currentBranch, _ := sc.Git.GetCurrentBranch()
			currentInfo, exists := sc.Graph.GetBranch(currentBranch)
			if !exists {
				return mcp.NewToolResultError(fmt.Sprintf("current branch '%s' is not in the stack", currentBranch)), nil
			}

			oldParent := currentInfo.Parent

			backupMgr := backup.NewManager(sc.Git, sc.RepoPath)
			downstreamBranches := restack.GetDownstreamBranches(sc.Graph, currentBranch)
			affectedBranches := append([]string{currentBranch}, downstreamBranches...)

			backups, err := backupMgr.CreateBackupsForStack(affectedBranches)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to create backups: %v", err)), nil
			}

			if err := sc.Git.CheckoutBranch(oldParent); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to checkout parent: %v", err)), nil
			}

			if err := sc.Git.CreateAndCheckoutBranch(name); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to create branch: %v", err)), nil
			}

			baseSHA, _ := sc.Git.GetCommitSHA(oldParent)
			headSHA, _ := sc.Git.GetCommitSHA(name)
			sc.Graph.AddBranch(name, oldParent, baseSHA, headSHA)
			sc.Graph.Branches[currentBranch].Parent = name

			if err := sc.Save(); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to save graph: %v", err)), nil
			}

			engine := restack.NewEngine(sc.Git, backupMgr)
			result, err := engine.Restack(sc.Graph, name)
			if err != nil {
				if result != nil && result.Conflicts {
					sc.Save()
					return mcp.NewToolResultError(fmt.Sprintf("conflict during restack at '%s' — resolve and run st_continue", result.ConflictsAt)), nil
				}
				backupMgr.RestoreStack(backups)
				return mcp.NewToolResultError(fmt.Sprintf("restack failed: %v", err)), nil
			}

			backupMgr.CleanupStackBackups(affectedBranches)
			sc.Git.CheckoutBranch(name)

			return mcp.NewToolResultText(fmt.Sprintf("Inserted '%s' before '%s', restacked %d branches", name, currentBranch, len(result.Completed))), nil
		},
	)
}
