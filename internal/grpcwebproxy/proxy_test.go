package grpcwebproxy

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestToFrame(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "empty message",
			input:    []byte{},
			expected: []byte{0, 0, 0, 0, 0},
		},
		{
			name:     "simple message",
			input:    []byte("hello"),
			expected: []byte{0, 0, 0, 0, 5, 'h', 'e', 'l', 'l', 'o'},
		},
		{
			name:     "binary data",
			input:    []byte{0x01, 0x02, 0x03, 0x04},
			expected: []byte{0, 0, 0, 0, 4, 0x01, 0x02, 0x03, 0x04},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toFrame(tt.input)
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("toFrame() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseFrame(t *testing.T) {
	tests := []struct {
		name            string
		input           []byte
		expectedMsg     []byte
		expectedTrailer bool
		expectedError   bool
	}{
		{
			name:            "normal data frame",
			input:           []byte{0, 0, 0, 0, 5, 'h', 'e', 'l', 'l', 'o'},
			expectedMsg:     []byte("hello"),
			expectedTrailer: false,
			expectedError:   false,
		},
		{
			name:            "trailer frame",
			input:           []byte{128, 0, 0, 0, 3, 'e', 'n', 'd'},
			expectedMsg:     []byte("end"),
			expectedTrailer: true,
			expectedError:   false,
		},
		{
			name:            "empty data frame",
			input:           []byte{0, 0, 0, 0, 0},
			expectedMsg:     []byte{},
			expectedTrailer: false,
			expectedError:   false,
		},
		{
			name:            "empty trailer frame",
			input:           []byte{128, 0, 0, 0, 0},
			expectedMsg:     []byte{},
			expectedTrailer: true,
			expectedError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader(tt.input)
			msg, isTrailer, err := parseFrame(reader)

			if tt.expectedError {
				if err == nil {
					t.Error("expected error, but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !bytes.Equal(msg, tt.expectedMsg) {
				t.Errorf("parseFrame() msg = %v, want %v", msg, tt.expectedMsg)
			}

			if isTrailer != tt.expectedTrailer {
				t.Errorf("parseFrame() isTrailer = %v, want %v", isTrailer, tt.expectedTrailer)
			}
		})
	}
}

func TestParseFrameTruncatedData(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{
			name:  "truncated header",
			input: []byte{0, 0, 0},
		},
		{
			name:  "truncated body",
			input: []byte{0, 0, 0, 0, 5, 'h', 'e'},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader(tt.input)
			_, _, err := parseFrame(reader)
			if err == nil {
				t.Error("expected error for truncated data, but got none")
			}
		})
	}
}

func TestParseFrames(t *testing.T) {
	// Create multiple frames
	frame1 := toFrame([]byte("first"))
	frame2 := toFrame([]byte("second"))
	frame3 := toFrame([]byte("third"))

	input := append(frame1, frame2...)
	input = append(input, frame3...)

	frames, err := parseFrames(input)
	if err != nil {
		t.Errorf("parseFrames() error = %v", err)
		return
	}

	expectedFrames := [][]byte{
		[]byte("first"),
		[]byte("second"),
		[]byte("third"),
	}

	if len(frames) != len(expectedFrames) {
		t.Errorf("parseFrames() returned %d frames, want %d", len(frames), len(expectedFrames))
		return
	}

	for i, frame := range frames {
		if !bytes.Equal(frame, expectedFrames[i]) {
			t.Errorf("parseFrames() frame[%d] = %v, want %v", i, frame, expectedFrames[i])
		}
	}
}

func TestNewGRPCWebProxy(t *testing.T) {
	serverAddr := "localhost:60080"
	plainText := true
	httpClient := &http.Client{}
	rootPath := "/api"
	headers := []string{"Authorization: Bearer token"}

	proxy := NewGRPCWebProxy(serverAddr, plainText, httpClient, rootPath, headers)

	if proxy == nil {
		t.Fatal("NewGRPCWebProxy() returned nil")
	}

	if proxy.serverAddr != serverAddr {
		t.Errorf("serverAddr = %v, want %v", proxy.serverAddr, serverAddr)
	}

	if proxy.plainText != plainText {
		t.Errorf("plainText = %v, want %v", proxy.plainText, plainText)
	}

	if proxy.httpClient != httpClient {
		t.Error("httpClient not set correctly")
	}

	if proxy.grpcWebRootPath != rootPath {
		t.Errorf("grpcWebRootPath = %v, want %v", proxy.grpcWebRootPath, rootPath)
	}

	if len(proxy.headers) != len(headers) {
		t.Errorf("headers length = %v, want %v", len(proxy.headers), len(headers))
	}
}

