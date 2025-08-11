package argocde2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	// Global test state for sequential execution
	testAppCreated = false
	testMutex      sync.Mutex
)

func getEnvOrSkip(t *testing.T, key string) string {
	value := os.Getenv(key)
	if value == "" {
		t.Skipf("Skipping test: %s environment variable not set", key)
	}
	return value
}

func startMCPServer(t *testing.T) (*exec.Cmd, io.WriteCloser, io.ReadCloser) {
	return startMCPServerWithOptions(t, false)
}

func startMCPServerWithGRPCWeb(t *testing.T) (*exec.Cmd, io.WriteCloser, io.ReadCloser) {
	return startMCPServerWithOptions(t, true)
}

func startMCPServerWithOptions(t *testing.T, useGRPCWeb bool) (*exec.Cmd, io.WriteCloser, io.ReadCloser) {
	server := getEnvOrSkip(t, "ARGOCD_SERVER")
	token := getEnvOrSkip(t, "ARGOCD_AUTH_TOKEN")

	cmd := exec.Command("go", "run", "../../cmd/argocd-mcp-server/main.go")

	env := os.Environ()
	env = append(env,
		fmt.Sprintf("ARGOCD_SERVER=%s", server),
		fmt.Sprintf("ARGOCD_AUTH_TOKEN=%s", token),
	)

	if insecure := os.Getenv("ARGOCD_INSECURE"); insecure != "" {
		env = append(env, fmt.Sprintf("ARGOCD_INSECURE=%s", insecure))
	}

	if plaintext := os.Getenv("ARGOCD_PLAINTEXT"); plaintext != "" {
		env = append(env, fmt.Sprintf("ARGOCD_PLAINTEXT=%s", plaintext))
	}

	// Force gRPC-Web mode if requested
	if useGRPCWeb {
		env = append(env, "ARGOCD_GRPC_WEB=true")
		if grpcWebRootPath := os.Getenv("ARGOCD_GRPC_WEB_ROOT_PATH"); grpcWebRootPath != "" {
			env = append(env, fmt.Sprintf("ARGOCD_GRPC_WEB_ROOT_PATH=%s", grpcWebRootPath))
		}
	} else {
		// Explicitly disable gRPC-Web for normal tests
		env = append(env, "ARGOCD_GRPC_WEB=false")
	}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		env = append(env, fmt.Sprintf("LOG_LEVEL=%s", logLevel))
	} else {
		env = append(env, "LOG_LEVEL=info")
	}

	cmd.Env = env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("failed to create stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start MCP server: %v", err)
	}

	time.Sleep(1 * time.Second)

	return cmd, stdin, stdout
}

func sendRequest(t *testing.T, stdin io.WriteCloser, stdout io.ReadCloser, request interface{}) map[string]interface{} {
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	if _, err := stdin.Write(data); err != nil {
		t.Fatalf("failed to write request: %v", err)
	}
	if _, err := stdin.Write([]byte("\n")); err != nil {
		t.Fatalf("failed to write newline: %v", err)
	}

	decoder := json.NewDecoder(stdout)
	var response map[string]interface{}
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	return response
}

func initializeMCPConnection(t *testing.T, stdin io.WriteCloser, stdout io.ReadCloser) {
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
				"name":    "argocd-e2e-test",
				"version": "1.0.0",
			},
		},
	}

	response := sendRequest(t, stdin, stdout, initRequest)

	if response["jsonrpc"] != "2.0" {
		t.Fatalf("expected jsonrpc 2.0, got %v", response["jsonrpc"])
	}

	if _, ok := response["result"].(map[string]interface{}); !ok {
		t.Fatalf("initialization failed: %v", response)
	}
}

