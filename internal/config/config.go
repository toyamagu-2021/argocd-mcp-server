// Package config loads process configuration from environment variables.
//
// It is the only place that reads connection-related environment variables.
// Tool handlers and the Argo CD client receive already-parsed values.
package config

import (
	"errors"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// Environment variable names.
const (
	EnvServer          = "ARGOCD_SERVER"
	EnvAuthToken       = "ARGOCD_AUTH_TOKEN"
	EnvInsecure        = "ARGOCD_INSECURE"
	EnvPlainText       = "ARGOCD_PLAINTEXT"
	EnvGRPCWeb         = "ARGOCD_GRPC_WEB"
	EnvGRPCWebRootPath = "ARGOCD_GRPC_WEB_ROOT_PATH"
	EnvGRPCMaxSizeMB   = "ARGOCD_GRPC_MAX_SIZE_MB"
)

// DefaultGRPCMaxSizeMB is the default gRPC message size limit in MiB.
const DefaultGRPCMaxSizeMB = 200

// ErrServerRequired is returned when ARGOCD_SERVER is not set.
var ErrServerRequired = errors.New("ARGOCD_SERVER environment variable is not set. Please set it using: export ARGOCD_SERVER=your-argocd-server.com")

// ErrAuthTokenRequired is returned when ARGOCD_AUTH_TOKEN is not set.
var ErrAuthTokenRequired = errors.New("ARGOCD_AUTH_TOKEN environment variable is not set. Please set it using: export ARGOCD_AUTH_TOKEN=$(argocd account generate-token)")

// Config is the process configuration.
type Config struct {
	Server             string
	AuthToken          string
	Insecure           bool
	PlainText          bool
	GRPCWeb            bool
	GRPCWebRootPath    string
	MaxGRPCMessageSize int
}

// Load reads the configuration from the environment and validates it.
func Load() (*Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (*Config, error) {
	cfg := &Config{
		Server:          getenv(EnvServer),
		AuthToken:       getenv(EnvAuthToken),
		Insecure:        parseBool(getenv(EnvInsecure)),
		PlainText:       parseBool(getenv(EnvPlainText)),
		GRPCWeb:         parseBool(getenv(EnvGRPCWeb)),
		GRPCWebRootPath: getenv(EnvGRPCWebRootPath),
	}

	cfg.MaxGRPCMessageSize = parseIntClamped(getenv(EnvGRPCMaxSizeMB), DefaultGRPCMaxSizeMB, 1, math.MaxInt32/(1024*1024)) * 1024 * 1024

	if cfg.AuthToken == "" {
		return nil, ErrAuthTokenRequired
	}
	if cfg.Server == "" {
		return nil, ErrServerRequired
	}
	return cfg, nil
}

// ToClientConfig converts the process configuration into an Argo CD client configuration.
func (c *Config) ToClientConfig() *client.Config {
	return &client.Config{
		ServerAddr:         c.Server,
		AuthToken:          c.AuthToken,
		Insecure:           c.Insecure,
		PlainText:          c.PlainText,
		GRPCWeb:            c.GRPCWeb,
		GRPCWebRootPath:    c.GRPCWebRootPath,
		MaxGRPCMessageSize: c.MaxGRPCMessageSize,
	}
}

// parseBool keeps the historical behaviour: only the literal "true" enables a flag.
func parseBool(v string) bool {
	return strings.TrimSpace(v) == "true"
}

// parseIntClamped keeps the historical lenient behaviour of ARGOCD_GRPC_MAX_SIZE_MB:
// empty or unparsable values fall back to def, out-of-range values are clamped.
func parseIntClamped(v string, def, minVal, maxVal int) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	if n < minVal {
		return minVal
	}
	if n > maxVal {
		return maxVal
	}
	return n
}
