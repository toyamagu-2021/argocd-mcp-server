package mockargocde2e

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParallel_ListClusters(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "list_cluster",
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

	// Parse the JSON response to verify it contains clusters
	// By default (detailed=false), list_cluster returns an array directly, not wrapped in an object
	var clusters []interface{}
	if err := json.Unmarshal([]byte(text), &clusters); err != nil {
		t.Fatalf("failed to parse cluster list JSON: %v", err)
	}

	// Check for expected mock clusters
	expectedClusters := map[string]bool{
		"https://kubernetes.default.svc":       false,
		"https://external-cluster.example.com": false,
	}

	for _, cluster := range clusters {
		if clusterMap, ok := cluster.(map[string]interface{}); ok {
			if server, ok := clusterMap["server"].(string); ok {
				if _, expected := expectedClusters[server]; expected {
					expectedClusters[server] = true
				}
			}
		}
	}

	for server, found := range expectedClusters {
		if !found {
			t.Errorf("Expected cluster %s was not found in the list", server)
		}
	}

	t.Logf("Successfully listed %d clusters", len(clusters))
}

func TestParallel_GetCluster(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_cluster",
			"arguments": map[string]interface{}{
				"server": "https://kubernetes.default.svc",
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

	// Verify the cluster details
	if server, ok := cluster["server"].(string); !ok || server != "https://kubernetes.default.svc" {
		t.Errorf("expected server to be https://kubernetes.default.svc, got %v", server)
	}

	if name, ok := cluster["name"].(string); !ok || name != "in-cluster" {
		t.Errorf("expected name to be in-cluster, got %v", name)
	}

	// Check serverVersion which is nested under info
	if info, ok := cluster["info"].(map[string]interface{}); ok {
		if version, ok := info["serverVersion"].(string); !ok || version != "1.28" {
			t.Errorf("expected serverVersion to be 1.28, got %v", version)
		}
	} else {
		t.Errorf("expected info field to be present and be a map, got %T", cluster["info"])
	}

	t.Logf("Successfully retrieved cluster details")
}

func TestParallel_GetClusterNotFound(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_cluster",
			"arguments": map[string]interface{}{
				"server": "https://non-existent-cluster.example.com",
			},
		},
	}

	response := sendSharedRequest(t, callToolRequest)

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

	if !strings.Contains(errorText, "not found") {
		t.Errorf("expected error message to contain 'not found', got: %s", errorText)
	}

	t.Logf("Successfully handled non-existent cluster error")
}

func TestParallel_GetClusterMissingParameter(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "get_cluster",
			"arguments": map[string]interface{}{},
		},
	}

	response := sendSharedRequest(t, callToolRequest)

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	isError, ok := result["isError"].(bool)
	if !ok || !isError {
		t.Error("expected isError to be true for missing parameter")
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

	if !strings.Contains(errorText, "server is required") {
		t.Errorf("expected error message to contain 'server is required', got: %s", errorText)
	}

	t.Logf("Successfully handled missing parameter error")
}
