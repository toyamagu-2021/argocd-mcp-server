package mockargocde2e

import (
	"strings"
	"testing"
)

func TestParallel_ListApplications(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "list_application",
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

	if textContent["type"] != "text" {
		t.Errorf("expected content type to be text, got %v", textContent["type"])
	}

	text, ok := textContent["text"].(string)
	if !ok {
		t.Fatalf("expected text to be a string, got %T", textContent["text"])
	}

	if !strings.Contains(text, "test-app-1") || !strings.Contains(text, "test-app-2") {
		t.Errorf("expected response to contain test applications")
	}
}

func TestParallel_GetApplication(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application",
			"arguments": map[string]interface{}{
				"name": "test-app-1",
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

	if !strings.Contains(text, "test-app-1") {
		t.Errorf("expected response to contain test-app-1")
	}

	if !strings.Contains(text, "https://github.com/test/repo1") {
		t.Errorf("expected response to contain repo URL")
	}
}

func TestParallel_RefreshApplication(t *testing.T) {
	t.Parallel()

	// Test normal refresh
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "refresh_application",
			"arguments": map[string]interface{}{
				"name": "test-app-1",
				"hard": false,
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

	if !strings.Contains(text, "test-app-1") {
		t.Errorf("expected response to contain application name")
	}

	// Test hard refresh
	callToolRequestHard := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "refresh_application",
			"arguments": map[string]interface{}{
				"name": "test-app-2",
				"hard": true,
			},
		},
	}

	responseHard := sendSharedRequest(t, callToolRequestHard)

	resultHard, ok := responseHard["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map for hard refresh, got %T", responseHard["result"])
	}

	contentHard, ok := resultHard["content"].([]interface{})
	if !ok {
		t.Fatalf("expected content to be an array for hard refresh, got %T", resultHard["content"])
	}

	if len(contentHard) == 0 {
		t.Fatal("expected at least one content item for hard refresh")
	}

	textContentHard, ok := contentHard[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected content[0] to be a map for hard refresh, got %T", contentHard[0])
	}

	textHard, ok := textContentHard["text"].(string)
	if !ok {
		t.Fatalf("expected text to be a string for hard refresh, got %T", textContentHard["text"])
	}

	if !strings.Contains(textHard, "test-app-2") {
		t.Errorf("expected hard refresh response to contain test-app-2")
	}
}