// TestRealArgoCD_Suite runs all E2E tests in a controlled sequential order
func TestRealArgoCD_Suite(t *testing.T) {
	// Skip if running in parallel mode
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	// Test with normal gRPC connection
	t.Run("gRPC", func(t *testing.T) {
		// Basic connectivity tests (can run in any order)
		t.Run("Initialize", testInitialize)
		t.Run("ListTools", testListTools)
		t.Run("ListApplications", testListApplications)
		t.Run("InvalidAppName", testInvalidAppName)
		t.Run("WithTimeout", testWithTimeout)

		// Project tests
		t.Run("ListProjects", testListProjects)
		t.Run("GetProject", testGetProject)
		t.Run("ListClusters", testListClusters)
		t.Run("GetCluster", testGetCluster)
		t.Run("GetClusterNotFound", testGetClusterNotFound)
		t.Run("InvalidProjectName", testInvalidProjectName)
		t.Run("CreateProject", testCreateProject)

		// Application lifecycle tests (must run in order)
		t.Run("ApplicationLifecycle", func(t *testing.T) {
			// These subtests will run sequentially in order
			t.Run("01_CreateApplication", testCreateApplication)
			t.Run("02_GetCreatedApplication", testGetCreatedApplication)
			t.Run("03_GetCreatedApplicationEvents", testGetCreatedApplicationEvents)
			t.Run("04_GetCreatedApplicationManifests", testGetCreatedApplicationManifests)
			t.Run("05_ListApplicationsWithCreated", testListApplicationsWithCreated)
			t.Run("06_SyncCreatedApplication", testSyncCreatedApplication)
			t.Run("07_DeleteCreatedApplication", testDeleteCreatedApplication)
		})

		// Tests that require existing application
		if appName := os.Getenv("TEST_APP_NAME"); appName != "" {
			t.Run("GetExistingApplication", testGetExistingApplication)
			t.Run("GetExistingApplicationEvents", testGetExistingApplicationEvents)
			t.Run("GetExistingApplicationManifests", testGetExistingApplicationManifests)
			t.Run("SyncExistingApplication_DryRun", testSyncExistingApplicationDryRun)
		}
	})

	// Test with gRPC-Web connection
	t.Run("gRPC-Web", func(t *testing.T) {
		// Basic connectivity tests (can run in any order)
		t.Run("Initialize", testInitializeGRPCWeb)
		t.Run("ListTools", testListToolsGRPCWeb)
		t.Run("ListApplications", testListApplicationsGRPCWeb)
		t.Run("InvalidAppName", testInvalidAppNameGRPCWeb)
		t.Run("WithTimeout", testWithTimeoutGRPCWeb)

		// Project tests
		t.Run("ListProjects", testListProjectsGRPCWeb)
		t.Run("GetProject", testGetProjectGRPCWeb)
		t.Run("ListClusters", testListClustersGRPCWeb)
		t.Run("GetCluster", testGetClusterGRPCWeb)
		t.Run("GetClusterNotFound", testGetClusterNotFoundGRPCWeb)
		t.Run("InvalidProjectName", testInvalidProjectNameGRPCWeb)
		t.Run("CreateProject", testCreateProjectGRPCWeb)

		// Application lifecycle tests (must run in order)
		t.Run("ApplicationLifecycle", func(t *testing.T) {
			// These subtests will run sequentially in order
			t.Run("01_CreateApplication", testCreateApplicationGRPCWeb)
			t.Run("02_GetCreatedApplication", testGetCreatedApplicationGRPCWeb)
			t.Run("03_GetCreatedApplicationEvents", testGetCreatedApplicationEventsGRPCWeb)
			t.Run("04_GetCreatedApplicationManifests", testGetCreatedApplicationManifestsGRPCWeb)
			t.Run("05_ListApplicationsWithCreated", testListApplicationsWithCreatedGRPCWeb)
			t.Run("06_SyncCreatedApplication", testSyncCreatedApplicationGRPCWeb)
			t.Run("07_DeleteCreatedApplication", testDeleteCreatedApplicationGRPCWeb)
		})

		// Tests that require existing application
		if appName := os.Getenv("TEST_APP_NAME"); appName != "" {
			t.Run("GetExistingApplication", testGetExistingApplicationGRPCWeb)
			t.Run("GetExistingApplicationEvents", testGetExistingApplicationEventsGRPCWeb)
			t.Run("GetExistingApplicationManifests", testGetExistingApplicationManifestsGRPCWeb)
			t.Run("SyncExistingApplication_DryRun", testSyncExistingApplicationDryRunGRPCWeb)
		}
	})
}

func testInitialize(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

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
				"name":    "argocd-e2e-test",
				"version": "1.0.0",
			},
		},
	}

	response := sendRequest(t, stdin, stdout, initRequest)

	if response["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %v", response["jsonrpc"])
	}

	if response["id"] != float64(1) {
		t.Errorf("expected id 1, got %v", response["id"])
	}

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected serverInfo to be a map, got %T", result["serverInfo"])
	}

	if serverInfo["name"] != "argocd-mcp-server" {
		t.Errorf("expected server name argocd-mcp-server, got %v", serverInfo["name"])
	}

	capabilities, ok := result["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected capabilities to be a map, got %T", result["capabilities"])
	}

	tools, ok := capabilities["tools"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected tools to be a map, got %T", capabilities["tools"])
	}

	if tools["listChanged"] != true {
		t.Errorf("expected tools.listChanged to be true, got %v", tools["listChanged"])
	}
}

func testListTools(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	listToolsRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}

	response := sendRequest(t, stdin, stdout, listToolsRequest)

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("expected tools to be an array, got %T", result["tools"])
	}

	expectedTools := []string{"list_application", "get_application", "get_application_manifests", "create_application", "sync_application", "delete_application", "list_project", "get_project", "create_project"}
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

	t.Logf("Successfully retrieved %d tools from real ArgoCD server", len(tools))
}

func testListApplications(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "list_application",
			"arguments": map[string]interface{}{},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		t.Logf("Error response: %v", errObj)
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "permission") {
			t.Skip("No permission to list applications on this ArgoCD server")
		}
		t.Fatalf("Error calling list_application: %v", errObj["message"])
	}

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

	t.Logf("Successfully listed applications from real ArgoCD server")
	t.Logf("Response snippet: %.500s...", text)
}

