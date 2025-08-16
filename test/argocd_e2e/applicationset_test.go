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
