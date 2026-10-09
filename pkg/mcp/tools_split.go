package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/cpave3/staccato/pkg/split"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const splitDescribeDescription = `Describe a branch so you can plan how to split it into smaller stacked branches. Read-only.

Returns JSON: the branch's parent and head, every commit (abbreviated sha, subject, files, insertions/deletions), every changed file (status, insertions/deletions, the commits that touch it, renamed_from), totals, and churn.

Reading it:
- Cut at "commits" only when each commit belongs wholly to one slice. Split by "paths" when a commit touches files that belong in different slices.
- "totals" is the net change; "churn" adds up every commit. When churn is much larger than totals the commits rework each other, so split by "paths" rather than by "commits".
- A file touched by several commits, or needed by more than one slice, is where a split gets hard: give it to one slice, or use an override in st_split_apply.
- "needs_restack": true means the branch must be restacked before it can be split.`

const splitApplyDescription = `Split one branch of the stack into several stacked branches, following a plan. Use st_split_describe first.

Every slice is built before any branch moves, and the top slice must end with exactly the content the branch had, so a plan that cannot be carried out changes nothing. Descendant branches are moved onto the top slice automatically.

The plan lists slices from the bottom of the stack (nearest the parent) to the top:

{
  "branch": "feature",
  "head": "<head from st_split_describe>",
  "slices": [
    {"name": "feature-core", "message": "core: add types", "paths": ["pkg/core/"]},
    {"name": "feature-api", "commits": ["1a2b3c4d5e6f"]},
    {"name": "feature", "message": "ui: add page", "rest": true}
  ]
}

Each slice takes its content from exactly one source:
- "paths": git pathspecs relative to the repository root (a file, a directory, a glob, or magic such as ":(exclude)docs/"). Every changed file they match is brought to its FINAL state, deletions included, in one new commit. A path takes only files that no earlier slice has taken; taking none is an error. Needs "message".
- "commits": existing commits of the branch, kept with their message and author, applied in the order given. No "message".
- "rest": true — everything the earlier slices left out. Last slice only. Needs "message".

A "paths" slice may add "overrides": {"<repo path>": "<file holding the content>"} to give a file content that is only partway to final, when its changes belong to two slices. A later slice must list that path to finish it. Content files are absolute paths, or relative to the repository root.

Name the top slice after the original branch so its pull request and children stay attached; to delete the original name instead, set "drop_original": true.

The working tree is not touched, so uncommitted changes are fine. When a split rewrites commits, the originals are kept on the branch named in "backup"; there is no undo command.

Set dry_run to build the slices and get their sizes and commit ids without moving anything (the ids are previews: applying builds the commits again). Each slice must build on its own, so put a change with everything that needs it: check with dry_run, then build each reported sha, or run 'st split apply --exec "<build command>"' from a shell, which applies only if the command passes in every slice.`

func registerSplitTools(s *server.MCPServer, sc *stcontext.StaccatoContext) {
	s.AddTool(
		mcp.NewTool("st_split_describe",
			mcp.WithDescription(splitDescribeDescription),
			mcp.WithToolAnnotation(readOnly()),
			mcp.WithString("branch", mcp.Description("Branch to describe (default: the branch checked out where the server runs)")),
		),
		splitDescribeHandler(sc),
	)

	stringList := map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	s.AddTool(
		mcp.NewTool("st_split_apply",
			mcp.WithDescription(splitApplyDescription),
			mcp.WithToolAnnotation(destructive()),
			mcp.WithObject("plan", mcp.Required(),
				mcp.Description("The split plan"),
				mcp.Properties(map[string]any{
					"branch":        map[string]any{"type": "string", "description": "Branch to split"},
					"head":          map[string]any{"type": "string", "description": "Expected tip of the branch, from st_split_describe; the split is refused if the branch has moved"},
					"drop_original": map[string]any{"type": "boolean", "description": "Delete the original branch name; required when no slice keeps it"},
					"slices": map[string]any{
						"type":        "array",
						"minItems":    2,
						"description": "Slices from the bottom of the stack to the top",
						"items": map[string]any{
							"type":     "object",
							"required": []string{"name"},
							"properties": map[string]any{
								"name":      map[string]any{"type": "string", "description": "Branch name for this slice"},
								"message":   map[string]any{"type": "string", "description": "Commit message, for paths and rest slices"},
								"paths":     stringList,
								"commits":   stringList,
								"rest":      map[string]any{"type": "boolean"},
								"overrides": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
							},
						},
					},
				}),
			),
			mcp.WithBoolean("dry_run", mcp.Description("Build and report the slices without moving any branch")),
		),
		splitApplyHandler(sc),
	)
}

func splitDescribeHandler(sc *stcontext.StaccatoContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// The server outlives any one command; the CLI may have changed the
		// graph since it was loaded.
		if err := sc.Reload(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		description, err := split.Describe(sc, req.GetString("branch", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		data, _ := json.MarshalIndent(description, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

func splitApplyHandler(sc *stcontext.StaccatoContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Some clients send a nested object as a JSON string.
		var raw []byte
		switch plan := req.GetArguments()["plan"].(type) {
		case nil:
			return mcp.NewToolResultError(`"plan" is required`), nil
		case string:
			raw = []byte(plan)
		default:
			var err error
			if raw, err = json.Marshal(plan); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid plan: %v", err)), nil
			}
		}
		plan, err := split.ParsePlan(raw)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if err := sc.Reload(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		// The server may run in a different worktree from the agent, so "the
		// current branch" is not safe to assume for something that rewrites it.
		if plan.Branch == "" {
			return mcp.NewToolResultError(fmt.Sprintf(`the plan needs a "branch": this server runs in %s, which may not be the checkout you are working in`, sc.RepoPath)), nil
		}

		result, err := split.Apply(sc, plan, split.Options{
			DryRun:  req.GetBool("dry_run", false),
			BaseDir: sc.RepoPath,
		})
		if err != nil {
			// A failed apply may have left the graph half-edited in memory.
			sc.Reload()
			return mcp.NewToolResultError(err.Error()), nil
		}
		split.FireHooks(sc.RepoPath, result)

		data, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}
