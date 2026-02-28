package main

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	stcontext "github.com/cpave3/staccato/pkg/context"
	stmcp "github.com/cpave3/staccato/pkg/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start the MCP server (stdio transport)",
		Long:  "Runs the Staccato MCP server over stdin/stdout for use by LLM clients.",
		RunE: func(cmd *cobra.Command, args []string) error {
			sc, err := stcontext.Load("")
			if err != nil {
				return err
			}

			srv := stmcp.NewServer(sc)
			stdio := server.NewStdioServer(srv)
			return stdio.Listen(context.Background(), os.Stdin, os.Stdout)
		},
	}
}
