package mockargocde2e

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/tools"
)

func TestParallel_GetUserInfo(t *testing.T) {
	t.Parallel()

	// Call get_user_info tool
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "get_user_info",
			"arguments": map[string]interface{}{},
		},
	}

	response := sendSharedRequest(t, callToolRequest)

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

	// Verify the mock response
	assert.True(t, userInfo.LoggedIn, "expected user to be logged in")
	assert.Equal(t, "test-user", userInfo.Username, "expected username to be 'test-user'")
	assert.Equal(t, "argocd", userInfo.Issuer, "expected issuer to be 'argocd'")
	assert.Equal(t, []string{"admin", "developers"}, userInfo.Groups, "expected groups to match")
}
