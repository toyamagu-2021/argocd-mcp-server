package mockargocde2e

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParallel_ListProjects(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "list_project",
			"arguments": map[string]interface{}{},
		},
	}

	response := sendSharedRequest(t, callToolRequest)

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	content, ok := result["content"].([]interface{})
	if !ok {
		t.Fatalf("expected content to be an array, got %T", result["content"])
	}

	if len(content) == 0 {
		t.Fatal("expected at least one content item")
	}

	textContent, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected content[0] to be a map, got %T", content[0])
	}

	text, ok := textContent["text"].(string)
	if !ok {
		t.Fatalf("expected text to be a string, got %T", textContent["text"])
	}

	// Log the response for debugging
	t.Logf("Response text: %s", text)

	// Check if it's an error message
	if strings.Contains(text, "Failed to") || strings.Contains(text, "Error") {
		t.Fatalf("Error response: %s", text)
	}

	// Parse the JSON response to verify it contains projects
	var projects []interface{}
	if err := json.Unmarshal([]byte(text), &projects); err != nil {
		t.Fatalf("failed to parse projects JSON: %v, text: %s", err, text)
	}

	// Verify we have expected number of projects
	if len(projects) != 3 {
		t.Errorf("expected 3 projects, got %d", len(projects))
	}

	// Check for expected project names
	expectedProjects := map[string]bool{
		"default":     false,
		"production":  false,
		"development": false,
	}

	for _, proj := range projects {
		if projMap, ok := proj.(map[string]interface{}); ok {
			if metadata, ok := projMap["metadata"].(map[string]interface{}); ok {
				if name, ok := metadata["name"].(string); ok {
					if _, exists := expectedProjects[name]; exists {
						expectedProjects[name] = true
					}
				}
			}
		}
	}

	for name, found := range expectedProjects {
		if !found {
			t.Errorf("expected project %s not found in list", name)
		}
	}

	t.Logf("Successfully listed %d projects from mock server", len(projects))
}

func TestParallel_GetProject(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		projectName  string
		expectError  bool
		expectedDesc string
	}{
		{
			name:         "get default project",
			projectName:  "default",
			expectError:  false,
			expectedDesc: "Default project",
		},
		{
			name:         "get production project",
			projectName:  "production",
			expectError:  false,
			expectedDesc: "Production project",
		},
		{
			name:         "get development project",
			projectName:  "development",
			expectError:  false,
			expectedDesc: "Development project",
		},
		{
			name:        "get non-existent project",
			projectName: "non-existent",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			callToolRequest := map[string]interface{}{
				"jsonrpc": "2.0",
				"method":  "tools/call",
				"params": map[string]interface{}{
					"name": "get_project",
					"arguments": map[string]interface{}{
						"name": tc.projectName,
					},
				},
			}

			response := sendSharedRequest(t, callToolRequest)

			result, ok := response["result"].(map[string]interface{})
			if !ok {
				t.Fatalf("expected result to be a map, got %T", response["result"])
			}

			content, ok := result["content"].([]interface{})
			if !ok {
				t.Fatalf("expected content to be an array, got %T", result["content"])
			}

			if len(content) == 0 {
				t.Fatal("expected at least one content item")
			}

			textContent, ok := content[0].(map[string]interface{})
			if !ok {
				t.Fatalf("expected content[0] to be a map, got %T", content[0])
			}

			text, ok := textContent["text"].(string)
			if !ok {
				t.Fatalf("expected text to be a string, got %T", textContent["text"])
			}

			if tc.expectError {
				if !strings.Contains(strings.ToLower(text), "not found") && !strings.Contains(strings.ToLower(text), "error") {
					t.Errorf("expected error message for non-existent project, got: %s", text)
				}
			} else {
				// Parse the JSON response to verify it's a valid project
				var project map[string]interface{}
				if err := json.Unmarshal([]byte(text), &project); err != nil {
					t.Fatalf("failed to parse project JSON: %v", err)
				}

				// Verify the project has expected fields
				if metadata, ok := project["metadata"].(map[string]interface{}); ok {
					if name, ok := metadata["name"].(string); ok {
						if name != tc.projectName {
							t.Errorf("expected project name %s, got %s", tc.projectName, name)
						}
					} else {
						t.Error("project metadata.name not found")
					}
				} else {
					t.Error("project metadata not found")
				}

				if spec, ok := project["spec"].(map[string]interface{}); ok {
					if desc, ok := spec["description"].(string); ok {
						if desc != tc.expectedDesc {
							t.Errorf("expected description '%s', got '%s'", tc.expectedDesc, desc)
						}
					}

					// Check for common project spec fields
					if _, ok := spec["sourceRepos"]; !ok {
						t.Error("sourceRepos not found in project spec")
					}
					if _, ok := spec["destinations"]; !ok {
						t.Error("destinations not found in project spec")
					}
				} else {
					t.Error("project spec not found")
				}

				t.Logf("Successfully retrieved project %s from mock server", tc.projectName)
			}
		})
	}
}

func TestParallel_GetProjectWithDetails(t *testing.T) {
	t.Parallel()

	// Test getting production project with detailed verification
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_project",
			"arguments": map[string]interface{}{
				"name": "production",
			},
		},
	}

	response := sendSharedRequest(t, callToolRequest)

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	content, ok := result["content"].([]interface{})
	if !ok {
		t.Fatalf("expected content to be an array, got %T", result["content"])
	}

	if len(content) == 0 {
		t.Fatal("expected at least one content item")
	}

	textContent, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected content[0] to be a map, got %T", content[0])
	}

	text, ok := textContent["text"].(string)
	if !ok {
		t.Fatalf("expected text to be a string, got %T", textContent["text"])
	}

	// Parse and verify production project details
	var project map[string]interface{}
	if err := json.Unmarshal([]byte(text), &project); err != nil {
		t.Fatalf("failed to parse project JSON: %v", err)
	}

	spec, ok := project["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("project spec not found")
	}

	// Verify source repos
	sourceRepos, ok := spec["sourceRepos"].([]interface{})
	if !ok {
		t.Fatal("sourceRepos not found or wrong type")
	}
	if len(sourceRepos) != 2 {
		t.Errorf("expected 2 source repos, got %d", len(sourceRepos))
	}

	// Verify destinations
	destinations, ok := spec["destinations"].([]interface{})
	if !ok {
		t.Fatal("destinations not found or wrong type")
	}
	if len(destinations) != 1 {
		t.Errorf("expected 1 destination, got %d", len(destinations))
	}

	// Verify cluster resource whitelist
	whitelist, ok := spec["clusterResourceWhitelist"].([]interface{})
	if !ok {
		t.Fatal("clusterResourceWhitelist not found or wrong type")
	}
	if len(whitelist) != 3 {
		t.Errorf("expected 3 whitelisted resources, got %d", len(whitelist))
	}

	// Verify roles
	roles, ok := spec["roles"].([]interface{})
	if !ok {
		t.Fatal("roles not found or wrong type")
	}
	if len(roles) != 1 {
		t.Errorf("expected 1 role, got %d", len(roles))
	}

	t.Log("Successfully verified production project details")
}
