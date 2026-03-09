package mcp

import (
	"context"
	_ "embed"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

//go:embed prompts/split-monolithic-pr.md
var splitPromptTemplate string

//go:embed prompts/learn-staccato.md
var learnPromptContent string

func registerPrompts(s *server.MCPServer) {
	// Register as a prompt (with template arguments)
	s.AddPrompt(
		mcp.NewPrompt("split-monolithic-pr",
			mcp.WithPromptDescription("Guide for splitting a large PR into focused, stacked commits/branches."),
			mcp.WithArgument("base_branch",
				mcp.ArgumentDescription("The base/trunk branch (default: main)"),
			),
			mcp.WithArgument("source_branch",
				mcp.ArgumentDescription("The source branch with changes (default: current branch)"),
			),
		),
		func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			baseBranch := "main"
			if v, ok := req.Params.Arguments["base_branch"]; ok && v != "" {
				baseBranch = v
			}
			sourceBranch := "current branch"
			if v, ok := req.Params.Arguments["source_branch"]; ok && v != "" {
				sourceBranch = v
			}

			rendered := strings.ReplaceAll(splitPromptTemplate, "{{base_branch}}", baseBranch)
			rendered = strings.ReplaceAll(rendered, "{{source_branch}}", sourceBranch)

			return &mcp.GetPromptResult{
				Description: "Guide for splitting a large PR into focused, stacked commits/branches.",
				Messages: []mcp.PromptMessage{
					mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(rendered)),
				},
			}, nil
		},
	)

	// Register learn-staccato prompt
	s.AddPrompt(
		mcp.NewPrompt("learn-staccato",
			mcp.WithPromptDescription("Comprehensive guide to using Staccato for Git stack management. Learn commands, workflows, and best practices."),
		),
		func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{
				Description: "Comprehensive guide to using Staccato for Git stack management.",
				Messages: []mcp.PromptMessage{
					mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(learnPromptContent)),
				},
			}, nil
		},
	)

	// Register learn-staccato as a resource
	s.AddResource(
		mcp.NewResource(
			"staccato://prompts/learn-staccato",
			"learn-staccato",
			mcp.WithResourceDescription("Comprehensive guide to using Staccato for Git stack management."),
			mcp.WithMIMEType("text/markdown"),
		),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      "staccato://prompts/learn-staccato",
					MIMEType: "text/markdown",
					Text:     learnPromptContent,
				},
			}, nil
		},
	)

	// Also register as a resource so it's discoverable via resources/list
	s.AddResource(
		mcp.NewResource(
			"staccato://prompts/split-monolithic-pr",
			"split-monolithic-pr",
			mcp.WithResourceDescription("Guide for splitting a large PR into focused, stacked commits/branches. Contains template placeholders {{base_branch}} and {{source_branch}}."),
			mcp.WithMIMEType("text/markdown"),
		),
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      "staccato://prompts/split-monolithic-pr",
					MIMEType: "text/markdown",
					Text:     splitPromptTemplate,
				},
			}, nil
		},
	)
}
