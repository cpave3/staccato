package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	stcontext "github.com/cpave3/staccato/pkg/context"
	stync "github.com/cpave3/staccato/pkg/sync"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerSyncTools(s *server.MCPServer, sc *stcontext.StaccatoContext) {
	// st_sync
	s.AddTool(
		mcp.NewTool("st_sync",
			mcp.WithDescription("Fetch, detect merged branches, restack, and push."),
			mcp.WithToolAnnotation(mutating()),
			mcp.WithBoolean("dry_run", mcp.Description("Show what would happen without making changes")),
			mcp.WithBoolean("down_only", mcp.Description("Only pull changes, skip pushing")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			dryRun := req.GetBool("dry_run", false)
			downOnly := req.GetBool("down_only", false)

			result, err := stync.Run(sc, stync.Options{
				DryRun:   dryRun,
				DownOnly: downOnly,
			})

			if err != nil && result == nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			resp := map[string]any{
				"fetched":         result.Fetched,
				"trunk_updated":   result.TrunkUpdated,
				"merged_branches": result.MergedBranches,
				"pushed_branches": result.PushedBranches,
				"restacked_count": result.RestackedCount,
				"dry_run":         dryRun,
			}
			if result.Conflicts {
				resp["conflicts"] = true
				resp["conflicts_at"] = result.ConflictsAt
			}

			data, _ := json.MarshalIndent(resp, "", "  ")
			if err != nil {
				// Return the structured result along with the error message
				return mcp.NewToolResultError(fmt.Sprintf("%s\n\n%s", err.Error(), string(data))), nil
			}
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// st_pr
	s.AddTool(
		mcp.NewTool("st_pr",
			mcp.WithDescription("Push the current branch and return info for PR creation. With stack=true, pushes and returns info for all branches in the lineage."),
			mcp.WithToolAnnotation(mutating()),
			mcp.WithBoolean("stack", mcp.Description("Push and return PR info for all branches in the current lineage")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			stack := req.GetBool("stack", false)

			currentBranch, err := sc.Git.GetCurrentBranch()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			if _, exists := sc.Graph.GetBranch(currentBranch); !exists {
				return mcp.NewToolResultError(fmt.Sprintf("branch '%s' is not in the stack", currentBranch)), nil
			}

			remoteURL, _ := sc.Git.GetRemoteURL("origin")

			if stack {
				// Get ancestors from root to current branch
				var lineage []string
				at := currentBranch
				for at != "" && at != sc.Graph.Root {
					lineage = append([]string{at}, lineage...)
					if b, exists := sc.Graph.GetBranch(at); exists {
						at = b.Parent
					} else {
						break
					}
				}

				var results []map[string]any
				for _, branch := range lineage {
					branchInfo, _ := sc.Graph.GetBranch(branch)
					pushed := false
					if !sc.Git.RemoteBranchExists(branch) {
						if err := sc.Git.Push(branch, false); err != nil {
							return mcp.NewToolResultError(fmt.Sprintf("failed to push '%s': %v", branch, err)), nil
						}
						pushed = true
					}
					results = append(results, map[string]any{
						"head":       branch,
						"base":       branchInfo.Parent,
						"remote_url": remoteURL,
						"pushed":     pushed,
					})
				}
				data, _ := json.MarshalIndent(results, "", "  ")
				return mcp.NewToolResultText(string(data)), nil
			}

			// Single branch mode
			branchInfo, _ := sc.Graph.GetBranch(currentBranch)
			pushed := false
			if !sc.Git.RemoteBranchExists(currentBranch) {
				if err := sc.Git.Push(currentBranch, false); err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("failed to push: %v", err)), nil
				}
				pushed = true
			}

			resp := map[string]any{
				"head":       currentBranch,
				"base":       branchInfo.Parent,
				"remote_url": remoteURL,
				"pushed":     pushed,
			}
			data, _ := json.MarshalIndent(resp, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)
}
