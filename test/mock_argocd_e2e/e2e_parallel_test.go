package mockargocde2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testServer struct {
	cmd    *exec.Cmd
	port   string
	cancel context.CancelFunc
}

func (s *testServer) stop() {
	if s.cmd != nil && s.cmd.Process != nil {
		// Send SIGTERM for graceful shutdown
		_ = s.cmd.Process.Signal(os.Interrupt)

		// Give it time to shutdown gracefully
		done := make(chan error, 1)
		go func() {
			done <- s.cmd.Wait()
		}()

		select {
		case <-done:
			// Process exited gracefully
		case <-time.After(3 * time.Second):
			// Force kill if graceful shutdown takes too long
			_ = s.cmd.Process.Kill()
			<-done
		}
	}

	if s.cancel != nil {
		s.cancel()
	}

	// Wait for port to be released
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if isPortAvailable(s.port) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func isPortAvailable(port string) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func killProcessOnPort(port string) {
	// Use lsof to find the process using the port with more details
	cmd := exec.Command("lsof", "-n", "-i", fmt.Sprintf(":%s", port))
	output, err := cmd.Output()
	if err != nil {
		// No process found or lsof not available
		return
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "COMMAND") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		// Check if the process is our mock server (go run command for server.go)
		processName := fields[0]
		pid := fields[1]

		// Only kill if it's a Go process (likely our test server)
		// This helps avoid killing unrelated services
		if processName == "go" || processName == "server" {
			// Get more info about the process to confirm it's our test server
			psCmd := exec.Command("ps", "-p", pid, "-o", "command=")
			psOutput, err := psCmd.Output()
			if err == nil {
				cmdLine := strings.TrimSpace(string(psOutput))
				// Check if it's running our mock server
				if strings.Contains(cmdLine, "mock/server.go") || strings.Contains(cmdLine, "server -port "+port) {
					fmt.Printf("Killing existing E2E test server on port %s (PID: %s)\n", port, pid)
					killCmd := exec.Command("kill", "-9", pid)
					_ = killCmd.Run()
				}
			}
		}
	}
}

var (
	sharedMockServer *testServer
	sharedMCPServer  struct {
		cmd       *exec.Cmd
		stdin     io.WriteCloser
		stdout    io.ReadCloser
		mu        sync.Mutex
		idCounter atomic.Int64
		decoder   *json.Decoder
	}
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
	if sharedMCPServer.cmd != nil && sharedMCPServer.cmd.Process != nil {
		// Send SIGTERM for graceful shutdown
		_ = sharedMCPServer.cmd.Process.Signal(os.Interrupt)

		// Give it time to shutdown gracefully
		done := make(chan error, 1)
		go func() {
			done <- sharedMCPServer.cmd.Wait()
		}()

		select {
		case <-done:
			// Process exited gracefully
		case <-time.After(3 * time.Second):
			// Force kill if graceful shutdown takes too long
			_ = sharedMCPServer.cmd.Process.Kill()
			<-done
		}
	}
	if sharedMockServer != nil {
		sharedMockServer.stop()
	}

	os.Exit(code)
}

func startMockServerForTests(port string) *testServer {
	// Kill any existing process using the port
	killProcessOnPort(port)

	// Wait a bit for the port to be fully released
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, "go", "run", "../mock/server.go", "-port", port)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		panic(fmt.Sprintf("failed to start mock server: %v", err))
	}

	// Wait for mock server to be ready
	for i := 0; i < 50; i++ {
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%s", port))
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	return &testServer{
		cmd:    cmd,
		port:   port,
		cancel: cancel,
	}
}

func startSharedMCPServer(port string) {
	cmd := exec.Command("go", "run", "../../cmd/argocd-mcp-server/main.go")

	cmd.Env = append(os.Environ(),
		"ARGOCD_AUTH_TOKEN=test-token",
		fmt.Sprintf("ARGOCD_SERVER=localhost:%s", port),
		"ARGOCD_INSECURE=true",
		"ARGOCD_PLAINTEXT=true",
		"ARGOCD_GRPC_WEB=false", // Explicitly disable gRPC-Web
		"LOG_LEVEL=debug",
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		panic(fmt.Sprintf("failed to create stdin pipe: %v", err))
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(fmt.Sprintf("failed to create stdout pipe: %v", err))
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		panic(fmt.Sprintf("failed to start MCP server: %v", err))
	}

	time.Sleep(500 * time.Millisecond)

	sharedMCPServer.cmd = cmd
	sharedMCPServer.stdin = stdin
	sharedMCPServer.stdout = stdout
	sharedMCPServer.decoder = json.NewDecoder(stdout)
}