func testInvalidAppName(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application",
			"arguments": map[string]interface{}{
				"name": "non-existent-app-12345",
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		t.Logf("Received expected error for non-existent app: %v", errObj["message"])
	} else if result, ok := response["result"].(map[string]interface{}); ok {
		content, _ := result["content"].([]interface{})
		if len(content) > 0 {
			textContent, _ := content[0].(map[string]interface{})
			text, _ := textContent["text"].(string)
			if strings.Contains(strings.ToLower(text), "not found") || strings.Contains(strings.ToLower(text), "error") {
				t.Log("Received expected error message for non-existent app")
			} else {
				t.Errorf("Expected error for non-existent app, but got: %s", text)
			}
		}
	}
}

func testWithTimeout(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	done := make(chan bool, 1)

	go func() {
		callToolRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name":      "list_application",
				"arguments": map[string]interface{}{},
			},
		}

		response := sendRequest(t, stdin, stdout, callToolRequest)

		if _, ok := response["result"].(map[string]interface{}); ok {
			t.Log("Successfully completed request within timeout")
		}
		done <- true
	}()

	select {
	case <-ctx.Done():
		t.Fatal("Test timed out")
	case <-done:
		t.Log("Test completed successfully")
	}
}

// Application lifecycle tests
func testCreateApplication(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()

	testAppName := os.Getenv("TEST_CREATE_APP_NAME")
	if testAppName == "" {
		testAppName = "test-app-create-e2e"
	}

	testRepoURL := os.Getenv("TEST_CREATE_REPO_URL")
	if testRepoURL == "" {
		testRepoURL = "https://github.com/argoproj/argocd-example-apps.git"
	}

	testPath := os.Getenv("TEST_CREATE_PATH")
	if testPath == "" {
		testPath = "guestbook"
	}

	testDestNamespace := os.Getenv("TEST_CREATE_DEST_NAMESPACE")
	if testDestNamespace == "" {
		testDestNamespace = "default"
	}

	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "create_application",
			"arguments": map[string]interface{}{
				"name":           testAppName,
				"repo_url":       testRepoURL,
				"path":           testPath,
				"dest_namespace": testDestNamespace,
				"upsert":         true,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		errorMsg := fmt.Sprintf("%v", errObj["message"])
		t.Logf("Error response: %v", errObj)

		if strings.Contains(errorMsg, "permission") ||
			strings.Contains(errorMsg, "denied") ||
			strings.Contains(errorMsg, "unauthorized") {
			t.Skip("No permission to create applications on this ArgoCD server")
		}

		if strings.Contains(errorMsg, "already exists") && !strings.Contains(errorMsg, "upsert") {
			t.Logf("Application already exists, which is expected in some test environments")
			testAppCreated = true
			return
		}

		t.Fatalf("Error calling create_application: %v", errorMsg)
	}

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

	if !strings.Contains(text, testAppName) {
		t.Errorf("expected response to contain application name %s", testAppName)
	}

	if !strings.Contains(text, testRepoURL) {
		t.Errorf("expected response to contain repository URL %s", testRepoURL)
	}

	testAppCreated = true
	t.Logf("Successfully created/updated application %s from repository %s", testAppName, testRepoURL)
	t.Logf("Response snippet: %.500s...", text)
}

func testGetCreatedApplication(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()

	if !testAppCreated {
		t.Skip("Skipping test: Application was not created in previous test")
	}

	testAppName := os.Getenv("TEST_CREATE_APP_NAME")
	if testAppName == "" {
		testAppName = "test-app-create-e2e"
	}

	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application",
			"arguments": map[string]interface{}{
				"name": testAppName,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		errorMsg := fmt.Sprintf("%v", errObj["message"])
		t.Logf("Error response: %v", errObj)

		if strings.Contains(errorMsg, "not found") {
			t.Skip("Application not found - create test must run first")
		}

		if strings.Contains(errorMsg, "permission") {
			t.Skip("No permission to get application on this ArgoCD server")
		}

		t.Fatalf("Error calling get_application: %v", errorMsg)
	}

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

	if !strings.Contains(text, testAppName) {
		t.Errorf("expected response to contain application name %s", testAppName)
	}

	t.Logf("Successfully retrieved application %s from real ArgoCD server", testAppName)
	t.Logf("Response snippet: %.500s...", text)
}

func testListApplicationsWithCreated(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()

	if !testAppCreated {
		t.Skip("Skipping test: Application was not created in previous test")
	}

	testAppName := os.Getenv("TEST_CREATE_APP_NAME")
	if testAppName == "" {
		testAppName = "test-app-create-e2e"
	}

	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "list_application",
			"arguments": map[string]interface{}{},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		t.Logf("Error response: %v", errObj)
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "permission") {
			t.Skip("No permission to list applications on this ArgoCD server")
		}
		t.Fatalf("Error calling list_application: %v", errObj["message"])
	}

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

	if !strings.Contains(text, testAppName) {
		t.Errorf("Expected to find created application %s in list", testAppName)
	} else {
		t.Logf("Found created application %s in list", testAppName)
	}

	t.Logf("Successfully listed applications from real ArgoCD server")
	t.Logf("Response snippet: %.500s...", text)
}

