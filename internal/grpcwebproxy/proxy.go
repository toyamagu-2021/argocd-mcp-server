package grpcwebproxy

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// GRPCWebProxy handles gRPC-Web to gRPC translation
type GRPCWebProxy struct {
	serverAddr      string
	plainText       bool
	httpClient      *http.Client
	grpcWebRootPath string
	headers         []string

	// Proxy management
	proxyMutex      sync.Mutex
	proxyListener   net.Listener
	proxyServer     *grpc.Server
	proxyUsersCount int
}

// NewGRPCWebProxy creates a new gRPC-Web proxy
func NewGRPCWebProxy(serverAddr string, plainText bool, httpClient *http.Client, grpcWebRootPath string, headers []string) *GRPCWebProxy {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	return &GRPCWebProxy{
		serverAddr:      serverAddr,
		plainText:       plainText,
		httpClient:      httpClient,
		grpcWebRootPath: grpcWebRootPath,
		headers:         headers,
	}
}

// executeRequest sends a gRPC-Web request and returns the response
func (p *GRPCWebProxy) executeRequest(fullMethodName string, msg []byte, md metadata.MD) (*http.Response, error) {
	// Construct URL
	schema := "https"
	if p.plainText {
		schema = "http"
	}

	rootPath := p.grpcWebRootPath
	if rootPath == "" {
		rootPath = ""
	} else if !strings.HasPrefix(rootPath, "/") {
		rootPath = "/" + rootPath
	}

	requestURL := fmt.Sprintf("%s://%s%s%s", schema, p.serverAddr, rootPath, fullMethodName)

	// Create request with framed message
	req, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewReader(toFrame(msg)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("content-type", "application/grpc-web+proto")

	// Copy metadata to headers, skipping special gRPC headers
	for k, v := range md {
		// Skip special gRPC headers that start with ":"
		if strings.HasPrefix(k, ":") {
			continue
		}
		for _, val := range v {
			req.Header.Add(k, val)
		}
	}

	// Add custom headers
	for _, h := range p.headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}

	// Execute request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	return resp, nil
}

// startGRPCProxy starts the local gRPC proxy server
func (p *GRPCWebProxy) startGRPCProxy() (*grpc.Server, net.Listener, error) {
	// Generate random suffix for Unix socket
	randBytes := make([]byte, 8)
	if _, err := rand.Read(randBytes); err != nil {
		return nil, nil, fmt.Errorf("failed to generate random suffix: %w", err)
	}
	randSuffix := hex.EncodeToString(randBytes)

	// Create Unix socket
	socketPath := fmt.Sprintf("%s/argocd-mcp-%s.sock", os.TempDir(), randSuffix)
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create Unix socket: %w", err)
	}

	// Create gRPC server with custom codec
	proxySrv := grpc.NewServer(
		grpc.ForceServerCodec(&noopCodec{}),
		grpc.UnknownServiceHandler(func(srv interface{}, stream grpc.ServerStream) error {
			return p.handleStream(stream)
		}),
	)

	// Start server in background
	go func() {
		if err := proxySrv.Serve(ln); err != nil {
			// Log error if needed
		}
	}()

	return proxySrv, ln, nil
}

