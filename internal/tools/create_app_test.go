package tools

import (
	"testing"
)

func TestCreateAppTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if CreateAppTool.Name != "create_application" {
		t.Errorf("Expected tool name 'create_application', got %s", CreateAppTool.Name)
	}

	// Verify tool has description
	if CreateAppTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Check input schema exists
	if CreateAppTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", CreateAppTool.InputSchema.Type)
	}

	// Check that we have properties defined
	if len(CreateAppTool.InputSchema.Properties) == 0 {
		t.Error("Tool schema should have properties defined")
	}

	// Check required fields are marked as required
	expectedRequired := []string{"name", "repo_url", "dest_namespace"}
	if CreateAppTool.InputSchema.Required == nil {
		t.Error("Tool schema should have required fields defined")
	} else {
		for _, req := range expectedRequired {
			found := false
			for _, actual := range CreateAppTool.InputSchema.Required {
				if actual == req {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected required field '%s' not found in schema", req)
			}
		}
	}
}
