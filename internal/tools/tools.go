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
}