func testSyncCreatedApplication(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()

	if !testAppCreated {
		t.Skip("Skipping test: Application was not created in previous test")
	}

	testAppName := os.Getenv("TEST_CREATE_APP_NAME")
	if testAppName == "" {
		testAppName = "test-app-create-e2e"
	}

	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "sync_application",
			"arguments": map[string]interface{}{
				"name":    testAppName,
				"dry_run": false,
				"prune":   false,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		errorMsg := fmt.Sprintf("%v", errObj["message"])
		t.Logf("Error response: %v", errObj)

		if strings.Contains(errorMsg, "not found") {
			t.Skip("Application not found - create test must run first")
		}

		if strings.Contains(errorMsg, "permission") {
			t.Skip("No permission to sync application on this ArgoCD server")
		}

		t.Fatalf("Error calling sync_application: %v", errorMsg)
	}

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

	if !strings.Contains(text, testAppName) {
		t.Errorf("expected response to contain application name %s", testAppName)
	}

	t.Logf("Successfully synced application %s", testAppName)
	t.Logf("Response snippet: %.500s...", text)
}

func testDeleteCreatedApplication(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()

	if !testAppCreated {
		t.Skip("Skipping test: Application was not created in previous test")
	}

	testAppName := os.Getenv("TEST_CREATE_APP_NAME")
	if testAppName == "" {
		testAppName = "test-app-create-e2e"
	}

	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "delete_application",
			"arguments": map[string]interface{}{
				"name":    testAppName,
				"cascade": false,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		errorMsg := fmt.Sprintf("%v", errObj["message"])
		t.Logf("Error response: %v", errObj)

		if strings.Contains(errorMsg, "not found") {
			t.Skip("Application not found - may have already been deleted")
		}

		if strings.Contains(errorMsg, "permission") {
			t.Skip("No permission to delete application on this ArgoCD server")
		}

		t.Fatalf("Error calling delete_application: %v", errorMsg)
	}

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

	testAppCreated = false
	t.Logf("Successfully deleted application %s", testAppName)
	t.Logf("Response snippet: %.500s...", text)
}

// Tests for existing applications
func testGetExistingApplication(t *testing.T) {
	appName := os.Getenv("TEST_APP_NAME")
	if appName == "" {
		t.Skip("Skipping test: TEST_APP_NAME environment variable not set")
	}

	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application",
			"arguments": map[string]interface{}{
				"name": appName,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		t.Logf("Error response: %v", errObj)
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "not found") {
			t.Skipf("Application %s not found on this ArgoCD server", appName)
		}
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "permission") {
			t.Skip("No permission to get application on this ArgoCD server")
		}
		t.Fatalf("Error calling get_application: %v", errObj["message"])
	}

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

	if !strings.Contains(text, appName) {
		t.Errorf("expected response to contain application name %s", appName)
	}

	t.Logf("Successfully retrieved application %s from real ArgoCD server", appName)
	t.Logf("Response snippet: %.500s...", text)
}

func testSyncExistingApplicationDryRun(t *testing.T) {
	appName := os.Getenv("TEST_APP_NAME")
	if appName == "" {
		t.Skip("Skipping test: TEST_APP_NAME environment variable not set")
	}

	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "sync_application",
			"arguments": map[string]interface{}{
				"name":    appName,
				"dry_run": true,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		t.Logf("Error response: %v", errObj)
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "not found") {
			t.Skipf("Application %s not found on this ArgoCD server", appName)
		}
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "permission") {
			t.Skip("No permission to sync application on this ArgoCD server")
		}
		t.Fatalf("Error calling sync_application: %v", errObj["message"])
	}

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

	if !strings.Contains(text, appName) {
		t.Errorf("expected response to contain application name %s", appName)
	}

	t.Logf("Successfully performed dry-run sync for application %s", appName)
	t.Logf("Response snippet: %.500s...", text)
}

// gRPC-Web test functions
func testInitializeGRPCWeb(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServerWithGRPCWeb(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

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
				"name":    "argocd-e2e-test-grpcweb",
				"version": "1.0.0",
			},
		},
	}

	response := sendRequest(t, stdin, stdout, initRequest)

	if response["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %v", response["jsonrpc"])
	}

	if response["id"] != float64(1) {
		t.Errorf("expected id 1, got %v", response["id"])
	}

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected serverInfo to be a map, got %T", result["serverInfo"])
	}

	if serverInfo["name"] != "argocd-mcp-server" {
		t.Errorf("expected server name argocd-mcp-server, got %v", serverInfo["name"])
	}

	t.Log("Successfully initialized MCP server with gRPC-Web mode")
}

func testListToolsGRPCWeb(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServerWithGRPCWeb(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	listToolsRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}

	response := sendRequest(t, stdin, stdout, listToolsRequest)

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be a map, got %T", response["result"])
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("expected tools to be an array, got %T", result["tools"])
	}

	expectedTools := []string{"list_application", "get_application", "get_application_manifests", "create_application", "sync_application", "delete_application", "list_project", "get_project", "create_project"}
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

	t.Logf("Successfully retrieved %d tools via gRPC-Web", len(tools))
}

