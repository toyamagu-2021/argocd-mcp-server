package server

import (
	mcp_server "github.com/mark3labs/mcp-go/server"
)

// New creates and returns a new MCP server instance
func New() *mcp_server.MCPServer {
	s := mcp_server.NewMCPServer(
		"argocd-mcp-server",
		"1.0.0",
		// Add recovery middleware to protect server from panics in handlers
		mcp_server.WithRecovery(),
	)
	return s
}