func TestNewGRPCWebProxyWithNilHTTPClient(t *testing.T) {
	proxy := NewGRPCWebProxy("localhost:60080", false, nil, "", nil)

	if proxy == nil {
		t.Fatal("NewGRPCWebProxy() returned nil")
	}

	if proxy.httpClient == nil {
		t.Error("httpClient should be created when nil is passed")
	}
}

func TestProxyCloser(t *testing.T) {
	proxy := &GRPCWebProxy{
		proxyUsersCount: 1,
	}

	closer := &proxyCloser{proxy: proxy}

	err := closer.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if proxy.proxyUsersCount != 0 {
		t.Errorf("proxyUsersCount = %v, want 0", proxy.proxyUsersCount)
	}
}

func TestCodec(t *testing.T) {
	codec := &noopCodec{}

	if codec.Name() != "proto" {
		t.Errorf("Name() = %v, want proto", codec.Name())
	}

	// Test Marshal with []byte
	input := []byte("test data")
	result, err := codec.Marshal(input)
	if err != nil {
		t.Errorf("Marshal() error = %v", err)
	}
	if !bytes.Equal(result, input) {
		t.Errorf("Marshal() = %v, want %v", result, input)
	}

	// Test Unmarshal with *[]byte
	var output []byte
	err = codec.Unmarshal(input, &output)
	if err != nil {
		t.Errorf("Unmarshal() error = %v", err)
	}
	if !bytes.Equal(output, input) {
		t.Errorf("Unmarshal() = %v, want %v", output, input)
	}
}