func testListApplicationsGRPCWeb(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServerWithGRPCWeb(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "list_application",
			"arguments": map[string]interface{}{},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		t.Logf("Error response: %v", errObj)
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "permission") {
			t.Skip("No permission to list applications on this ArgoCD server")
		}
		t.Fatalf("Error calling list_application: %v", errObj["message"])
	}

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

	t.Logf("Successfully listed applications via gRPC-Web")
	t.Logf("Response snippet: %.500s...", text)
}

func testInvalidAppNameGRPCWeb(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServerWithGRPCWeb(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application",
			"arguments": map[string]interface{}{
				"name": "non-existent-app-12345",
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		t.Logf("Received expected error for non-existent app via gRPC-Web: %v", errObj["message"])
	} else if result, ok := response["result"].(map[string]interface{}); ok {
		content, _ := result["content"].([]interface{})
		if len(content) > 0 {
			textContent, _ := content[0].(map[string]interface{})
			text, _ := textContent["text"].(string)
			if strings.Contains(strings.ToLower(text), "not found") || strings.Contains(strings.ToLower(text), "error") {
				t.Log("Received expected error message for non-existent app via gRPC-Web")
			} else {
				t.Errorf("Expected error for non-existent app, but got: %s", text)
			}
		}
	}
}

func testWithTimeoutGRPCWeb(t *testing.T) {
	mcpCmd, stdin, stdout := startMCPServerWithGRPCWeb(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	done := make(chan bool, 1)

	go func() {
		callToolRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name":      "list_application",
				"arguments": map[string]interface{}{},
			},
		}

		response := sendRequest(t, stdin, stdout, callToolRequest)

		if _, ok := response["result"].(map[string]interface{}); ok {
			t.Log("Successfully completed request within timeout via gRPC-Web")
		}
		done <- true
	}()

	select {
	case <-ctx.Done():
		t.Fatal("Test timed out")
	case <-done:
		t.Log("Test completed successfully via gRPC-Web")
	}
}

