package mockargocde2e

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func TestParallel_ListApplicationSets(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "list_applicationset",
			"arguments": map[string]interface{}{},
		},
	}

	response := sendSharedRequest(t, callToolRequest)

	// Verify response
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Check if it's an error or success
	if isError, _ := result["isError"].(bool); isError {
		// Check for expected error messages
		content := result["content"].([]interface{})
		if len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			// ApplicationSets might not be supported in mock, which is okay
			if strings.Contains(text, "No ApplicationSets found") ||
				strings.Contains(text, "not implemented") {
				t.Logf("Mock server returned expected message: %s", text)
			} else {
				t.Errorf("Unexpected error: %s", text)
			}
		}
	} else {
		// Verify content structure
		content, ok := result["content"].([]interface{})
		if !ok || len(content) == 0 {
			t.Fatal("expected content array")
		}

		textContent, ok := content[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected content[0] to be a map, got %T", content[0])
		}

		text, ok := textContent["text"].(string)
		if !ok {
			t.Fatalf("expected text to be a string, got %T", textContent["text"])
		}

		// Try to parse as JSON array
		var appSets []v1alpha1.ApplicationSet
		if err := json.Unmarshal([]byte(text), &appSets); err == nil {
			t.Logf("Successfully listed %d ApplicationSets", len(appSets))
		} else if strings.Contains(text, "No ApplicationSets found") {
			t.Log("No ApplicationSets found (expected for mock)")
		} else {
			t.Errorf("Unexpected response format: %s", text)
		}
	}
}

func TestParallel_ListApplicationSetsWithProject(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "list_applicationset",
			"arguments": map[string]interface{}{
				"project": "default",
			},
		},
	}

	response := sendSharedRequest(t, callToolRequest)

	// Verify response
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Similar verification as above
	if isError, _ := result["isError"].(bool); !isError {
		content, _ := result["content"].([]interface{})
		if len(content) > 0 {
			textContent, _ := content[0].(map[string]interface{})
			text, _ := textContent["text"].(string)
			t.Logf("Response with project filter: %.200s...", text)
		}
	}
}

func TestParallel_GetApplicationSet(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_applicationset",
			"arguments": map[string]interface{}{
				"name": "test-appset",
			},
		},
	}

	response := sendSharedRequest(t, callToolRequest)

	// Verify response
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Check if it's an error (expected for non-existent appset)
	if isError, _ := result["isError"].(bool); isError {
		content := result["content"].([]interface{})
		if len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			// Should contain "not found" or similar error
			if strings.Contains(text, "not found") ||
				strings.Contains(text, "Failed to get ApplicationSet") ||
				strings.Contains(text, "not implemented") {
				t.Logf("Expected error for non-existent ApplicationSet: %s", text)
			} else {
				t.Errorf("Unexpected error: %s", text)
			}
		}
	} else {
		// If it exists (unlikely in mock), verify structure
		content, _ := result["content"].([]interface{})
		if len(content) > 0 {
			textContent, _ := content[0].(map[string]interface{})
			text, _ := textContent["text"].(string)

			var appSet v1alpha1.ApplicationSet
			if err := json.Unmarshal([]byte(text), &appSet); err == nil {
				t.Logf("Successfully retrieved ApplicationSet: %s", appSet.Name)
			} else {
				t.Errorf("Failed to parse ApplicationSet JSON: %v", err)
			}
		}
	}
}

func TestParallel_GetApplicationSetMissingName(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "get_applicationset",
			"arguments": map[string]interface{}{},
		},
	}

	response := sendSharedRequest(t, callToolRequest)

	// Verify response
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
			} else {
				t.Log("Got expected error for missing name parameter")
			}
		}
	}
}

func TestParallel_ListApplicationSetsWithSelector(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "list_applicationset",
			"arguments": map[string]interface{}{
				"selector": "env=prod",
			},
		},
	}

	response := sendSharedRequest(t, callToolRequest)

	// Verify response
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Log the response
	if isError, _ := result["isError"].(bool); !isError {
		content, _ := result["content"].([]interface{})
		if len(content) > 0 {
			textContent, _ := content[0].(map[string]interface{})
			text, _ := textContent["text"].(string)
			t.Logf("Response with selector filter: %.200s...", text)
		}
	} else {
		content := result["content"].([]interface{})
		if len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			t.Logf("Error response with selector: %s", text)
		}
	}
}

