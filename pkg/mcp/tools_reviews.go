package mcp

import (
	"context"
	"fmt"
	"os"

	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/cpave3/staccato/pkg/forge"
	"github.com/cpave3/staccato/pkg/reviews"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerReviewTools(s *server.MCPServer, sc *stcontext.StaccatoContext) {
	s.AddTool(
		mcp.NewTool("st_reviews",
			mcp.WithDescription("Collect PR review feedback for stack branches. Returns a unified markdown document with all inline comments, review submissions, and general comments from GitHub PRs."),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				ReadOnlyHint:    mcp.ToBoolPtr(true),
				DestructiveHint: mcp.ToBoolPtr(false),
				IdempotentHint:  mcp.ToBoolPtr(true),
				OpenWorldHint:   mcp.ToBoolPtr(true),
			}),
			mcp.WithString("scope", mcp.Description("Scope of branches to collect reviews from: 'all' (default), 'current', or 'to-current'")),
			mcp.WithString("out", mcp.Description("Optional file path to write output to instead of returning it")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			scopeStr := req.GetString("scope", "all")
			outPath := req.GetString("out", "")

			var scope reviews.Scope
			switch scopeStr {
			case "current":
				scope = reviews.ScopeCurrent
			case "to-current":
				scope = reviews.ScopeToCurrent
			default:
				scope = reviews.ScopeAll
			}

			currentBranch, err := sc.Git.GetCurrentBranch()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get current branch: %s", err)), nil
			}

			branches := reviews.ResolveBranches(sc.Graph, currentBranch, scope)
			if len(branches) == 0 {
				return mcp.NewToolResultText("No branches in scope."), nil
			}

			f, err := forge.Detect(sc.Git)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("forge detection failed: %s", err)), nil
			}

			prStatus, err := f.StackStatus(branches)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get PR status: %s", err)), nil
			}

			prs := make(map[string]int)
			for _, branch := range branches {
				info, ok := prStatus[branch]
				if ok && info.HasPR && info.Number > 0 {
					prs[branch] = info.Number
				}
			}

			if len(prs) == 0 {
				return mcp.NewToolResultText("No PRs found for branches in scope."), nil
			}

			remoteURL, err := sc.Git.GetRemoteURL("origin")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get remote URL: %s", err)), nil
			}
			owner, repo, err := reviews.ParseRemoteURL(remoteURL)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			items, err := reviews.FetchAll(owner, repo, prs, 5)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to fetch reviews: %s", err)), nil
			}

			items = reviews.ThreadReplies(items)

			result := reviews.ReviewResult{
				Items:     items,
				Scope:     scope,
				RepoOwner: owner,
				RepoName:  repo,
			}
			md := reviews.FormatMarkdown(result)

			if outPath != "" {
				if err := os.WriteFile(outPath, []byte(md), 0644); err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("failed to write output: %s", err)), nil
				}
				return mcp.NewToolResultText(fmt.Sprintf("Wrote feedback to %s", outPath)), nil
			}

			return mcp.NewToolResultText(md), nil
		},
	)
}