// Application lifecycle tests for gRPC-Web
func testCreateApplicationGRPCWeb(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()

	testAppName := os.Getenv("TEST_CREATE_APP_NAME")
	if testAppName == "" {
		testAppName = "test-app-create-e2e-grpcweb"
	}

	testRepoURL := os.Getenv("TEST_CREATE_REPO_URL")
	if testRepoURL == "" {
		testRepoURL = "https://github.com/argoproj/argocd-example-apps.git"
	}

	testPath := os.Getenv("TEST_CREATE_PATH")
	if testPath == "" {
		testPath = "guestbook"
	}

	testDestNamespace := os.Getenv("TEST_CREATE_DEST_NAMESPACE")
	if testDestNamespace == "" {
		testDestNamespace = "default"
	}

	mcpCmd, stdin, stdout := startMCPServerWithGRPCWeb(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "create_application",
			"arguments": map[string]interface{}{
				"name":           testAppName,
				"repo_url":       testRepoURL,
				"path":           testPath,
				"dest_namespace": testDestNamespace,
				"upsert":         true,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		errorMsg := fmt.Sprintf("%v", errObj["message"])
		t.Logf("Error response: %v", errObj)

		if strings.Contains(errorMsg, "permission") ||
			strings.Contains(errorMsg, "denied") ||
			strings.Contains(errorMsg, "unauthorized") {
			t.Skip("No permission to create applications on this ArgoCD server")
		}

		if strings.Contains(errorMsg, "already exists") && !strings.Contains(errorMsg, "upsert") {
			t.Logf("Application already exists, which is expected in some test environments")
			testAppCreated = true
			return
		}

		t.Fatalf("Error calling create_application: %v", errorMsg)
	}

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

	if !strings.Contains(text, testAppName) {
		t.Errorf("expected response to contain application name %s", testAppName)
	}

	testAppCreated = true
	t.Logf("Successfully created/updated application %s via gRPC-Web", testAppName)
	t.Logf("Response snippet: %.500s...", text)
}

func testGetCreatedApplicationGRPCWeb(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()

	if !testAppCreated {
		t.Skip("Skipping test: Application was not created in previous test")
	}

	testAppName := os.Getenv("TEST_CREATE_APP_NAME")
	if testAppName == "" {
		testAppName = "test-app-create-e2e-grpcweb"
	}

	mcpCmd, stdin, stdout := startMCPServerWithGRPCWeb(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application",
			"arguments": map[string]interface{}{
				"name": testAppName,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		errorMsg := fmt.Sprintf("%v", errObj["message"])
		t.Logf("Error response: %v", errObj)

		if strings.Contains(errorMsg, "not found") {
			t.Skip("Application not found - create test must run first")
		}

		if strings.Contains(errorMsg, "permission") {
			t.Skip("No permission to get application on this ArgoCD server")
		}

		t.Fatalf("Error calling get_application: %v", errorMsg)
	}

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

	if !strings.Contains(text, testAppName) {
		t.Errorf("expected response to contain application name %s", testAppName)
	}

	t.Logf("Successfully retrieved application %s via gRPC-Web", testAppName)
	t.Logf("Response snippet: %.500s...", text)
}

func testListApplicationsWithCreatedGRPCWeb(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()

	if !testAppCreated {
		t.Skip("Skipping test: Application was not created in previous test")
	}

	testAppName := os.Getenv("TEST_CREATE_APP_NAME")
	if testAppName == "" {
		testAppName = "test-app-create-e2e-grpcweb"
	}

	mcpCmd, stdin, stdout := startMCPServerWithGRPCWeb(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "list_application",
			"arguments": map[string]interface{}{},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		t.Logf("Error response: %v", errObj)
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "permission") {
			t.Skip("No permission to list applications on this ArgoCD server")
		}
		t.Fatalf("Error calling list_application: %v", errObj["message"])
	}

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

	if !strings.Contains(text, testAppName) {
		t.Errorf("Expected to find created application %s in list", testAppName)
	} else {
		t.Logf("Found created application %s in list via gRPC-Web", testAppName)
	}

	t.Logf("Successfully listed applications via gRPC-Web")
	t.Logf("Response snippet: %.500s...", text)
}

func testSyncCreatedApplicationGRPCWeb(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()

	if !testAppCreated {
		t.Skip("Skipping test: Application was not created in previous test")
	}

	testAppName := os.Getenv("TEST_CREATE_APP_NAME")
	if testAppName == "" {
		testAppName = "test-app-create-e2e-grpcweb"
	}

	mcpCmd, stdin, stdout := startMCPServerWithGRPCWeb(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "sync_application",
			"arguments": map[string]interface{}{
				"name":    testAppName,
				"dry_run": false,
				"prune":   false,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		errorMsg := fmt.Sprintf("%v", errObj["message"])
		t.Logf("Error response: %v", errObj)

		if strings.Contains(errorMsg, "not found") {
			t.Skip("Application not found - create test must run first")
		}

		if strings.Contains(errorMsg, "permission") {
			t.Skip("No permission to sync application on this ArgoCD server")
		}

		t.Fatalf("Error calling sync_application: %v", errorMsg)
	}

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

	if !strings.Contains(text, testAppName) {
		t.Errorf("expected response to contain application name %s", testAppName)
	}

	t.Logf("Successfully synced application %s via gRPC-Web", testAppName)
	t.Logf("Response snippet: %.500s...", text)
}

func testDeleteCreatedApplicationGRPCWeb(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()

	if !testAppCreated {
		t.Skip("Skipping test: Application was not created in previous test")
	}

	testAppName := os.Getenv("TEST_CREATE_APP_NAME")
	if testAppName == "" {
		testAppName = "test-app-create-e2e-grpcweb"
	}

	mcpCmd, stdin, stdout := startMCPServerWithGRPCWeb(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "delete_application",
			"arguments": map[string]interface{}{
				"name":    testAppName,
				"cascade": false,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		errorMsg := fmt.Sprintf("%v", errObj["message"])
		t.Logf("Error response: %v", errObj)

		if strings.Contains(errorMsg, "not found") {
			t.Skip("Application not found - may have already been deleted")
		}

		if strings.Contains(errorMsg, "permission") {
			t.Skip("No permission to delete application on this ArgoCD server")
		}

		t.Fatalf("Error calling delete_application: %v", errorMsg)
	}

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

	testAppCreated = false
	t.Logf("Successfully deleted application %s via gRPC-Web", testAppName)
	t.Logf("Response snippet: %.500s...", text)
}

// Tests for existing applications with gRPC-Web
func testGetExistingApplicationGRPCWeb(t *testing.T) {
	appName := os.Getenv("TEST_APP_NAME")
	if appName == "" {
		t.Skip("Skipping test: TEST_APP_NAME environment variable not set")
	}

	mcpCmd, stdin, stdout := startMCPServerWithGRPCWeb(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application",
			"arguments": map[string]interface{}{
				"name": appName,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		t.Logf("Error response: %v", errObj)
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "not found") {
			t.Skipf("Application %s not found on this ArgoCD server", appName)
		}
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "permission") {
			t.Skip("No permission to get application on this ArgoCD server")
		}
		t.Fatalf("Error calling get_application: %v", errObj["message"])
	}

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

	if !strings.Contains(text, appName) {
		t.Errorf("expected response to contain application name %s", appName)
	}

	t.Logf("Successfully retrieved application %s via gRPC-Web", appName)
	t.Logf("Response snippet: %.500s...", text)
}

func testSyncExistingApplicationDryRunGRPCWeb(t *testing.T) {
	appName := os.Getenv("TEST_APP_NAME")
	if appName == "" {
		t.Skip("Skipping test: TEST_APP_NAME environment variable not set")
	}

	mcpCmd, stdin, stdout := startMCPServerWithGRPCWeb(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "sync_application",
			"arguments": map[string]interface{}{
				"name":    appName,
				"dry_run": true,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		t.Logf("Error response: %v", errObj)
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "not found") {
			t.Skipf("Application %s not found on this ArgoCD server", appName)
		}
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "permission") {
			t.Skip("No permission to sync application on this ArgoCD server")
		}
		t.Fatalf("Error calling sync_application: %v", errObj["message"])
	}

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

	if !strings.Contains(text, appName) {
		t.Errorf("expected response to contain application name %s", appName)
	}

	t.Logf("Successfully performed dry-run sync for application %s via gRPC-Web", appName)
	t.Logf("Response snippet: %.500s...", text)
}

// Test get_application_manifests for existing application
func testGetExistingApplicationEvents(t *testing.T) {
	appName := os.Getenv("TEST_APP_NAME")
	if appName == "" {
		t.Skip("Skipping test: TEST_APP_NAME environment variable not set")
	}

	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application_events",
			"arguments": map[string]interface{}{
				"name": appName,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		t.Logf("Error response: %v", errObj)
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "not found") {
			t.Skipf("Application %s not found on this ArgoCD server", appName)
		}
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "permission") {
			t.Skip("No permission to get application events on this ArgoCD server")
		}
		t.Fatalf("Error calling get_application_events: %v", errObj["message"])
	}

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
	var eventsResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &eventsResp); err != nil {
		t.Fatalf("expected response to be valid JSON: %v", err)
	}

	// Check for expected fields in events response
	if _, ok := eventsResp["items"]; !ok {
		t.Error("expected response to contain items field")
	}

	t.Logf("Successfully retrieved events for application %s", appName)
	t.Logf("Response snippet: %.500s...", text)
}

