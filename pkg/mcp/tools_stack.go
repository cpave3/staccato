package mcp

import (
	"context"
	"encoding/json"

	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/cpave3/staccato/pkg/forge"
	"github.com/cpave3/staccato/pkg/graph"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// stackNode represents a branch in the stack tree for JSON serialization.
type stackNode struct {
	Branch   string      `json:"branch"`
	Current  bool        `json:"current,omitempty"`
	Children []stackNode `json:"children,omitempty"`
}

// statusNode extends stackNode with PR status info.
type statusNode struct {
	Branch       string       `json:"branch"`
	Current      bool         `json:"current,omitempty"`
	PRNumber     int          `json:"pr_number,omitempty"`
	PRState      string       `json:"pr_state,omitempty"`
	PRDraft      bool         `json:"pr_draft,omitempty"`
	ReviewStatus string       `json:"review_status,omitempty"`
	CheckStatus  string       `json:"check_status,omitempty"`
	PRURL        string       `json:"pr_url,omitempty"`
	Children     []statusNode `json:"children,omitempty"`
}

func registerStackTools(s *server.MCPServer, sc *stcontext.StaccatoContext) {
	// st_log
	s.AddTool(
		mcp.NewTool("st_log",
			mcp.WithDescription("Show the stack tree structure as JSON."),
			mcp.WithToolAnnotation(readOnly()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			currentBranch, _ := sc.Git.GetCurrentBranch()
			tree := buildStackTree(sc.Graph, sc.Graph.Root, currentBranch)
			data, err := json.MarshalIndent(tree, "", "  ")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// st_status
	s.AddTool(
		mcp.NewTool("st_status",
			mcp.WithDescription("Show stack tree with PR status annotations."),
			mcp.WithToolAnnotation(readOnly()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			currentBranch, _ := sc.Git.GetCurrentBranch()

			var prStatus map[string]*forge.PRStatusInfo
			f, err := forge.Detect(sc.Git)
			if err == nil {
				var branches []string
				for name := range sc.Graph.Branches {
					branches = append(branches, name)
				}
				prStatus, _ = f.StackStatus(branches)
			}

			tree := buildStatusTree(sc.Graph, sc.Graph.Root, currentBranch, prStatus)
			data, err := json.MarshalIndent(tree, "", "  ")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	// st_current
	s.AddTool(
		mcp.NewTool("st_current",
			mcp.WithDescription("Return the current branch and its position in the stack."),
			mcp.WithToolAnnotation(readOnly()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			currentBranch, err := sc.Git.GetCurrentBranch()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			info := map[string]any{
				"branch":   currentBranch,
				"root":     sc.Graph.Root,
				"in_stack": false,
			}

			if b, exists := sc.Graph.GetBranch(currentBranch); exists {
				info["parent"] = b.Parent
				info["in_stack"] = true
			} else if currentBranch == sc.Graph.Root {
				info["in_stack"] = true
			}

			data, _ := json.MarshalIndent(info, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)
}

func buildStackTree(g *graph.Graph, root, currentBranch string) stackNode {
	node := stackNode{
		Branch:  root,
		Current: root == currentBranch,
	}
	children := g.GetChildren(root)
	for _, child := range children {
		node.Children = append(node.Children, buildStackTree(g, child.Name, currentBranch))
	}
	return node
}

func buildStatusTree(g *graph.Graph, root, currentBranch string, prStatus map[string]*forge.PRStatusInfo) statusNode {
	node := statusNode{
		Branch:  root,
		Current: root == currentBranch,
	}

	if prStatus != nil {
		if info, ok := prStatus[root]; ok && info.HasPR {
			node.PRNumber = info.Number
			node.PRState = info.State
			node.PRDraft = info.IsDraft
			node.ReviewStatus = info.ReviewStatus
			node.CheckStatus = info.CheckStatus
			node.PRURL = info.URL
		}
	}

	children := g.GetChildren(root)
	for _, child := range children {
		node.Children = append(node.Children, buildStatusTree(g, child.Name, currentBranch, prStatus))
	}
	return node
}
