package tools

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// toolEntry binds a tool schema to the constructor of its handler.
type toolEntry struct {
	tool mcp.Tool
	make func(Deps) server.ToolHandlerFunc
}

// entries returns every public tool in registration order.
func entries() []toolEntry {
	return []toolEntry{
		{ListAppsTool, makeListApplications},
		{GetAppTool, makeGetApplication},
		{GetAppManifestsTool, makeGetApplicationManifests},
		{GetAppEventsTool, makeGetApplicationEvents},
		{GetApplicationLogsToolDefinition, makeGetApplicationLogs},
		{GetApplicationResourceTreeTool, makeGetApplicationResourceTree},
		{CreateAppTool, makeCreateApplication},
		{SyncAppTool, makeSyncApplication},
		{RefreshAppTool, makeRefreshApplication},
		{DeleteAppTool, makeDeleteApplication},
		{TerminateOperationTool, makeTerminateOperation},
		{ListProjectsTool, makeListProjects},
		{GetProjectTool, makeGetProject},
		{CreateProjectTool, makeCreateProject},
		{ListClusterTool, makeListCluster},
		{GetClusterTool, makeGetCluster},
		{ListApplicationSetTool, makeListApplicationSets},
		{GetApplicationSetTool, makeGetApplicationSet},
		{CreateApplicationSetTool, makeCreateApplicationSet},
		{DeleteApplicationSetTool, makeDeleteApplicationSet},
		{ListRepositoryTool, makeListRepository},
		{GetRepositoryTool, makeGetRepository},
		{GetUserInfoTool, makeGetUserInfo},
	}
}

// Register adds all tools to s, binding each handler to d.
func Register(s *server.MCPServer, d Deps) {
	for _, e := range entries() {
		s.AddTool(e.tool, e.make(d))
	}
}