func testGetExistingApplicationManifests(t *testing.T) {
	appName := os.Getenv("TEST_APP_NAME")
	if appName == "" {
		t.Skip("Skipping test: TEST_APP_NAME environment variable not set")
	}

	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application_manifests",
			"arguments": map[string]interface{}{
				"name": appName,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		t.Logf("Error response: %v", errObj)
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "not found") {
			t.Skipf("Application %s not found on this ArgoCD server", appName)
		}
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "permission") {
			t.Skip("No permission to get application manifests on this ArgoCD server")
		}
		t.Fatalf("Error calling get_application_manifests: %v", errObj["message"])
	}

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

	t.Logf("Successfully retrieved manifests for application %s", appName)
	t.Logf("Response snippet: %.500s...", text)
}

// Test get_application_manifests for created application
func testGetCreatedApplicationEvents(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()

	if !testAppCreated {
		t.Skip("Skipping test: Application was not created in previous test")
	}

	testAppName := os.Getenv("TEST_CREATE_APP_NAME")
	if testAppName == "" {
		testAppName = "test-app-create-e2e"
	}

	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application_events",
			"arguments": map[string]interface{}{
				"name": testAppName,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		errorMsg := fmt.Sprintf("%v", errObj["message"])
		t.Logf("Error response: %v", errObj)

		if strings.Contains(errorMsg, "not found") {
			t.Skip("Application not found - create test must run first")
		}

		if strings.Contains(errorMsg, "permission") {
			t.Skip("No permission to get application events on this ArgoCD server")
		}

		t.Fatalf("Error calling get_application_events: %v", errorMsg)
	}

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
	var eventsResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &eventsResp); err != nil {
		t.Fatalf("expected response to be valid JSON: %v", err)
	}

	t.Logf("Successfully retrieved events for created application %s", testAppName)
	t.Logf("Response snippet: %.500s...", text)
}

func testGetCreatedApplicationManifests(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()

	if !testAppCreated {
		t.Skip("Skipping test: Application was not created in previous test")
	}

	testAppName := os.Getenv("TEST_CREATE_APP_NAME")
	if testAppName == "" {
		testAppName = "test-app-create-e2e"
	}

	mcpCmd, stdin, stdout := startMCPServer(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application_manifests",
			"arguments": map[string]interface{}{
				"name": testAppName,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		errorMsg := fmt.Sprintf("%v", errObj["message"])
		t.Logf("Error response: %v", errObj)

		if strings.Contains(errorMsg, "not found") {
			t.Skip("Application not found - create test must run first")
		}

		if strings.Contains(errorMsg, "permission") {
			t.Skip("No permission to get application manifests on this ArgoCD server")
		}

		t.Fatalf("Error calling get_application_manifests: %v", errorMsg)
	}

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

	t.Logf("Successfully retrieved manifests for created application %s", testAppName)
	t.Logf("Response snippet: %.500s...", text)
}

// gRPC-Web test for get_application_events
func testGetExistingApplicationEventsGRPCWeb(t *testing.T) {
	appName := os.Getenv("TEST_APP_NAME")
	if appName == "" {
		t.Skip("Skipping test: TEST_APP_NAME environment variable not set")
	}

	mcpCmd, stdin, stdout := startMCPServerWithGRPCWeb(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application_events",
			"arguments": map[string]interface{}{
				"name": appName,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		t.Logf("Error response: %v", errObj)
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "not found") {
			t.Skipf("Application %s not found on this ArgoCD server", appName)
		}
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "permission") {
			t.Skip("No permission to get application events on this ArgoCD server")
		}
		t.Fatalf("Error calling get_application_events: %v", errObj["message"])
	}

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
	var eventsResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &eventsResp); err != nil {
		t.Fatalf("expected response to be valid JSON: %v", err)
	}

	t.Logf("Successfully retrieved events for application %s via gRPC-Web", appName)
	t.Logf("Response snippet: %.500s...", text)
}

