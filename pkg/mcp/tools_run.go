package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerRunTool(s *server.MCPServer, sc *stcontext.StaccatoContext) {
	s.AddTool(
		mcp.NewTool("st_run",
			mcp.WithDescription(
				"Run any st (staccato) CLI command. This is a universal wrapper "+
					"that can execute any subcommand. For structured JSON output, prefer "+
					"st_log, st_status, or st_current. Examples: \"up\", \"down\", \"restack --to-current\", "+
					"\"delete feature-x\", \"modify --all\". "+
					"Note: 'st mcp' cannot be run recursively.",
			),
			mcp.WithToolAnnotation(mutating()),
			mcp.WithString("command", mcp.Required(), mcp.Description("The st subcommand and arguments (e.g. \"up\", \"restack --to-current\")")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			command, err := req.RequireString("command")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			command = strings.TrimSpace(command)
			if command == "" {
				return mcp.NewToolResultError("command is required"), nil
			}

			// Block recursive MCP invocation
			parts := strings.Fields(command)
			if len(parts) > 0 && parts[0] == "mcp" {
				return mcp.NewToolResultError("cannot run 'st mcp' recursively"), nil
			}

			// Resolve binary path
			binary, err := os.Executable()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to resolve binary: %v", err)), nil
			}

			cmd := exec.CommandContext(ctx, binary, parts...)
			cmd.Dir = sc.RepoPath
			output, runErr := cmd.CombinedOutput()

			if runErr != nil {
				return mcp.NewToolResultError(string(output)), nil
			}

			result := strings.TrimSpace(string(output))
			if result == "" {
				result = "Command completed successfully"
			}
			return mcp.NewToolResultText(result), nil
		},
	)
}
