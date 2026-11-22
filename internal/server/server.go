package server

import (
	mcp_server "github.com/mark3labs/mcp-go/server"
)

// Name is the MCP server name advertised to clients.
const Name = "argocd-mcp-server"

// New creates and returns a new MCP server instance advertising the given version.
func New(version string) *mcp_server.MCPServer {
	s := mcp_server.NewMCPServer(
		Name,
		version,
		// Add recovery middleware to protect server from panics in handlers
		mcp_server.WithRecovery(),
	)
	return s
}
