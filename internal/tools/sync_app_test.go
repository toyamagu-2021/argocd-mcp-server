package tools

import (
	"testing"
)

func TestSyncAppTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if SyncAppTool.Name != "sync_application" {
		t.Errorf("Expected tool name 'sync_application', got %s", SyncAppTool.Name)
	}

	// Verify tool has description
	if SyncAppTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Check input schema exists
	if SyncAppTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", SyncAppTool.InputSchema.Type)
	}

	// Check that we have properties defined
	if len(SyncAppTool.InputSchema.Properties) == 0 {
		t.Error("Tool should have properties defined")
	}

	// Check that we have required fields defined
	if len(SyncAppTool.InputSchema.Required) == 0 {
		t.Error("Tool should have required fields defined (name should be required)")
	}
}
