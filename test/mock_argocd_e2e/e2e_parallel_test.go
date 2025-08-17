package mockargocde2e

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	// TODO: Skip mock_argocd_e2e tests in CI/CD environments
	if os.Getenv("GITHUB_RUN_ID") != "" {
		fmt.Println("Skipping mock_argocd_e2e tests in CI/CD environment")
		os.Exit(0)
	}

	// Setup shared servers once for all tests
	mockServer := startMockServerForTests("60200")
	sharedMockServer = mockServer

	// Start MCP server
	startSharedMCPServer("60200")

	// Initialize MCP server
	initializeSharedMCPServerForTests()

	// Run tests
	code := m.Run()

	// Cleanup
	// Close stdin/stdout pipes first to signal MCP server to shutdown
	if sharedMCPServer.stdin != nil {
		_ = sharedMCPServer.stdin.Close()
	}
	if sharedMCPServer.stdout != nil {
		_ = sharedMCPServer.stdout.Close()
	}

	if sharedMCPServer.cmd != nil && sharedMCPServer.cmd.Process != nil {
		// Kill the entire process group to ensure all child processes are terminated
		pgid, _ := syscall.Getpgid(sharedMCPServer.cmd.Process.Pid)
		_ = syscall.Kill(-pgid, syscall.SIGTERM)

		// Give it time to shutdown gracefully
		done := make(chan error, 1)
		go func() {
			done <- sharedMCPServer.cmd.Wait()
		}()

		select {
		case <-done:
			// Process exited gracefully
		case <-time.After(3 * time.Second):
			// Force kill the entire process group if graceful shutdown takes too long
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
			<-done
		}
	}
	if sharedMockServer != nil {
		sharedMockServer.stop()
	}

	os.Exit(code)
}

func TestParallel_Initialize(t *testing.T) {
	t.Parallel()

	// The server is already initialized in setupSharedServers,
	// so we just verify it responds to a simple request
	listToolsRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/list",
	}

	response := sendSharedRequest(t, listToolsRequest)

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("expected tools to be an array, got %T", result["tools"])
	}

	if len(tools) == 0 {
		t.Errorf("expected at least one tool")
	}
}

func TestParallel_ListTools(t *testing.T) {
	t.Parallel()

	listToolsRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/list",
	}

	response := sendSharedRequest(t, listToolsRequest)

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("expected tools to be an array, got %T", result["tools"])
	}

	expectedTools := []string{"list_application", "get_application", "sync_application", "delete_application", "create_application"}
	toolNames := make([]string, 0)

	for _, tool := range tools {
		toolMap, ok := tool.(map[string]interface{})
		if !ok {
			continue
		}
		name, ok := toolMap["name"].(string)
		if ok {
			toolNames = append(toolNames, name)
		}
	}

	for _, expected := range expectedTools {
		found := false
		for _, name := range toolNames {
			if name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected tool %s not found", expected)
		}
	}
}

func TestParallel_ConcurrentRequests(t *testing.T) {
	t.Parallel()

	// Run multiple requests concurrently
	var wg sync.WaitGroup
	numRequests := 10
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// Alternate between different types of requests
			var request map[string]interface{}
			if index%2 == 0 {
				request = map[string]interface{}{
					"jsonrpc": "2.0",
					"method":  "tools/call",
					"params": map[string]interface{}{
						"name":      "list_application",
						"arguments": map[string]interface{}{},
					},
				}
			} else {
				request = map[string]interface{}{
					"jsonrpc": "2.0",
					"method":  "tools/call",
					"params": map[string]interface{}{
						"name": "get_application",
						"arguments": map[string]interface{}{
							"name": fmt.Sprintf("test-app-%d", (index%2)+1),
						},
					},
				}
			}

			response := sendSharedRequest(t, request)

			if response["error"] != nil {
				errors <- fmt.Errorf("request %d failed: %v", index, response["error"])
				return
			}

			result, ok := response["result"].(map[string]interface{})
			if !ok {
				errors <- fmt.Errorf("request %d: expected result to be a map", index)
				return
			}

			content, ok := result["content"].([]interface{})
			if !ok || len(content) == 0 {
				errors <- fmt.Errorf("request %d: expected content array", index)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}
}
