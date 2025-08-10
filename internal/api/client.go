package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/errors"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/logging"
)

// Client represents an ArgoCD API client
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	logger     *logrus.Logger
}

// NewClient creates a new ArgoCD API client
func NewClient() (*Client, error) {
	token := os.Getenv("ARGOCD_AUTH_TOKEN")
	server := os.Getenv("ARGOCD_SERVER")

	if token == "" || server == "" {
		return nil, errors.NewAuthenticationError("ARGOCD_AUTH_TOKEN and ARGOCD_SERVER environment variables must be set", nil)
	}

	// Ensure server has https:// prefix
	if server[:4] != "http" {
		server = "https://" + server
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: server + "/api/v1",
		token:   token,
		logger:  logging.GetLogger(),
	}, nil
}

// doRequest performs an HTTP request with authentication
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, errors.NewInternalError("failed to marshal request body", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, errors.NewInternalError("failed to create request", err)
	}

	// Set common headers
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	c.logger.WithFields(map[string]interface{}{
		"method": method,
		"url":    url,
	}).Debug("Making API request")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.NewCLIError("failed to execute request", err, map[string]interface{}{
			"method": method,
			"url":    url,
		})
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.NewInternalError("failed to read response body", err)
	}

	// Check for non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.logger.WithFields(map[string]interface{}{
			"status": resp.StatusCode,
			"body":   string(responseBody),
		}).Error("API request failed")

		// Handle specific error codes
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return nil, errors.NewAuthenticationError("authentication failed", fmt.Errorf("status code: %d", resp.StatusCode))
		case http.StatusNotFound:
			return nil, errors.NewNotFoundError("resource not found", map[string]interface{}{
				"path":   path,
				"status": resp.StatusCode,
			})
		default:
			return nil, errors.NewCLIError("API request failed", fmt.Errorf("status code: %d", resp.StatusCode), map[string]interface{}{
				"status": resp.StatusCode,
				"body":   string(responseBody),
				"path":   path,
			})
		}
	}

	return responseBody, nil
}

// Get performs a GET request
func (c *Client) Get(ctx context.Context, path string) ([]byte, error) {
	return c.doRequest(ctx, http.MethodGet, path, nil)
}

// Post performs a POST request
func (c *Client) Post(ctx context.Context, path string, body interface{}) ([]byte, error) {
	return c.doRequest(ctx, http.MethodPost, path, body)
}

// Delete performs a DELETE request
func (c *Client) Delete(ctx context.Context, path string) ([]byte, error) {
	return c.doRequest(ctx, http.MethodDelete, path, nil)
}

// Put performs a PUT request
func (c *Client) Put(ctx context.Context, path string, body interface{}) ([]byte, error) {
	return c.doRequest(ctx, http.MethodPut, path, body)
}

// Patch performs a PATCH request
func (c *Client) Patch(ctx context.Context, path string, body interface{}) ([]byte, error) {
	return c.doRequest(ctx, http.MethodPatch, path, body)
}
