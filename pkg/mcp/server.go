package mcp

import (
	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewServer creates an MCP server with all staccato tools and prompts registered.
// The StaccatoContext is captured by handler closures — mutations apply in-place,
// and sc.Save() is called after write operations to persist state.
func NewServer(sc *stcontext.StaccatoContext) *server.MCPServer {
	s := server.NewMCPServer(
		"staccato",
		"0.1.0",
		server.WithToolCapabilities(false),
		server.WithPromptCapabilities(false),
		server.WithResourceCapabilities(false, false),
	)

	registerGitTools(s, sc)
	registerStackTools(s, sc)
	registerBranchTools(s, sc)
	registerManagementTools(s, sc)
	registerSyncTools(s, sc)
	registerRunTool(s, sc)
	registerReviewTools(s, sc)
	registerPrompts(s)

	return s
}

// readOnly is a helper for read-only tool annotations.
func readOnly() mcp.ToolAnnotation {
	return mcp.ToolAnnotation{
		ReadOnlyHint:    mcp.ToBoolPtr(true),
		DestructiveHint: mcp.ToBoolPtr(false),
		IdempotentHint:  mcp.ToBoolPtr(true),
		OpenWorldHint:   mcp.ToBoolPtr(false),
	}
}

// destructive is a helper for destructive tool annotations.
func destructive() mcp.ToolAnnotation {
	return mcp.ToolAnnotation{
		ReadOnlyHint:    mcp.ToBoolPtr(false),
		DestructiveHint: mcp.ToBoolPtr(true),
		IdempotentHint:  mcp.ToBoolPtr(false),
		OpenWorldHint:   mcp.ToBoolPtr(false),
	}
}

// mutating is a helper for mutating (but not destructive) tool annotations.
func mutating() mcp.ToolAnnotation {
	return mcp.ToolAnnotation{
		ReadOnlyHint:    mcp.ToBoolPtr(false),
		DestructiveHint: mcp.ToBoolPtr(false),
		IdempotentHint:  mcp.ToBoolPtr(false),
		OpenWorldHint:   mcp.ToBoolPtr(false),
	}
}
