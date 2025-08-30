package mockargocde2e

import (
	"strings"
	"testing"
)

func TestParallel_TerminateOperation(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "terminate_operation",
			"arguments": map[string]interface{}{
				"name": "test-app-1",
			},
		},
	}

	response := sendSharedRequest(t, callToolRequest)

	// Check if there's an error - it's OK if there's no operation to terminate
	if errObj, ok := response["error"].(map[string]interface{}); ok {
		errorMsg, ok := errObj["message"].(string)
		if !ok {
			t.Fatalf("expected error message to be a string, got %T", errObj["message"])
		}

		// It's OK if there's no operation to terminate
		if strings.Contains(errorMsg, "no operation is in progress") ||
			strings.Contains(errorMsg, "operation has already completed") {
			t.Logf("No operation in progress for test-app-1 (expected)")
			return
		}

		// Any other error is unexpected
		t.Fatalf("Unexpected error calling terminate_operation: %v", errorMsg)
	}

	// If no error, verify the success response
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

	// Check that the response mentions successful termination
	if !strings.Contains(text, "Successfully terminated operation") {
		t.Errorf("expected response to indicate successful termination, got: %s", text)
	}

	if !strings.Contains(text, "test-app-1") {
		t.Errorf("expected response to contain application name test-app-1")
	}
}

func TestParallel_TerminateOperationWithNamespace(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "terminate_operation",
			"arguments": map[string]interface{}{
				"name":          "test-app-2",
				"app_namespace": "argocd",
			},
		},
	}

	response := sendSharedRequest(t, callToolRequest)

	// Check if there's an error - it's OK if there's no operation to terminate
	if errObj, ok := response["error"].(map[string]interface{}); ok {
		errorMsg, ok := errObj["message"].(string)
		if !ok {
			t.Fatalf("expected error message to be a string, got %T", errObj["message"])
		}

		// It's OK if there's no operation to terminate
		if strings.Contains(errorMsg, "no operation is in progress") ||
			strings.Contains(errorMsg, "operation has already completed") {
			t.Logf("No operation in progress for test-app-2 (expected)")
			return
		}

		// Any other error is unexpected
		t.Fatalf("Unexpected error calling terminate_operation: %v", errorMsg)
	}

	// If no error, verify the success response
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

	// Check that the response mentions successful termination with namespace
	if !strings.Contains(text, "Successfully terminated operation") {
		t.Errorf("expected response to indicate successful termination, got: %s", text)
	}

	if !strings.Contains(text, "test-app-2") {
		t.Errorf("expected response to contain application name test-app-2")
	}

	if !strings.Contains(text, "argocd") {
		t.Errorf("expected response to contain namespace argocd")
	}
}

func TestParallel_TerminateOperationNotFound(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "terminate_operation",
			"arguments": map[string]interface{}{
				"name": "non-existent-app",
			},
		},
	}

	response := sendSharedRequest(t, callToolRequest)

	// Check if there's an error in the error field first
	if errObj, ok := response["error"].(map[string]interface{}); ok {
		errorMsg, _ := errObj["message"].(string)
		if strings.Contains(errorMsg, "not found") || strings.Contains(errorMsg, "NotFound") {
			return // Expected error
		}
		t.Fatalf("unexpected error: %v", errorMsg)
	}

	// Check if there's an error in the result field (wrapped as isError=true)
	if result, ok := response["result"].(map[string]interface{}); ok {
		if isError, _ := result["isError"].(bool); isError {
			// Get the error message from content
			if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
				if textContent, ok := content[0].(map[string]interface{}); ok {
					if text, ok := textContent["text"].(string); ok {
						if strings.Contains(text, "not found") || strings.Contains(text, "NotFound") {
							return // Expected error
						}
						t.Errorf("expected 'not found' error, got: %s", text)
					}
				}
			}
		}
	}

	t.Fatalf("expected error response for non-existent application, got: %v", response)
}