// gRPC-Web test for get_application_manifests
func testGetExistingApplicationManifestsGRPCWeb(t *testing.T) {
	appName := os.Getenv("TEST_APP_NAME")
	if appName == "" {
		t.Skip("Skipping test: TEST_APP_NAME environment variable not set")
	}

	mcpCmd, stdin, stdout := startMCPServerWithGRPCWeb(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application_manifests",
			"arguments": map[string]interface{}{
				"name": appName,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		t.Logf("Error response: %v", errObj)
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "not found") {
			t.Skipf("Application %s not found on this ArgoCD server", appName)
		}
		if strings.Contains(fmt.Sprintf("%v", errObj["message"]), "permission") {
			t.Skip("No permission to get application manifests on this ArgoCD server")
		}
		t.Fatalf("Error calling get_application_manifests: %v", errObj["message"])
	}

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

	t.Logf("Successfully retrieved manifests for application %s via gRPC-Web", appName)
	t.Logf("Response snippet: %.500s...", text)
}

// gRPC-Web test for get_application_events for created application
func testGetCreatedApplicationEventsGRPCWeb(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()

	if !testAppCreated {
		t.Skip("Skipping test: Application was not created in previous test")
	}

	testAppName := os.Getenv("TEST_CREATE_APP_NAME")
	if testAppName == "" {
		testAppName = "test-app-create-e2e-grpcweb"
	}

	mcpCmd, stdin, stdout := startMCPServerWithGRPCWeb(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application_events",
			"arguments": map[string]interface{}{
				"name": testAppName,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		errorMsg := fmt.Sprintf("%v", errObj["message"])
		t.Logf("Error response: %v", errObj)

		if strings.Contains(errorMsg, "not found") {
			t.Skip("Application not found - create test must run first")
		}

		if strings.Contains(errorMsg, "permission") {
			t.Skip("No permission to get application events on this ArgoCD server")
		}

		t.Fatalf("Error calling get_application_events: %v", errorMsg)
	}

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

	// Check if the response is an error message
	if strings.Contains(text, "Failed to get application events") {
		t.Logf("Error in text response: %s", text)

		if strings.Contains(text, "PermissionDenied") || strings.Contains(text, "permission denied") {
			t.Skip("No permission to get application events via gRPC-Web on this ArgoCD server")
		}

		t.Fatalf("Unexpected error in response: %s", text)
	}

	// Parse JSON to validate structure
	var eventsResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &eventsResp); err != nil {
		t.Fatalf("expected response to be valid JSON: %v", err)
	}

	t.Logf("Successfully retrieved events for created application %s via gRPC-Web", testAppName)
	t.Logf("Response snippet: %.500s...", text)
}

// gRPC-Web test for get_application_manifests for created application
func testGetCreatedApplicationManifestsGRPCWeb(t *testing.T) {
	testMutex.Lock()
	defer testMutex.Unlock()

	if !testAppCreated {
		t.Skip("Skipping test: Application was not created in previous test")
	}

	testAppName := os.Getenv("TEST_CREATE_APP_NAME")
	if testAppName == "" {
		testAppName = "test-app-create-e2e-grpcweb"
	}

	mcpCmd, stdin, stdout := startMCPServerWithGRPCWeb(t)
	defer func() {
		_ = mcpCmd.Process.Kill()
		_ = mcpCmd.Wait()
	}()

	initializeMCPConnection(t, stdin, stdout)

	callToolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_application_manifests",
			"arguments": map[string]interface{}{
				"name": testAppName,
			},
		},
	}

	response := sendRequest(t, stdin, stdout, callToolRequest)

	if errObj, ok := response["error"].(map[string]interface{}); ok {
		errorMsg := fmt.Sprintf("%v", errObj["message"])
		t.Logf("Error response: %v", errObj)

		if strings.Contains(errorMsg, "not found") {
			t.Skip("Application not found - create test must run first")
		}

		if strings.Contains(errorMsg, "permission") {
			t.Skip("No permission to get application manifests on this ArgoCD server")
		}

		t.Fatalf("Error calling get_application_manifests: %v", errorMsg)
	}

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

	// Check if the response is an error message
	if strings.Contains(text, "Failed to get application manifests") {
		t.Logf("Error in text response: %s", text)

		if strings.Contains(text, "PermissionDenied") || strings.Contains(text, "permission denied") {
			t.Skip("No permission to get application manifests via gRPC-Web on this ArgoCD server")
		}

		if strings.Contains(text, "not found") {
			t.Skip("Application not found - create test must run first")
		}

		t.Fatalf("Error getting manifests: %s", text)
	}

	// Parse JSON to validate structure
	var manifestResp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &manifestResp); err != nil {
		t.Logf("Raw text response: %q", text)
		if len(text) > 100 {
			t.Logf("First 100 chars: %q", text[:100])
		}
		t.Fatalf("expected response to be valid JSON: %v", err)
	}

	t.Logf("Successfully retrieved manifests for created application %s via gRPC-Web", testAppName)
	t.Logf("Response snippet: %.500s...", text)
}
