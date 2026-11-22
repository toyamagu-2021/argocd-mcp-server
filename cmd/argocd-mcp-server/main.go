package main

import (
	"flag"
	"fmt"
	"os"

	mcp_server "github.com/mark3labs/mcp-go/server"
	"github.com/sirupsen/logrus"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/config"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/logging"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/tools"
)

var (
	version = "1.0.0"
	commit  = "none"
	date    = "unknown"
)

func main() {
	versionFlag := flag.Bool("version", false, "Print version information")
	flag.Parse()

	// Check for version flag
	if *versionFlag {
		fmt.Printf("argocd-mcp-server version %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	// Check for version subcommand
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("argocd-mcp-server version %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	log := logging.GetLogger()

	// Log startup
	log.WithFields(logrus.Fields{
		"version": version,
		"commit":  commit,
		"date":    date,
		"pid":     os.Getpid(),
	}).Info("Starting ArgoCD MCP Server")

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	log.WithField("server", cfg.Server).Debug("ArgoCD server configured")

	// One Argo CD client for the whole process. With gRPC-Web enabled this
	// also owns the single local proxy.
	argo, err := client.New(cfg.ToClientConfig())
	if err != nil {
		log.WithError(err).Fatal("Failed to create Argo CD client")
	}
	defer func() { _ = argo.Close() }()

	s := server.New(version)
	tools.Register(s, tools.Deps{Client: argo})
	log.Info("All tools registered successfully")

	// Start server via stdio
	log.Info("ArgoCD MCP Server started. Waiting for requests on stdin...")
	if err := mcp_server.ServeStdio(s); err != nil {
		log.WithError(err).Error("Server error")
		_ = argo.Close()
		os.Exit(1)
	}
}
