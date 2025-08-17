package argocde2e

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/tools"
)

func testGetUserInfo(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	// Call get_user_info tool
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "get_user_info",
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

	// Parse the JSON response
	var userInfo tools.UserInfo
	err := json.Unmarshal([]byte(text), &userInfo)
	require.NoError(t, err, "expected valid JSON response for user info")

	// Verify the response contains expected fields
	// In a real ArgoCD environment, we expect to be logged in
	assert.NotEmpty(t, userInfo.Username, "expected username to be present")
	assert.True(t, userInfo.LoggedIn, "expected user to be logged in")
}
