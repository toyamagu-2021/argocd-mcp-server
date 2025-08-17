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
	"syscall"
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
		// Kill the entire process group to ensure all child processes are terminated
		pgid, _ := syscall.Getpgid(s.cmd.Process.Pid)
		_ = syscall.Kill(-pgid, syscall.SIGTERM)

		// Give it time to shutdown gracefully
		done := make(chan error, 1)
		go func() {
			done <- s.cmd.Wait()
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

func startMockServerForTests(port string) *testServer {
	// Kill any existing process using the port
	killProcessOnPort(port)

	// Wait a bit for the port to be fully released
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, "go", "run", "../mock/server.go", "-port", port)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Set process group ID to enable killing all child processes
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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
	
	// Set process group ID to enable killing all child processes
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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
