package argocde2e

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestListProjects tests the list_project tool
func testListProjects(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "list_project",
			"arguments": map[string]interface{}{},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		t.Logf("Error response: %v", errObj)
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "permission") {
			t.Skip("No permission to list projects on this ArgoCD server")
		}
		t.Fatalf("Error calling list_project: %v", errObj["message"])
	}

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

	if textContent["type"] != "text" {
		t.Errorf("expected content type to be text, got %v", textContent["type"])
	}

	text, ok := textContent["text"].(string)
	if !ok {
		t.Fatalf("expected text to be a string, got %T", textContent["text"])
	}

	// Parse the JSON response to verify it contains projects
	if !strings.Contains(text, "No projects found") {
		var projects []interface{}
		if err := json.Unmarshal([]byte(text), &projects); err != nil {
			t.Fatalf("failed to parse projects JSON: %v", err)
		}

		// Check for default project
		foundDefault := false
		for _, proj := range projects {
			if projMap, ok := proj.(map[string]interface{}); ok {
				if metadata, ok := projMap["metadata"].(map[string]interface{}); ok {
					if name, ok := metadata["name"].(string); ok && name == "default" {
						foundDefault = true
						break
					}
				}
			}
		}

		if !foundDefault {
			t.Log("Warning: 'default' project not found in list, but this might be expected in some environments")
		}

		t.Logf("Successfully listed %d projects from real ArgoCD server", len(projects))
	} else {
		t.Log("No projects found on ArgoCD server (expected in minimal setups)")
	}

	t.Logf("Response snippet: %.500s...", text)
}

// TestGetProject tests the get_project tool
func testGetProject(t *testing.T) {
	// Try to get the default project which should always exist
	projectName := "default"

	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_project",
			"arguments": map[string]interface{}{
				"name": projectName,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		errorMsg := fmt.Sprintf("%v", errObj["message"])
		t.Logf("Error response: %v", errObj)

		if strings.Contains(errorMsg, "not found") {
			t.Skipf("Project %s not found on this ArgoCD server", projectName)
		}
		if strings.Contains(errorMsg, "permission") {
			t.Skip("No permission to get project on this ArgoCD server")
		}
		t.Fatalf("Error calling get_project: %v", errorMsg)
	}

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

	// Parse the JSON response to verify it's a valid project
	var project map[string]interface{}
	if err := json.Unmarshal([]byte(text), &project); err != nil {
		t.Fatalf("failed to parse project JSON: %v", err)
	}

	// Verify the project has expected fields
	if metadata, ok := project["metadata"].(map[string]interface{}); ok {
		if name, ok := metadata["name"].(string); ok {
			if name != projectName {
				t.Errorf("expected project name %s, got %s", projectName, name)
			}
		} else {
			t.Error("project metadata.name not found")
		}
	} else {
		t.Error("project metadata not found")
	}

	if spec, ok := project["spec"].(map[string]interface{}); ok {
		// Check for common project spec fields
		if _, ok := spec["sourceRepos"]; !ok {
			t.Log("Warning: sourceRepos not found in project spec")
		}
		if _, ok := spec["destinations"]; !ok {
			t.Log("Warning: destinations not found in project spec")
		}
	} else {
		t.Error("project spec not found")
	}

	t.Logf("Successfully retrieved project %s from real ArgoCD server", projectName)
	t.Logf("Response snippet: %.500s...", text)
}

// TestInvalidProjectName tests error handling for non-existent project
func testInvalidProjectName(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_project",
			"arguments": map[string]interface{}{
				"name": "non-existent-project-12345",
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		t.Logf("Received expected error for non-existent project: %v", errObj["message"])
	} else if result, ok := response["result"].(map[string]interface{}); ok {
		content, _ := result["content"].([]interface{})
		if len(content) > 0 {
			textContent, _ := content[0].(map[string]interface{})
			text, _ := textContent["text"].(string)
			if strings.Contains(strings.ToLower(text), "not found") || strings.Contains(strings.ToLower(text), "error") {
				t.Log("Received expected error message for non-existent project")
			} else {
				t.Errorf("Expected error for non-existent project, but got: %s", text)
			}
		}
	}
}

// TestCreateProject tests the create_project tool
func testCreateProject(t *testing.T) {
	// Use a unique project name to avoid conflicts
	projectName := fmt.Sprintf("test-project-%d", time.Now().Unix())

	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	// Create the project
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "create_project",
			"arguments": map[string]interface{}{
				"name":                         projectName,
				"description":                  "Test project created by E2E test",
				"source_repos":                 "https://github.com/example/*,https://gitlab.com/example/*",
				"destination_server":           "https://kubernetes.default.svc",
				"destination_namespace":        "test-*",
				"cluster_resource_whitelist":   "apps:Deployment,batch:Job",
				"namespace_resource_whitelist": ":Service,:ConfigMap,apps:StatefulSet",
				"upsert":                       false,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		errorMsg := fmt.Sprintf("%v", errObj["message"])
		t.Logf("Error response: %v", errObj)

		if strings.Contains(errorMsg, "permission") {
			t.Skip("No permission to create projects on this ArgoCD server")
		}
		if strings.Contains(errorMsg, "already exists") {
			t.Logf("Project %s already exists, trying with upsert", projectName)
			// Retry with upsert
			callToolRequest["params"].(map[string]interface{})["arguments"].(map[string]interface{})["upsert"] = true
			response = sendRequest(t, stdin, stdout, callToolRequest)
			if errObj, ok := response["error"].(map[string]interface{}); ok {
				t.Fatalf("Error calling create_project with upsert: %v", errObj["message"])
			}
		} else {
			t.Fatalf("Error calling create_project: %v", errorMsg)
		}
	}

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

	// Parse the JSON response to verify the created project
	var project map[string]interface{}
	if err := json.Unmarshal([]byte(text), &project); err != nil {
		t.Fatalf("failed to parse project JSON: %v", err)
	}

	// Verify the project has expected fields
	if metadata, ok := project["metadata"].(map[string]interface{}); ok {
		if name, ok := metadata["name"].(string); ok {
			if name != projectName {
				t.Errorf("expected project name %s, got %s", projectName, name)
			}
		}
	}

	if spec, ok := project["spec"].(map[string]interface{}); ok {
		// Verify description
		if desc, ok := spec["description"].(string); ok {
			if desc != "Test project created by E2E test" {
				t.Errorf("unexpected description: %s", desc)
			}
		}

		// Verify source repos
		if sourceRepos, ok := spec["sourceRepos"].([]interface{}); ok {
			expectedRepos := 2
			if len(sourceRepos) != expectedRepos {
				t.Errorf("expected %d source repos, got %d", expectedRepos, len(sourceRepos))
			}
		}

		// Verify destinations
		if destinations, ok := spec["destinations"].([]interface{}); ok {
			if len(destinations) != 1 {
				t.Errorf("expected 1 destination, got %d", len(destinations))
			}
		}
	}

	t.Logf("Successfully created project %s on real ArgoCD server", projectName)
	t.Logf("Response snippet: %.500s...", text)

	// Cleanup: Try to delete the created project
	// Note: This requires delete_project tool which might not be implemented yet
	t.Logf("Note: Manual cleanup may be required for project %s", projectName)
}
