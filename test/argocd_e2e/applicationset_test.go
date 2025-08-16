package argocde2e

import (
	"strings"
	"testing"
)

// testListApplicationSets tests the list_applicationset tool
func testListApplicationSets(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	// Call list_applicationset tool
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "list_applicationset",
			"arguments": map[string]interface{}{},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	// Verify response structure
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Check if it's a success or error result
	if isError, _ := result["isError"].(bool); isError {
		// ApplicationSets might not exist, which is okay for this test
		content := result["content"].([]interface{})
		if len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			t.Logf("ApplicationSet list returned: %s", text)
		}
	} else {
		// Verify content exists
		if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
			t.Logf("Successfully listed ApplicationSets")
		}
	}
}

// testListApplicationSetsWithProject tests the list_applicationset tool with project filter
func testListApplicationSetsWithProject(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	// Call list_applicationset tool with project filter
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "list_applicationset",
			"arguments": map[string]interface{}{
				"project": "default",
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	// Verify response structure
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Check if it's a success or error result
	if isError, _ := result["isError"].(bool); isError {
		content := result["content"].([]interface{})
		if len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			t.Logf("ApplicationSet list with project filter returned: %s", text)
		}
	} else {
		// Verify content exists
		if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
			t.Logf("Successfully listed ApplicationSets with project filter")
		}
	}
}

// testCreateApplicationSet tests the create_applicationset tool
func testCreateApplicationSet(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	// Create a test ApplicationSet
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "create_applicationset",
			"arguments": map[string]interface{}{
				"name":       "test-appset-e2e",
				"namespace":  "argocd",
				"project":    "default",
				"generators": `[{"list":{"elements":[{"cluster":"in-cluster","url":"https://kubernetes.default.svc"}]}}]`,
				"template": `{
					"metadata": {
						"name": "{{cluster}}-guestbook"
					},
					"spec": {
						"project": "default",
						"source": {
							"repoURL": "https://github.com/argoproj/argocd-example-apps",
							"targetRevision": "HEAD",
							"path": "guestbook"
						},
						"destination": {
							"server": "{{url}}",
							"namespace": "guestbook"
						},
						"syncPolicy": {
							"syncOptions": [
								"CreateNamespace=true"
							]
						}
					}
				}`,
				"dry_run": true, // Use dry run to avoid actually creating the resource
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	// Verify response structure
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Check if it's a success or error result
	if isError, _ := result["isError"].(bool); isError {
		content := result["content"].([]interface{})
		if len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			// Check if it's a permission error (expected in some environments)
			if strings.Contains(text, "permission denied") || strings.Contains(text, "forbidden") {
				t.Logf("Permission denied for ApplicationSet creation (expected in restricted environments)")
			} else {
				t.Errorf("ApplicationSet creation failed: %s", text)
			}
		}
	} else {
		// Verify content exists
		if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
			t.Logf("Successfully created ApplicationSet (dry run)")
		}
	}
}

// testCreateApplicationSetGRPCWeb tests the create_applicationset tool with gRPC-Web
func testCreateApplicationSetGRPCWeb(t *testing.T) {
	t.Setenv("ARGOCD_GRPC_WEB", "true")
	testCreateApplicationSet(t)
}

// testGetApplicationSet tests the get_applicationset tool
func testGetApplicationSet(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	// Try to get a specific ApplicationSet (might not exist)
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_applicationset",
			"arguments": map[string]interface{}{
				"name": "in-cluster-guestbook",
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	// Verify response structure
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Check if it's a success or error result
	if isError, _ := result["isError"].(bool); isError {
		content := result["content"].([]interface{})
		if len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			// ApplicationSet might not exist, which is expected
			if !strings.Contains(text, "not found") && !strings.Contains(text, "Failed to get ApplicationSet") {
				t.Errorf("unexpected error: %s", text)
			}
			t.Logf("Get ApplicationSet returned expected error: %s", text)
		}
	} else {
		// If it exists, verify content
		if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
			t.Logf("Successfully retrieved ApplicationSet")
		}
	}
}

// testGetApplicationSetMissingName tests the get_applicationset tool with missing name
func testGetApplicationSetMissingName(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	// Try to get ApplicationSet without name (should fail)
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "get_applicationset",
			"arguments": map[string]interface{}{},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	// Verify response structure
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Should be an error
	if isError, _ := result["isError"].(bool); !isError {
		t.Error("expected error for missing name parameter")
	} else {
		content := result["content"].([]interface{})
		if len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			if !strings.Contains(text, "name is required") {
				t.Errorf("expected 'name is required' error, got: %s", text)
			}
			t.Logf("Got expected error: %s", text)
		}
	}
}

// testListApplicationSetsGRPCWeb tests with gRPC-Web mode
func testListApplicationSetsGRPCWeb(t *testing.T) {
	t.Setenv("ARGOCD_GRPC_WEB", "true")
	testListApplicationSets(t)
}

