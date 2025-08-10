package main

import (
	"os"

	mcp_server "github.com/mark3labs/mcp-go/server"
	"github.com/sirupsen/logrus"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/logging"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/tools"
)

func main() {
	log := logging.GetLogger()

	// Log startup
	log.WithFields(logrus.Fields{
		"version": "1.0.0",
		"pid":     os.Getpid(),
	}).Info("Starting ArgoCD MCP Server")

	// Check environment variables
	if os.Getenv("ARGOCD_AUTH_TOKEN") == "" {
		log.Fatal("ARGOCD_AUTH_TOKEN environment variable is not set. Please set it using: export ARGOCD_AUTH_TOKEN=$(argocd account generate-token)")
	}
	if os.Getenv("ARGOCD_SERVER") == "" {
		log.Fatal("ARGOCD_SERVER environment variable is not set. Please set it using: export ARGOCD_SERVER=your-argocd-server.com")
	}

	log.WithField("server", os.Getenv("ARGOCD_SERVER")).Debug("ArgoCD server configured")

	// 1. Create server instance
	log.Debug("Creating MCP server instance")
	s := server.New()

	// 2. Register all tools with the server
	log.Debug("Registering tools")
	tools.RegisterAll(s)
	log.Info("All tools registered successfully")

	// 3. Start server via stdio
	log.Info("ArgoCD MCP Server started. Waiting for requests on stdin...")
	if err := mcp_server.ServeStdio(s); err != nil {
		log.WithError(err).Fatal("Server error")
	}
}
