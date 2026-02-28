package mcp

import (
	"context"

	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerGitTools(s *server.MCPServer, sc *stcontext.StaccatoContext) {
	// st_git_log
	s.AddTool(
		mcp.NewTool("st_git_log",
			mcp.WithDescription("Show git log (oneline). Optional range, limit, and stat."),
			mcp.WithToolAnnotation(readOnly()),
			mcp.WithString("range", mcp.Description("Git range spec, e.g. 'main..HEAD'")),
			mcp.WithNumber("limit", mcp.Description("Max number of commits (default 20)")),
			mcp.WithBoolean("stat", mcp.Description("Include --stat output")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			rangeSpec := req.GetString("range", "")
			limit := req.GetInt("limit", 20)
			stat := req.GetBool("stat", false)
			out, err := sc.Git.Log(rangeSpec, limit, stat)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(out), nil
		},
	)

	// st_git_diff_stat
	s.AddTool(
		mcp.NewTool("st_git_diff_stat",
			mcp.WithDescription("Show diff --stat against a ref."),
			mcp.WithToolAnnotation(readOnly()),
			mcp.WithString("ref", mcp.Required(), mcp.Description("Reference to diff against")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			ref, err := req.RequireString("ref")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			out, err := sc.Git.DiffStat(ref)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(out), nil
		},
	)

	// st_git_cherry_pick
	s.AddTool(
		mcp.NewTool("st_git_cherry_pick",
			mcp.WithDescription("Cherry-pick one or more commits onto the current branch."),
			mcp.WithToolAnnotation(mutating()),
			mcp.WithArray("commits", mcp.Required(), mcp.Description("Commit SHAs to cherry-pick"), mcp.WithStringItems()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			commits := req.GetStringSlice("commits", nil)
			if len(commits) == 0 {
				return mcp.NewToolResultError("commits is required and must be non-empty"), nil
			}
			out, err := sc.Git.CherryPick(commits...)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(out), nil
		},
	)

	// st_git_checkout
	s.AddTool(
		mcp.NewTool("st_git_checkout",
			mcp.WithDescription("Checkout an existing branch."),
			mcp.WithToolAnnotation(mutating()),
			mcp.WithString("branch", mcp.Required(), mcp.Description("Branch to checkout")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			branch, err := req.RequireString("branch")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if err := sc.Git.CheckoutBranch(branch); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText("Checked out " + branch), nil
		},
	)

	// st_git_reset
	s.AddTool(
		mcp.NewTool("st_git_reset",
			mcp.WithDescription("Reset HEAD. Mode must be soft, mixed, or hard."),
			mcp.WithToolAnnotation(destructive()),
			mcp.WithString("ref", mcp.Description("Ref to reset to (omit for current HEAD)")),
			mcp.WithString("mode", mcp.Required(), mcp.Description("Reset mode: soft, mixed, or hard"), mcp.Enum("soft", "mixed", "hard")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			mode, err := req.RequireString("mode")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			ref := req.GetString("ref", "")
			out, err := sc.Git.Reset(ref, mode)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if out == "" {
				out = "Reset complete"
			}
			return mcp.NewToolResultText(out), nil
		},
	)

	// st_git_add
	s.AddTool(
		mcp.NewTool("st_git_add",
			mcp.WithDescription("Stage files."),
			mcp.WithToolAnnotation(mutating()),
			mcp.WithArray("paths", mcp.Required(), mcp.Description("File paths to stage"), mcp.WithStringItems()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			paths := req.GetStringSlice("paths", nil)
			if len(paths) == 0 {
				return mcp.NewToolResultError("paths is required and must be non-empty"), nil
			}
			out, err := sc.Git.Add(paths)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if out == "" {
				out = "Files staged"
			}
			return mcp.NewToolResultText(out), nil
		},
	)

	// st_git_commit
	s.AddTool(
		mcp.NewTool("st_git_commit",
			mcp.WithDescription("Create a commit with the given message."),
			mcp.WithToolAnnotation(mutating()),
			mcp.WithString("message", mcp.Required(), mcp.Description("Commit message")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			msg, err := req.RequireString("message")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			out, err := sc.Git.Commit(msg)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(out), nil
		},
	)

	// st_git_status
	s.AddTool(
		mcp.NewTool("st_git_status",
			mcp.WithDescription("Show working tree status (porcelain format)."),
			mcp.WithToolAnnotation(readOnly()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			out, err := sc.Git.Status()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if out == "" {
				out = "Working tree clean"
			}
			return mcp.NewToolResultText(out), nil
		},
	)

	// st_git_diff
	s.AddTool(
		mcp.NewTool("st_git_diff",
			mcp.WithDescription("Show diff output. Optionally staged only, optionally filtered to paths."),
			mcp.WithToolAnnotation(readOnly()),
			mcp.WithBoolean("staged", mcp.Description("Show only staged changes")),
			mcp.WithArray("paths", mcp.Description("Paths to filter diff to"), mcp.WithStringItems()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			staged := req.GetBool("staged", false)
			paths := req.GetStringSlice("paths", nil)
			out, err := sc.Git.Diff(staged, paths)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if out == "" {
				out = "No differences"
			}
			return mcp.NewToolResultText(out), nil
		},
	)
}