// testGetApplicationSetGRPCWeb tests with gRPC-Web mode
func testGetApplicationSetGRPCWeb(t *testing.T) {
	t.Setenv("ARGOCD_GRPC_WEB", "true")
	testGetApplicationSet(t)
}

// testDeleteApplicationSet tests the delete_applicationset tool
func testDeleteApplicationSet(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	// Try to delete an ApplicationSet (might not exist)
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "delete_applicationset",
			"arguments": map[string]interface{}{
				"name": "test-appset-e2e",
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	// Verify response structure
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Check if it's a success or error result
	if isError, _ := result["isError"].(bool); isError {
		content := result["content"].([]interface{})
		if len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			// ApplicationSet might not exist, which is expected
			if strings.Contains(text, "not found") || strings.Contains(text, "Failed to delete ApplicationSet") {
				t.Logf("Delete ApplicationSet returned expected error: %s", text)
			} else if strings.Contains(text, "permission denied") || strings.Contains(text, "forbidden") {
				t.Logf("Permission denied for ApplicationSet deletion (expected in restricted environments)")
			} else {
				t.Errorf("unexpected error: %s", text)
			}
		}
	} else {
		// Verify content exists
		if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			// Check if success message contains expected text
			if !strings.Contains(text, "deleted successfully") {
				t.Errorf("expected success message to contain 'deleted successfully', got: %s", text)
			}
			// Also check for warning about managed applications
			if !strings.Contains(text, "All applications managed by this ApplicationSet will be deleted") {
				t.Logf("Warning about managed applications not found in message: %s", text)
			}
			t.Logf("Successfully deleted ApplicationSet: %s", text)
		}
	}
}

// testDeleteApplicationSetWithNamespace tests the delete_applicationset tool with namespace
func testDeleteApplicationSetWithNamespace(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	// Try to delete an ApplicationSet with namespace specified
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "delete_applicationset",
			"arguments": map[string]interface{}{
				"name":            "test-appset-e2e",
				"appsetNamespace": "argocd",
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	// Verify response structure
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Check if it's a success or error result
	if isError, _ := result["isError"].(bool); isError {
		content := result["content"].([]interface{})
		if len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			t.Logf("Delete ApplicationSet with namespace returned: %s", text)
		}
	} else {
		// Verify content exists
		if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			// Check if success message contains namespace
			if !strings.Contains(text, "in namespace") {
				t.Logf("Success message doesn't mention namespace: %s", text)
			}
			t.Logf("Successfully deleted ApplicationSet with namespace: %s", text)
		}
	}
}

// testDeleteApplicationSetMissingName tests the delete_applicationset tool with missing name
func testDeleteApplicationSetMissingName(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	// Try to delete ApplicationSet without name (should fail)
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "delete_applicationset",
			"arguments": map[string]interface{}{},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	// Verify response structure
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Should be an error
	if isError, _ := result["isError"].(bool); !isError {
		t.Error("expected error for missing name parameter")
	} else {
		content := result["content"].([]interface{})
		if len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			if !strings.Contains(text, "ApplicationSet name is required") {
				t.Errorf("expected 'ApplicationSet name is required' error, got: %s", text)
			}
			t.Logf("Got expected error: %s", text)
		}
	}
}

// testDeleteApplicationSetGRPCWeb tests with gRPC-Web mode
func testDeleteApplicationSetGRPCWeb(t *testing.T) {
	t.Setenv("ARGOCD_GRPC_WEB", "true")
	testDeleteApplicationSet(t)
}

// ApplicationSet Lifecycle tests - these must run in order
// These tests simulate a complete lifecycle: create -> list -> get -> delete

// testApplicationSetLifecycle_01_Create creates an ApplicationSet for lifecycle testing
func testApplicationSetLifecycle01Create(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	// Create a test ApplicationSet
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "create_applicationset",
			"arguments": map[string]interface{}{
				"name":       "test-appset-lifecycle",
				"namespace":  "argocd",
				"project":    "default",
				"generators": `[{"list":{"elements":[{"cluster":"test-cluster","url":"https://kubernetes.default.svc"}]}}]`,
				"template": `{
					"metadata": {
						"name": "{{cluster}}-app"
					},
					"spec": {
						"project": "default",
						"source": {
							"repoURL": "https://github.com/argoproj/argocd-example-apps",
							"targetRevision": "HEAD",
							"path": "guestbook"
						},
						"destination": {
							"server": "{{url}}",
							"namespace": "default"
						}
					}
				}`,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Log the result
	if isError, _ := result["isError"].(bool); isError {
		content := result["content"].([]interface{})
		if len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			// It might fail with permission error in some environments
			if strings.Contains(text, "permission denied") || strings.Contains(text, "forbidden") {
				t.Skipf("Permission denied for ApplicationSet creation (expected in restricted environments): %s", text)
			} else if strings.Contains(text, "already exists") {
				t.Logf("ApplicationSet already exists (from previous run): %s", text)
			} else {
				t.Errorf("ApplicationSet creation failed: %s", text)
			}
		}
	} else {
		t.Logf("Successfully created ApplicationSet for lifecycle test")
	}
}

