package mockargocde2e

import (
	"encoding/json"
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParallel_ListRepository(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "list_repository",
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
	var repos v1alpha1.Repositories
	err := json.Unmarshal([]byte(text), &repos)
	require.NoError(t, err, "expected valid JSON response for repositories")

	// Check that we got the mock repositories
	assert.Len(t, repos, 2, "expected 2 mock repositories")
	if len(repos) == 2 {
		assert.Equal(t, "https://github.com/example/repo1.git", repos[0].Repo)
		assert.Equal(t, "git", repos[0].Type)
		assert.Equal(t, "https://github.com/example/repo2.git", repos[1].Repo)
		assert.Equal(t, "git", repos[1].Type)
	}
}

func TestParallel_GetRepository(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_repository",
			"arguments": map[string]interface{}{
				"repo": "https://github.com/example/repo1.git",
			},
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
	var repo v1alpha1.Repository
	err := json.Unmarshal([]byte(text), &repo)
	require.NoError(t, err, "expected valid JSON response for repository")

	// Check that we got the correct mock repository
	assert.Equal(t, "https://github.com/example/repo1.git", repo.Repo)
	assert.Equal(t, "git", repo.Type)
	assert.Equal(t, "example-user", repo.Username)
}

func TestParallel_GetRepository_NotFound(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_repository",
			"arguments": map[string]interface{}{
				"repo": "https://github.com/nonexistent/repo.git",
			},
		},
	}

	response := sendSharedRequest(t, callToolRequest)

	// Verify response
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Check for error in the result
	isError, ok := result["isError"].(bool)
	require.True(t, ok, "expected isError field to be present")
	assert.True(t, isError, "expected error for non-existent repository")
}