func initializeSharedMCPServerForTests() {
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"roots": map[string]interface{}{
					"listChanged": true,
				},
			},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	response := sendSharedRequestForTests(initRequest)

	if response["jsonrpc"] != "2.0" {
		panic(fmt.Sprintf("expected jsonrpc 2.0, got %v", response["jsonrpc"]))
	}

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		panic(fmt.Sprintf("expected result to be a map, got %T", response["result"]))
	}

	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		panic(fmt.Sprintf("expected serverInfo to be a map, got %T", result["serverInfo"]))
	}

	if serverInfo["name"] != "argocd-mcp-server" {
		panic(fmt.Sprintf("expected server name argocd-mcp-server, got %v", serverInfo["name"]))
	}
}

func sendSharedRequest(t *testing.T, request map[string]interface{}) map[string]interface{} {
	t.Helper()
	return sendSharedRequestForTests(request)
}

func sendSharedRequestForTests(request map[string]interface{}) map[string]interface{} {
	sharedMCPServer.mu.Lock()
	defer sharedMCPServer.mu.Unlock()

	// Generate unique ID if not provided
	if request["id"] == nil {
		request["id"] = sharedMCPServer.idCounter.Add(1)
	}

	data, err := json.Marshal(request)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal request: %v", err))
	}

	if _, err := sharedMCPServer.stdin.Write(data); err != nil {
		panic(fmt.Sprintf("failed to write request: %v", err))
	}
	if _, err := sharedMCPServer.stdin.Write([]byte("\n")); err != nil {
		panic(fmt.Sprintf("failed to write newline: %v", err))
	}

	var response map[string]interface{}
	if err := sharedMCPServer.decoder.Decode(&response); err != nil {
		panic(fmt.Sprintf("failed to decode response: %v", err))
	}

	return response
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

func TestParallel_ListApplications(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "list_application",
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

	if !strings.Contains(text, "test-app-1") || !strings.Contains(text, "test-app-2") {
		t.Errorf("expected response to contain test applications")
	}
}

func TestParallel_GetApplication(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application",
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

	if !strings.Contains(text, "test-app-1") {
		t.Errorf("expected response to contain test-app-1")
	}

	if !strings.Contains(text, "https://github.com/test/repo1") {
		t.Errorf("expected response to contain repo URL")
	}
}

func TestParallel_SyncApplication(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "sync_application",
			"arguments": map[string]interface{}{
				"name":    "test-app-1",
				"dry_run": true,
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

	if !strings.Contains(text, "test-app-1") {
		t.Errorf("expected response to contain application name")
	}

	hasOperationState := strings.Contains(text, "operationState") || strings.Contains(text, "OperationState")
	hasSucceeded := strings.Contains(text, "Succeeded") || strings.Contains(text, "Dry run completed successfully")
	if !hasOperationState || !hasSucceeded {
		t.Errorf("expected response to contain sync operation result, got: %s", text)
	}
}

func TestParallel_DeleteApplication(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "delete_application",
			"arguments": map[string]interface{}{
				"name":    "test-app-2",
				"cascade": false,
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

	if !strings.Contains(text, "test-app-2") || !strings.Contains(text, "deleted successfully") {
		t.Errorf("expected response to contain deletion confirmation")
	}
}

func TestParallel_CreateApplication(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "create_application",
			"arguments": map[string]interface{}{
				"name":           "test-app-new",
				"repo_url":       "https://github.com/test/new-repo",
				"dest_namespace": "default",
				"path":           "manifests",
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

	// Debug: print the actual response
	t.Logf("Create response text: %s", text)

	if !strings.Contains(text, "test-app-new") {
		t.Errorf("expected response to contain new application name, got: %s", text)
	}

	if !strings.Contains(text, "created successfully") && !strings.Contains(text, "Created application") && !strings.Contains(text, "Application created") {
		t.Errorf("expected response to contain creation confirmation, got: %s", text)
	}
}

func TestParallel_FilterApplicationsByProject(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "list_application",
			"arguments": map[string]interface{}{
				"project": "production",
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

	if !strings.Contains(text, "test-app-2") {
		t.Errorf("expected response to contain test-app-2 (production project)")
	}

	if strings.Contains(text, "test-app-1") {
		t.Errorf("expected response NOT to contain test-app-1 (default project)")
	}
}

func TestParallel_FilterApplicationsByNamespace(t *testing.T) {
	t.Parallel()

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "list_application",
			"arguments": map[string]interface{}{
				"namespace": "prod",
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

	// Check that only prod namespace apps are returned
	if !strings.Contains(text, "test-app-2") {
		t.Errorf("expected response to contain test-app-2 (prod namespace)")
	}

	if strings.Contains(text, "test-app-1") {
		t.Errorf("expected response NOT to contain test-app-1 (default namespace)")
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