// handleStream handles a single gRPC stream through the proxy
func (p *GRPCWebProxy) handleStream(stream grpc.ServerStream) error {
	ctx := stream.Context()
	fullMethodName, ok := grpc.Method(ctx)
	if !ok {
		return status.Error(codes.Internal, "failed to get method name")
	}

	// Get metadata from context
	md, _ := metadata.FromIncomingContext(ctx)

	// Handle client streaming - process first message only for unary calls
	var msg []byte
	if err := stream.RecvMsg(&msg); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	// Execute gRPC-Web request
	resp, err := p.executeRequest(fullMethodName, msg, md)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	defer func() { _ = resp.Body.Close() }()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		// Check for gRPC status in headers even for non-200 responses
		if grpcStatus := resp.Header.Get("Grpc-Status"); grpcStatus != "" {
			grpcMessage := resp.Header.Get("Grpc-Message")
			code, _ := strconv.Atoi(grpcStatus)
			return status.Error(codes.Code(code), grpcMessage)
		}
		body, _ := io.ReadAll(resp.Body)
		return status.Error(codes.Internal, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
	}

	// Check for gRPC status in headers (for immediate errors)
	if grpcStatus := resp.Header.Get("Grpc-Status"); grpcStatus != "" && grpcStatus != "0" {
		grpcMessage := resp.Header.Get("Grpc-Message")
		code, _ := strconv.Atoi(grpcStatus)
		return status.Error(codes.Code(code), grpcMessage)
	}

	// Read and parse response frames
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return status.Error(codes.Internal, fmt.Sprintf("failed to read response: %v", err))
	}

	// Send response frames back to client
	reader := bytes.NewReader(respBody)
	var trailerFrame []byte
	for {
		frame, isTrailer, err := parseFrame(reader)
		if err != nil {
			if err == io.EOF {
				break
			}
			return status.Error(codes.Internal, fmt.Sprintf("failed to parse frame: %v", err))
		}

		// Check if this is a trailer frame
		if isTrailer {
			// Store trailer for later processing
			trailerFrame = frame
			break
		}

		// Send data frame to client
		if err := stream.SendMsg(frame); err != nil {
			return err
		}
	}

	// Handle trailer if present
	if trailerFrame != nil {
		// Parse gRPC status from trailer
		if grpcErr := parseGRPCTrailer(trailerFrame); grpcErr != nil {
			return grpcErr
		}
	}

	return nil
}

// UseProxy returns a proxy address and a closer function
func (p *GRPCWebProxy) UseProxy() (net.Addr, io.Closer, error) {
	p.proxyMutex.Lock()
	defer p.proxyMutex.Unlock()

	// Start proxy if not already running
	if p.proxyListener == nil {
		server, listener, err := p.startGRPCProxy()
		if err != nil {
			return nil, nil, err
		}
		p.proxyServer = server
		p.proxyListener = listener
	}

	// Increment user count
	p.proxyUsersCount++

	// Return closer that decrements count and stops proxy if no users
	closer := &proxyCloser{
		proxy: p,
	}

	return p.proxyListener.Addr(), closer, nil
}

// proxyCloser implements io.Closer for proxy cleanup
type proxyCloser struct {
	proxy *GRPCWebProxy
}

func (c *proxyCloser) Close() error {
	c.proxy.proxyMutex.Lock()
	defer c.proxy.proxyMutex.Unlock()

	c.proxy.proxyUsersCount--
	if c.proxy.proxyUsersCount == 0 && c.proxy.proxyServer != nil {
		c.proxy.proxyServer.Stop()
		c.proxy.proxyListener = nil
		c.proxy.proxyServer = nil
	}

	return nil
}

// parseGRPCTrailer parses the gRPC status from a trailer frame
func parseGRPCTrailer(trailer []byte) error {
	// Trailers are in HTTP/2 header format
	// Simple parser for grpc-status and grpc-message
	trailerStr := string(trailer)
	lines := strings.Split(trailerStr, "\r\n")

	var grpcStatus int
	var grpcMessage string

	for _, line := range lines {
		if strings.HasPrefix(line, "grpc-status:") {
			statusStr := strings.TrimPrefix(line, "grpc-status:")
			statusStr = strings.TrimSpace(statusStr)
			grpcStatus, _ = strconv.Atoi(statusStr)
		} else if strings.HasPrefix(line, "grpc-message:") {
			grpcMessage = strings.TrimPrefix(line, "grpc-message:")
			grpcMessage = strings.TrimSpace(grpcMessage)
		}
	}

	if grpcStatus != 0 {
		return status.Error(codes.Code(grpcStatus), grpcMessage)
	}

	return nil
}