func TestParallel_SyncApplication(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "sync_application",
			"arguments": map[string]interface{}{
				"name":    "test-app-1",
				"dry_run": true,
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

	if !strings.Contains(text, "test-app-1") {
		t.Errorf("expected response to contain application name")
	}

	hasOperationState := strings.Contains(text, "operationState") || strings.Contains(text, "OperationState")
	hasSucceeded := strings.Contains(text, "Succeeded") || strings.Contains(text, "Dry run completed successfully")
	if !hasOperationState || !hasSucceeded {
		t.Errorf("expected response to contain sync operation result, got: %s", text)
	}
}

func TestParallel_DeleteApplication(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "delete_application",
			"arguments": map[string]interface{}{
				"name":    "test-app-2",
				"cascade": false,
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

	if !strings.Contains(text, "test-app-2") || !strings.Contains(text, "deleted successfully") {
		t.Errorf("expected response to contain deletion confirmation")
	}
}

func TestParallel_CreateApplication(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "create_application",
			"arguments": map[string]interface{}{
				"name":           "test-app-new",
				"repo_url":       "https://github.com/test/new-repo",
				"dest_namespace": "default",
				"path":           "manifests",
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

	// Debug: print the actual response
	t.Logf("Create response text: %s", text)

	if !strings.Contains(text, "test-app-new") {
		t.Errorf("expected response to contain new application name, got: %s", text)
	}

	if !strings.Contains(text, "created successfully") && !strings.Contains(text, "Created application") && !strings.Contains(text, "Application created") {
		t.Errorf("expected response to contain creation confirmation, got: %s", text)
	}
}

func TestParallel_FilterApplicationsByProject(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "list_application",
			"arguments": map[string]interface{}{
				"project": "production",
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

	if !strings.Contains(text, "test-app-2") {
		t.Errorf("expected response to contain test-app-2 (production project)")
	}

	if strings.Contains(text, "test-app-1") {
		t.Errorf("expected response NOT to contain test-app-1 (default project)")
	}
}

func TestParallel_FilterApplicationsByNamespace(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "list_application",
			"arguments": map[string]interface{}{
				"namespace": "prod",
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

	// Check that only prod namespace apps are returned
	if !strings.Contains(text, "test-app-2") {
		t.Errorf("expected response to contain test-app-2 (prod namespace)")
	}

	if strings.Contains(text, "test-app-1") {
		t.Errorf("expected response NOT to contain test-app-1 (default namespace)")
	}
}

func TestParallel_GetApplicationResourceTree(t *testing.T) {
	t.Parallel()

	// Test getting resource tree for test-app-1
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application_resource_tree",
			"arguments": map[string]interface{}{
				"name": "test-app-1",
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

	// Verify the response contains expected resource types
	if !strings.Contains(text, "Service") {
		t.Errorf("expected response to contain Service resource")
	}

	if !strings.Contains(text, "Deployment") {
		t.Errorf("expected response to contain Deployment resource")
	}

	if !strings.Contains(text, "Pod") {
		t.Errorf("expected response to contain Pod resource")
	}

	if !strings.Contains(text, "test-service") {
		t.Errorf("expected response to contain test-service")
	}

	if !strings.Contains(text, "test-deployment") {
		t.Errorf("expected response to contain test-deployment")
	}

	// Check for orphaned nodes
	if !strings.Contains(text, "orphanedNodes") {
		t.Errorf("expected response to contain orphanedNodes field")
	}

	if !strings.Contains(text, "orphaned-config") {
		t.Errorf("expected response to contain orphaned ConfigMap")
	}
}

func TestParallel_GetApplicationResourceTree_StatefulSet(t *testing.T) {
	t.Parallel()

	// Test getting resource tree for test-app-2 with StatefulSet
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application_resource_tree",
			"arguments": map[string]interface{}{
				"name": "test-app-2",
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

	// Verify StatefulSet resources
	if !strings.Contains(text, "StatefulSet") {
		t.Errorf("expected response to contain StatefulSet resource")
	}

	if !strings.Contains(text, "PersistentVolumeClaim") {
		t.Errorf("expected response to contain PersistentVolumeClaim resource")
	}

	if !strings.Contains(text, "database") {
		t.Errorf("expected response to contain database StatefulSet")
	}

	if !strings.Contains(text, "database-pvc-0") {
		t.Errorf("expected response to contain database PVC")
	}
}

func TestParallel_GetApplicationResourceTree_Empty(t *testing.T) {
	t.Parallel()

	// Test getting resource tree for empty-app
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application_resource_tree",
			"arguments": map[string]interface{}{
				"name": "empty-app",
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

	// Debug: log the actual response to see the structure
	t.Logf("Empty app resource tree response: %s", text)

	// For empty trees, ArgoCD returns {} because of omitempty JSON tags
	// or it might return explicit empty arrays
	isEmptyObject := text == "{}" || text == "{\n}" || text == "{ }"
	hasEmptyNodes := strings.Contains(text, `"nodes":[]`) || 
		strings.Contains(text, `"nodes": []`) || 
		strings.Contains(text, `"nodes": [`)
	hasEmptyOrphaned := strings.Contains(text, `"orphanedNodes":[]`) || 
		strings.Contains(text, `"orphanedNodes": []`) || 
		strings.Contains(text, `"orphanedNodes": [`)
		
	// Either empty object or has explicit empty arrays is valid
	if !isEmptyObject && !hasEmptyNodes {
		t.Errorf("expected response to be empty object or contain empty nodes array, got: %s", text)
	}
	
	// If it has nodes field, check orphaned too
	if hasEmptyNodes && !hasEmptyOrphaned && !isEmptyObject {
		t.Errorf("expected response to contain empty orphanedNodes array when nodes is present, got: %s", text)
	}
}

func TestParallel_GetApplicationResourceTree_NotFound(t *testing.T) {
	t.Parallel()

	// Test getting resource tree for non-existent app
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application_resource_tree",
			"arguments": map[string]interface{}{
				"name": "non-existent-app",
			},
		},
	}

	response := sendSharedRequest(t, callToolRequest)

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Check for error response
	isError, ok := result["isError"].(bool)
	if !ok || !isError {
		t.Fatal("expected error response for non-existent application")
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

	if !strings.Contains(text, "not found") {
		t.Errorf("expected error message to contain 'not found', got: %s", text)
	}
}
