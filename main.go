package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	miroclient "github.com/your-org/miro-mcp/internal/miro"
	"github.com/your-org/miro-mcp/internal/prompts"
	"github.com/your-org/miro-mcp/internal/tools"
)

const (
	serverName    = "miro-mcp"
	serverVersion = "1.0.0"
)

func main() {
	// Initialise the Miro REST client — fails fast if token is missing.
	miroCli, err := miroclient.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}

	// Create the MCP server.
	s := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(false),
		server.WithPromptCapabilities(false),
		server.WithRecovery(),
	)

	// Register all tools.
	tools.RegisterBoardTools(s, miroCli)
	tools.RegisterContextTools(s, miroCli)
	tools.RegisterDiagramTools(s, miroCli)
	tools.RegisterDocTools(s, miroCli)
	tools.RegisterImageTools(s, miroCli)
	tools.RegisterTableTools(s, miroCli)

	// Register built-in prompts.
	prompts.RegisterPrompts(s)

	// Start the stdio transport — VS Code spawns this process and communicates
	// over stdin/stdout. No port binding required.
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
