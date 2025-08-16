package tools

import (
	"github.com/mark3labs/mcp-go/server"
)

// RegisterAll registers all defined tools with the MCP server
func RegisterAll(s *server.MCPServer) {
	// Register list_application tool
	s.AddTool(ListAppsTool, HandleListApplications)

	// Register get_application tool
	s.AddTool(GetAppTool, HandleGetApplication)

	// Register get_application_manifests tool
	s.AddTool(GetAppManifestsTool, HandleGetApplicationManifests)

	// Register get_application_events tool
	s.AddTool(GetAppEventsTool, HandleGetApplicationEvents)

	// Register get_application_logs tool
	s.AddTool(GetApplicationLogsToolDefinition, HandleGetApplicationLogs)

	// Register create_application tool
	s.AddTool(CreateAppTool, HandleCreateApplication)

	// Register sync_application tool
	s.AddTool(SyncAppTool, HandleSyncApplication)

	// Register delete_application tool
	s.AddTool(DeleteAppTool, HandleDeleteApplication)

	// Register list_project tool
	s.AddTool(ListProjectsTool, HandleListProjects)

	// Register get_project tool
	s.AddTool(GetProjectTool, HandleGetProject)

	// Register create_project tool
	s.AddTool(CreateProjectTool, HandleCreateProject)

	// Register list_cluster tool
	s.AddTool(ListClusterTool, HandleListCluster)

	// Register get_cluster tool
	s.AddTool(GetClusterTool, HandleGetCluster)

	// Register list_applicationset tool
	s.AddTool(ListApplicationSetTool, HandleListApplicationSets)

	// Register get_applicationset tool
	s.AddTool(GetApplicationSetTool, HandleGetApplicationSet)
}