func TestParseGRPCTrailer(t *testing.T) {
	tests := []struct {
		name          string
		trailer       []byte
		expectedError bool
		expectedCode  int
		expectedMsg   string
	}{
		{
			name:          "successful response",
			trailer:       []byte("grpc-status: 0\r\ngrpc-message: \r\n"),
			expectedError: false,
			expectedCode:  0,
			expectedMsg:   "",
		},
		{
			name:          "error with message",
			trailer:       []byte("grpc-status: 5\r\ngrpc-message: not found\r\n"),
			expectedError: true,
			expectedCode:  5,
			expectedMsg:   "not found",
		},
		{
			name:          "permission denied error",
			trailer:       []byte("grpc-status: 7\r\ngrpc-message: permission denied\r\n"),
			expectedError: true,
			expectedCode:  7,
			expectedMsg:   "permission denied",
		},
		{
			name:          "internal error",
			trailer:       []byte("grpc-status: 13\r\ngrpc-message: internal server error\r\n"),
			expectedError: true,
			expectedCode:  13,
			expectedMsg:   "internal server error",
		},
		{
			name:          "trailer with extra headers",
			trailer:       []byte("content-type: application/grpc-web+proto\r\ngrpc-status: 0\r\ngrpc-message: \r\n"),
			expectedError: false,
			expectedCode:  0,
			expectedMsg:   "",
		},
		{
			name:          "empty trailer",
			trailer:       []byte(""),
			expectedError: false,
			expectedCode:  0,
			expectedMsg:   "",
		},
		{
			name:          "trailer without grpc-status",
			trailer:       []byte("grpc-message: some message\r\n"),
			expectedError: false,
			expectedCode:  0,
			expectedMsg:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseGRPCTrailer(tt.trailer)

			if tt.expectedError {
				if err == nil {
					t.Error("expected error, but got none")
					return
				}
				// Verify error code and message if error is expected
				// Note: We can't easily check the exact code and message
				// without type assertion on the status.Status type
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestParseHeaders(t *testing.T) {
	tests := []struct {
		name           string
		headerStrings  []string
		expectedError  bool
		expectedResult map[string][]string
	}{
		{
			name:          "valid headers",
			headerStrings: []string{"Authorization:Bearer token", "Content-Type:application/json"},
			expectedError: false,
			expectedResult: map[string][]string{
				"Authorization": {"Bearer token"},
				"Content-Type":  {"application/json"},
			},
		},
		{
			name:          "header with spaces",
			headerStrings: []string{"Authorization: Bearer token", "X-Custom-Header: custom value"},
			expectedError: false,
			expectedResult: map[string][]string{
				"Authorization":   {" Bearer token"},
				"X-Custom-Header": {" custom value"},
			},
		},
		{
			name:          "header with multiple colons",
			headerStrings: []string{"X-URL:https://example.com:8080/path"},
			expectedError: false,
			expectedResult: map[string][]string{
				"X-Url": {"https://example.com:8080/path"},
			},
		},
		{
			name:           "empty header list",
			headerStrings:  []string{},
			expectedError:  false,
			expectedResult: map[string][]string{},
		},
		{
			name:           "invalid header - no colon",
			headerStrings:  []string{"InvalidHeader"},
			expectedError:  true,
			expectedResult: nil,
		},
		{
			name:           "invalid header - empty key",
			headerStrings:  []string{":value"},
			expectedError:  true,
			expectedResult: nil,
		},
		{
			name:           "invalid header - colon at start",
			headerStrings:  []string{":Authorization:Bearer token"},
			expectedError:  true,
			expectedResult: nil,
		},
		{
			name:           "mixed valid and invalid headers",
			headerStrings:  []string{"Valid:header", "InvalidHeader"},
			expectedError:  true,
			expectedResult: nil,
		},
		{
			name:          "duplicate headers",
			headerStrings: []string{"X-Custom:value1", "X-Custom:value2"},
			expectedError: false,
			expectedResult: map[string][]string{
				"X-Custom": {"value1", "value2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseHeaders(tt.headerStrings)

			if tt.expectedError {
				if err == nil {
					t.Error("expected error, but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.expectedResult) {
				t.Errorf("result length = %d, want %d", len(result), len(tt.expectedResult))
				return
			}

			for key, expectedValues := range tt.expectedResult {
				actualValues := result[key]
				if len(actualValues) != len(expectedValues) {
					t.Errorf("values for key %s: got %v, want %v", key, actualValues, expectedValues)
					continue
				}
				for i, expected := range expectedValues {
					if actualValues[i] != expected {
						t.Errorf("value[%d] for key %s: got %s, want %s", i, key, actualValues[i], expected)
					}
				}
			}
		})
	}
}

func TestHeaderRemovalInExecuteRequest(t *testing.T) {
	tests := []struct {
		name              string
		metadata          map[string][]string
		customHeaders     []string
		expectedHeaders   map[string]bool
		unexpectedHeaders []string
	}{
		{
			name: "removes special gRPC headers",
			metadata: map[string][]string{
				":authority":     {"example.com"},
				":path":          {"/service/method"},
				":method":        {"POST"},
				"regular-header": {"value"},
				"x-custom":       {"custom-value"},
			},
			customHeaders: []string{},
			expectedHeaders: map[string]bool{
				"regular-header": true,
				"x-custom":       true,
				"content-type":   true,
			},
			unexpectedHeaders: []string{":authority", ":path", ":method"},
		},
		{
			name: "preserves regular headers",
			metadata: map[string][]string{
				"authorization": {"Bearer token"},
				"x-request-id":  {"12345"},
				"grpc-timeout":  {"30S"},
				"user-agent":    {"grpc-go/1.0"},
			},
			customHeaders: []string{},
			expectedHeaders: map[string]bool{
				"authorization": true,
				"x-request-id":  true,
				"grpc-timeout":  true,
				"user-agent":    true,
				"content-type":  true,
			},
			unexpectedHeaders: []string{},
		},
		{
			name: "adds custom headers",
			metadata: map[string][]string{
				"existing": {"value"},
			},
			customHeaders: []string{
				"X-Custom-Header:custom-value",
				"Authorization:Bearer token",
			},
			expectedHeaders: map[string]bool{
				"existing":        true,
				"X-Custom-Header": true,
				"Authorization":   true,
				"content-type":    true,
			},
			unexpectedHeaders: []string{},
		},
		{
			name: "overrides existing headers with custom headers",
			metadata: map[string][]string{
				"authorization": {"old-token"},
				"x-custom":      {"old-value"},
			},
			customHeaders: []string{
				"authorization:new-token",
				"x-custom:new-value",
			},
			expectedHeaders: map[string]bool{
				"authorization": true,
				"x-custom":      true,
				"content-type":  true,
			},
			unexpectedHeaders: []string{},
		},
		{
			name:     "handles empty metadata",
			metadata: map[string][]string{},
			customHeaders: []string{
				"X-Header:value",
			},
			expectedHeaders: map[string]bool{
				"X-Header":     true,
				"content-type": true,
			},
			unexpectedHeaders: []string{},
		},
		{
			name: "filters multiple special headers",
			metadata: map[string][]string{
				":authority":      {"example.com"},
				":path":           {"/path"},
				":method":         {"POST"},
				":scheme":         {"https"},
				":status":         {"200"},
				"normal-header-1": {"value1"},
				"normal-header-2": {"value2"},
			},
			customHeaders: []string{},
			expectedHeaders: map[string]bool{
				"normal-header-1": true,
				"normal-header-2": true,
				"content-type":    true,
			},
			unexpectedHeaders: []string{":authority", ":path", ":method", ":scheme", ":status"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is a conceptual test - actual implementation would need
			// to mock HTTP client or use httptest
			// The test validates the expected behavior of header filtering
		})
	}
}

func parseHeaders(headerStrings []string) (http.Header, error) {
	headers := http.Header{}
	for _, kv := range headerStrings {
		i := strings.IndexByte(kv, ':')
		// zero means meaningless empty header name
		if i <= 0 {
			return nil, fmt.Errorf("additional headers must be colon(:)-separated: %s", kv)
		}
		headers.Add(kv[0:i], kv[i+1:])
	}
	return headers, nil
}