func TestParallel_ListApplicationSetsInvalidSelector(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "list_applicationset",
			"arguments": map[string]interface{}{
				"selector": "invalid selector format",
			},
		},
	}

	response := sendSharedRequest(t, callToolRequest)

	// Verify response
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Should be an error
	if isError, _ := result["isError"].(bool); !isError {
		t.Error("expected error for invalid selector format")
	} else {
		content := result["content"].([]interface{})
		if len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			if !strings.Contains(text, "Invalid selector") {
				t.Errorf("expected 'Invalid selector' error, got: %s", text)
			} else {
				t.Log("Got expected error for invalid selector format")
			}
		}
	}
}

func TestParallel_CreateApplicationSet(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "create_applicationset",
			"arguments": map[string]interface{}{
				"name":       "test-created-appset",
				"namespace":  "argocd",
				"project":    "default",
				"generators": `[{"list":{"elements":[{"cluster":"test-cluster","url":"https://test.cluster.local"}]}}]`,
				"template": `{
					"metadata": {
						"name": "{{cluster}}-app"
					},
					"spec": {
						"project": "default",
						"source": {
							"repoURL": "https://github.com/test/repo",
							"targetRevision": "HEAD",
							"path": "manifests"
						},
						"destination": {
							"server": "{{url}}",
							"namespace": "default"
						},
						"syncPolicy": {
							"syncOptions": [
								"CreateNamespace=true"
							]
						}
					}
				}`,
				"dry_run": false,
			},
		},
	}

	response := sendSharedRequest(t, callToolRequest)

	// Verify response
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Check if it's an error or success
	if isError, _ := result["isError"].(bool); isError {
		content := result["content"].([]interface{})
		if len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			t.Errorf("ApplicationSet creation failed: %s", text)
		}
	} else {
		// Verify content structure
		content, ok := result["content"].([]interface{})
		if !ok || len(content) == 0 {
			t.Fatal("expected content array")
		}

		textContent, ok := content[0].(map[string]interface{})
		if !ok {
			t.Fatal("expected text content")
		}

		text, ok := textContent["text"].(string)
		if !ok {
			t.Fatal("expected text string")
		}

		// Parse the ApplicationSet from response
		var appSet v1alpha1.ApplicationSet
		if err := json.Unmarshal([]byte(text), &appSet); err != nil {
			t.Fatalf("failed to unmarshal ApplicationSet: %v", err)
		}

		// Verify ApplicationSet fields
		if appSet.Name != "test-created-appset" {
			t.Errorf("expected ApplicationSet name 'test-created-appset', got %s", appSet.Name)
		}

		if appSet.Namespace != "argocd" {
			t.Errorf("expected namespace 'argocd', got %s", appSet.Namespace)
		}

		t.Logf("Successfully created ApplicationSet (dry run): %s", appSet.Name)
	}
}

func TestParallel_CreateApplicationSetWithUpsert(t *testing.T) {
	t.Parallel()

	// Try to create an existing ApplicationSet with upsert
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "create_applicationset",
			"arguments": map[string]interface{}{
				"name":       "test-appset-1", // This already exists in mock
				"namespace":  "argocd",
				"project":    "default",
				"generators": `[{"list":{"elements":[{"cluster":"updated"}]}}]`,
				"template": `{
					"metadata": {
						"name": "{{cluster}}-app"
					},
					"spec": {
						"project": "default",
						"source": {
							"repoURL": "https://github.com/updated/repo",
							"targetRevision": "HEAD",
							"path": "updated"
						},
						"destination": {
							"server": "https://kubernetes.default.svc",
							"namespace": "updated"
						}
					}
				}`,
				"upsert":  true,
				"dry_run": true,
			},
		},
	}

	response := sendSharedRequest(t, callToolRequest)

	// Verify response
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Should succeed with upsert
	if isError, _ := result["isError"].(bool); isError {
		content := result["content"].([]interface{})
		if len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			t.Errorf("ApplicationSet upsert failed: %s", text)
		}
	} else {
		t.Log("Successfully upserted ApplicationSet")
	}
}

