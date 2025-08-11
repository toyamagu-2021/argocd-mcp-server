package argocde2e

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// testListClusters tests the list_cluster tool
func testListClusters(t *testing.T) {
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
			"name":      "list_cluster",
			"arguments": map[string]interface{}{},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		t.Logf("Error response: %v", errObj)
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "permission") {
			t.Skip("No permission to list clusters on this ArgoCD server")
		}
		t.Fatalf("Error calling list_cluster: %v", errObj["message"])
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

	// Parse the JSON response to verify it contains clusters
	var clusterList map[string]interface{}
	if err := json.Unmarshal([]byte(text), &clusterList); err != nil {
		t.Fatalf("failed to parse cluster list JSON: %v", err)
	}

	items, ok := clusterList["items"].([]interface{})
	if !ok {
		t.Fatalf("expected items to be an array, got %T", clusterList["items"])
	}

	// Check for the default in-cluster
	foundInCluster := false
	for _, cluster := range items {
		if clusterMap, ok := cluster.(map[string]interface{}); ok {
			if server, ok := clusterMap["server"].(string); ok && server == "https://kubernetes.default.svc" {
				foundInCluster = true
				t.Logf("Found in-cluster: %v", clusterMap["name"])
				break
			}
		}
	}

	if !foundInCluster {
		t.Error("Expected to find https://kubernetes.default.svc cluster in the list")
	}

	t.Logf("Successfully listed %d clusters from real ArgoCD server", len(items))
	t.Logf("Response snippet: %.500s...", text)
}

// testGetCluster tests the get_cluster tool
func testGetCluster(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	// Test getting the in-cluster
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_cluster",
			"arguments": map[string]interface{}{
				"server": "https://kubernetes.default.svc",
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		t.Logf("Error response: %v", errObj)
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "permission") {
			t.Skip("No permission to get cluster on this ArgoCD server")
		}
		t.Fatalf("Error calling get_cluster: %v", errObj["message"])
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

	// Parse the JSON response to verify it contains the cluster
	var cluster map[string]interface{}
	if err := json.Unmarshal([]byte(text), &cluster); err != nil {
		t.Fatalf("failed to parse cluster JSON: %v", err)
	}

	// Verify the cluster server
	if server, ok := cluster["server"].(string); !ok || server != "https://kubernetes.default.svc" {
		t.Errorf("expected server to be https://kubernetes.default.svc, got %v", server)
	}

	// Check for cluster name
	if name, ok := cluster["name"].(string); ok {
		t.Logf("Cluster name: %s", name)
	}

	t.Logf("Successfully retrieved cluster details from real ArgoCD server")
	t.Logf("Response snippet: %.500s...", text)
}

// testGetClusterNotFound tests the get_cluster tool with a non-existent cluster
func testGetClusterNotFound(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	// Test getting a non-existent cluster
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_cluster",
			"arguments": map[string]interface{}{
				"server": "https://non-existent-cluster.example.com",
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	// We expect an error for non-existent cluster
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	isError, ok := result["isError"].(bool)
	if !ok || !isError {
		t.Error("expected isError to be true for non-existent cluster")
	}

	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatal("expected error content")
	}

	textContent, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected content[0] to be a map, got %T", content[0])
	}

	errorText, ok := textContent["text"].(string)
	if !ok {
		t.Fatalf("expected text to be a string, got %T", textContent["text"])
	}

	// ArgoCD may return either "not found" or "permission denied" for non-existent clusters
	// depending on the authorization model to avoid information disclosure
	if !strings.Contains(errorText, "not found") && 
		!strings.Contains(errorText, "NotFound") && 
		!strings.Contains(errorText, "permission denied") &&
		!strings.Contains(errorText, "PermissionDenied") {
		t.Errorf("expected error message to contain 'not found' or 'permission denied', got: %s", errorText)
	}

	t.Logf("Successfully handled non-existent cluster error")
}

// testListClustersGRPCWeb tests the list_cluster tool with gRPC-Web
func testListClustersGRPCWeb(t *testing.T) {
	t.Setenv("ARGOCD_GRPC_WEB", "true")
	testListClusters(t)
}

// testGetClusterGRPCWeb tests the get_cluster tool with gRPC-Web
func testGetClusterGRPCWeb(t *testing.T) {
	t.Setenv("ARGOCD_GRPC_WEB", "true")
	testGetCluster(t)
}

// testGetClusterNotFoundGRPCWeb tests the get_cluster tool with a non-existent cluster using gRPC-Web
func testGetClusterNotFoundGRPCWeb(t *testing.T) {
	t.Setenv("ARGOCD_GRPC_WEB", "true")
	testGetClusterNotFound(t)
}
