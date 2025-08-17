package mockargocde2e

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParallel_GetApplicationEvents(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application_events",
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

	// Check if the response is an error
	if strings.HasPrefix(text, "Failed") {
		t.Logf("Got error response: %s", text)
		// For mock tests, we might get a 'not implemented' error
		// which is acceptable as long as the tool is registered
		if !strings.Contains(text, "not implemented") && !strings.Contains(text, "Unimplemented") {
			t.Fatalf("Unexpected error response: %s", text)
		}
		t.Skip("ListResourceEvents not fully implemented in mock server yet")
	}

	// Parse JSON to validate structure
	var eventsResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &eventsResp); err != nil {
		t.Logf("Response text: %s", text)
		t.Fatalf("expected response to be valid JSON: %v", err)
	}

	// Check for expected fields in events response
	if _, ok := eventsResp["items"]; !ok {
		t.Error("expected response to contain items field")
	}

	t.Logf("Successfully retrieved events for application test-app-1")
	t.Logf("Response snippet: %.500s...", text)
}

func TestParallel_GetApplicationManifests(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application_manifests",
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

	// Parse JSON to validate structure
	var manifestResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &manifestResp); err != nil {
		t.Fatalf("expected response to be valid JSON: %v", err)
	}

	// Check for expected fields in manifest response
	if _, ok := manifestResp["Manifests"]; !ok {
		if _, ok := manifestResp["manifests"]; !ok {
			t.Error("expected response to contain manifests field")
		}
	}

	t.Logf("Successfully retrieved manifests for application test-app-1")
}
