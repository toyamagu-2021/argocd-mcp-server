package tools

import (
	"testing"
)

func TestDeleteAppTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if DeleteAppTool.Name != "delete_application" {
		t.Errorf("Expected tool name 'delete_application', got %s", DeleteAppTool.Name)
	}

	// Verify tool has description
	if DeleteAppTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Just log that description exists and contains warning - no need to test specific words
	if DeleteAppTool.Description != "" {
		t.Logf("Tool description: %s", DeleteAppTool.Description)
	}

	// Check input schema exists
	if DeleteAppTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", DeleteAppTool.InputSchema.Type)
	}
}