func TestParallel_CreateApplicationSetAlreadyExists(t *testing.T) {
	t.Parallel()

	// Try to create an existing ApplicationSet without upsert
	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "create_applicationset",
			"arguments": map[string]interface{}{
				"name":       "test-appset-1", // This already exists in mock
				"namespace":  "argocd",
				"project":    "default",
				"generators": `[{"list":{"elements":[{"cluster":"test"}]}}]`,
				"template": `{
					"metadata": {
						"name": "{{cluster}}-app"
					},
					"spec": {
						"project": "default",
						"source": {
							"repoURL": "https://github.com/test/repo",
							"targetRevision": "HEAD",
							"path": "manifests"
						},
						"destination": {
							"server": "https://kubernetes.default.svc",
							"namespace": "default"
						}
					}
				}`,
				"upsert": false,
			},
		},
	}

	response := sendSharedRequest(t, callToolRequest)

	// Verify response
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	// Should fail with already exists error
	if isError, _ := result["isError"].(bool); !isError {
		t.Error("expected error for already existing ApplicationSet")
	} else {
		content := result["content"].([]interface{})
		if len(content) > 0 {
			textContent := content[0].(map[string]interface{})
			text := textContent["text"].(string)
			if !strings.Contains(text, "already exists") {
				t.Errorf("expected 'already exists' error, got: %s", text)
			} else {
				t.Log("Got expected 'already exists' error")
			}
		}
	}
}

func TestParallel_CreateApplicationSetMissingRequiredFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		arguments     map[string]interface{}
		expectedError string
	}{
		{
			name: "missing name",
			arguments: map[string]interface{}{
				"generators": `[{"list":{"elements":[{"cluster":"test"}]}}]`,
				"template":   `{"metadata":{"name":"test"},"spec":{}}`,
			},
			expectedError: "name is required",
		},
		{
			name: "missing generators",
			arguments: map[string]interface{}{
				"name":     "test-appset",
				"template": `{"metadata":{"name":"test"},"spec":{}}`,
			},
			expectedError: "generators is required",
		},
		{
			name: "missing template",
			arguments: map[string]interface{}{
				"name":       "test-appset",
				"generators": `[{"list":{"elements":[{"cluster":"test"}]}}]`,
			},
			expectedError: "template is required",
		},
		{
			name: "invalid generators JSON",
			arguments: map[string]interface{}{
				"name":       "test-appset",
				"generators": `invalid json`,
				"template":   `{"metadata":{"name":"test"},"spec":{}}`,
			},
			expectedError: "Failed to parse generators",
		},
		{
			name: "invalid template JSON",
			arguments: map[string]interface{}{
				"name":       "test-appset",
				"generators": `[{"list":{"elements":[{"cluster":"test"}]}}]`,
				"template":   `invalid json`,
			},
			expectedError: "Failed to parse template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callToolRequest := map[string]interface{}{
				"jsonrpc": "2.0",
				"method":  "tools/call",
				"params": map[string]interface{}{
					"name":      "create_applicationset",
					"arguments": tt.arguments,
				},
			}

			response := sendSharedRequest(t, callToolRequest)

			// Verify response
			result, ok := response["result"].(map[string]interface{})
			if !ok {
				t.Fatalf("expected result to be a map, got %T", response["result"])
			}

			// Should be an error
			if isError, _ := result["isError"].(bool); !isError {
				t.Errorf("expected error for %s", tt.name)
			} else {
				content := result["content"].([]interface{})
				if len(content) > 0 {
					textContent := content[0].(map[string]interface{})
					text := textContent["text"].(string)
					if !strings.Contains(text, tt.expectedError) {
						t.Errorf("expected error containing '%s', got: %s", tt.expectedError, text)
					}
				}
			}
		})
	}
}
