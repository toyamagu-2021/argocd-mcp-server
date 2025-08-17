package argocde2e

import (
	"encoding/json"
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testListRepository(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	// Call list_repository tool
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "list_repository",
			"arguments": map[string]interface{}{},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	// Verify response
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	content, ok := result["content"].([]interface{})
	if !ok {
		t.Fatalf("expected content to be an array, got %T", result["content"])
	}

	require.Len(t, content, 1, "expected exactly one content item")

	contentItem, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected content item to be a map, got %T", content[0])
	}

	contentType, ok := contentItem["type"].(string)
	if !ok {
		t.Fatalf("expected content type to be a string, got %T", contentItem["type"])
	}
	assert.Equal(t, "text", contentType, "expected content type to be 'text'")

	text, ok := contentItem["text"].(string)
	if !ok {
		t.Fatalf("expected text to be a string, got %T", contentItem["text"])
	}

	// Check if it's either "No repositories found." or valid JSON
	if text != "No repositories found." {
		// Try to parse as JSON
		var repos v1alpha1.Repositories
		err := json.Unmarshal([]byte(text), &repos)
		assert.NoError(t, err, "expected valid JSON response for repositories")
	}
}

func testGetRepository(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	// Call get_repository tool with a test repository URL
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_repository",
			"arguments": map[string]interface{}{
				"repo": "https://github.com/argoproj/argocd-example-apps.git",
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	// Verify response
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Check for error in the result
	if isError, ok := result["isError"].(bool); ok && isError {
		// This is expected if the repository doesn't exist
		t.Logf("Repository not found (expected in test environment)")
		return
	}

	content, ok := result["content"].([]interface{})
	if !ok {
		t.Fatalf("expected content to be an array, got %T", result["content"])
	}

	require.Len(t, content, 1, "expected exactly one content item")

	contentItem, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected content item to be a map, got %T", content[0])
	}

	contentType, ok := contentItem["type"].(string)
	if !ok {
		t.Fatalf("expected content type to be a string, got %T", contentItem["type"])
	}
	assert.Equal(t, "text", contentType, "expected content type to be 'text'")

	text, ok := contentItem["text"].(string)
	if !ok {
		t.Fatalf("expected text to be a string, got %T", contentItem["text"])
	}

	// Try to parse as JSON if not an error
	var repo v1alpha1.Repository
	err := json.Unmarshal([]byte(text), &repo)
	assert.NoError(t, err, "expected valid JSON response for repository")
}

// gRPC-Web version of the tests