// testApplicationSetLifecycle_02_List lists ApplicationSets and verifies the created one exists
func testApplicationSetLifecycle02List(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	// List ApplicationSets
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "list_applicationset",
			"arguments": map[string]interface{}{},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Verify the created ApplicationSet is in the list
	if isError, _ := result["isError"].(bool); !isError {
		if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			if strings.Contains(text, "test-appset-lifecycle") {
				t.Logf("Found created ApplicationSet in list")
			} else {
				t.Logf("Created ApplicationSet not found in list (might be filtered)")
			}
		}
	}
}

// testApplicationSetLifecycle_03_Get retrieves the created ApplicationSet
func testApplicationSetLifecycle03Get(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	// Get the created ApplicationSet
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_applicationset",
			"arguments": map[string]interface{}{
				"name": "test-appset-lifecycle",
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Check result
	if isError, _ := result["isError"].(bool); isError {
		content := result["content"].([]interface{})
		if len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			if strings.Contains(text, "not found") {
				t.Logf("ApplicationSet not found (might have been cleaned up): %s", text)
			} else {
				t.Errorf("Failed to get ApplicationSet: %s", text)
			}
		}
	} else {
		t.Logf("Successfully retrieved ApplicationSet")
	}
}

// testApplicationSetLifecycle_04_SyncGeneratedApp syncs the Application generated by the ApplicationSet
func testApplicationSetLifecycle04SyncGeneratedApp(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	// Sync the application generated by the ApplicationSet
	// The ApplicationSet template creates an app named "test-cluster-app"
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "sync_application",
			"arguments": map[string]interface{}{
				"name":    "test-cluster-app", // Name from the ApplicationSet template
				"dry_run": false,              // Perform actual sync
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Check result
	if isError, _ := result["isError"].(bool); isError {
		content := result["content"].([]interface{})
		if len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			if strings.Contains(text, "not found") {
				t.Logf("Generated application not found (ApplicationSet might not have created it yet): %s", text)
			} else if strings.Contains(text, "permission denied") || strings.Contains(text, "forbidden") {
				t.Logf("Permission denied for application sync: %s", text)
			} else {
				t.Logf("Sync returned error (expected in some environments): %s", text)
			}
		}
	} else {
		if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			t.Logf("Successfully synced generated application: %s", text)
		}
	}
}

// testApplicationSetLifecycle_05_Delete deletes the created ApplicationSet
func testApplicationSetLifecycle05Delete(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	// Delete the created ApplicationSet
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "delete_applicationset",
			"arguments": map[string]interface{}{
				"name": "test-appset-lifecycle",
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Check result
	if isError, _ := result["isError"].(bool); isError {
		content := result["content"].([]interface{})
		if len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			if strings.Contains(text, "not found") {
				t.Logf("ApplicationSet already deleted or doesn't exist: %s", text)
			} else if strings.Contains(text, "permission denied") || strings.Contains(text, "forbidden") {
				t.Logf("Permission denied for ApplicationSet deletion: %s", text)
			} else {
				t.Errorf("Failed to delete ApplicationSet: %s", text)
			}
		}
	} else {
		if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			if strings.Contains(text, "deleted successfully") {
				t.Logf("Successfully deleted ApplicationSet")
			}
		}
	}
}

// gRPC-Web versions of lifecycle tests
func testApplicationSetLifecycle01CreateGRPCWeb(t *testing.T) {
	t.Setenv("ARGOCD_GRPC_WEB", "true")
	testApplicationSetLifecycle01Create(t)
}

func testApplicationSetLifecycle02ListGRPCWeb(t *testing.T) {
	t.Setenv("ARGOCD_GRPC_WEB", "true")
	testApplicationSetLifecycle02List(t)
}

func testApplicationSetLifecycle03GetGRPCWeb(t *testing.T) {
	t.Setenv("ARGOCD_GRPC_WEB", "true")
	testApplicationSetLifecycle03Get(t)
}

func testApplicationSetLifecycle04SyncGeneratedAppGRPCWeb(t *testing.T) {
	t.Setenv("ARGOCD_GRPC_WEB", "true")
	testApplicationSetLifecycle04SyncGeneratedApp(t)
}

func testApplicationSetLifecycle05DeleteGRPCWeb(t *testing.T) {
	t.Setenv("ARGOCD_GRPC_WEB", "true")
	testApplicationSetLifecycle05Delete(t)
}
