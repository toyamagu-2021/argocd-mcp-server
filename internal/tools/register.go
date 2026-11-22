package tools

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// toolEntry binds a tool schema to the constructor of its handler.
type toolEntry struct {
	tool mcp.Tool
	make func(Deps) server.ToolHandlerFunc
	// mutating tools change Argo CD state and are not registered in read-only mode.
	mutating bool
}

// entries returns every public tool in registration order.
func entries() []toolEntry {
	return []toolEntry{
		{ListAppsTool, makeListApplications, false},
		{GetAppTool, makeGetApplication, false},
		{GetAppManifestsTool, makeGetApplicationManifests, false},
		{GetAppEventsTool, makeGetApplicationEvents, false},
		{GetApplicationLogsToolDefinition, makeGetApplicationLogs, false},
		{GetApplicationResourceTreeTool, makeGetApplicationResourceTree, false},
		{CreateAppTool, makeCreateApplication, true},
		{SyncAppTool, makeSyncApplication, true},
		{RefreshAppTool, makeRefreshApplication, true},
		{DeleteAppTool, makeDeleteApplication, true},
		{TerminateOperationTool, makeTerminateOperation, true},
		{ListProjectsTool, makeListProjects, false},
		{GetProjectTool, makeGetProject, false},
		{CreateProjectTool, makeCreateProject, true},
		{ListClusterTool, makeListCluster, false},
		{GetClusterTool, makeGetCluster, false},
		{ListApplicationSetTool, makeListApplicationSets, false},
		{GetApplicationSetTool, makeGetApplicationSet, false},
		{CreateApplicationSetTool, makeCreateApplicationSet, true},
		{DeleteApplicationSetTool, makeDeleteApplicationSet, true},
		{ListRepositoryTool, makeListRepository, false},
		{GetRepositoryTool, makeGetRepository, false},
		{GetUserInfoTool, makeGetUserInfo, false},
	}
}

// Register adds the tools to s, binding each handler to d.
// When d.ReadOnly is set, tools that change Argo CD state are skipped.
func Register(s *server.MCPServer, d Deps) {
	for _, e := range entries() {
		if d.ReadOnly && e.mutating {
			continue
		}
		s.AddTool(e.tool, e.make(d))
	}
}
