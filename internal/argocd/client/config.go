package client

import (
	"crypto/tls"
	"net/http"
	"time"
)

// Config holds configuration parameters for the ArgoCD gRPC client
type Config struct {
	ServerAddr        string
	AuthToken         string
	PlainText         bool
	Insecure          bool
	GRPCWeb           bool
	GRPCWebRootPath   string
	CertFile          string
	ClientCertFile    string
	ClientCertKeyFile string
	Headers           []string
	UserAgent         string
	Timeout           time.Duration
}

// NewHTTPClient creates a new HTTP client with TLS configuration for gRPC-Web proxy
func (c *Config) NewHTTPClient() *http.Client {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: c.Insecure,
	}

	if c.ClientCertFile != "" && c.ClientCertKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(c.ClientCertFile, c.ClientCertKeyFile)
		if err == nil {
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
	}

	timeout := c.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
}

// Validate checks that required configuration parameters are present
func (c *Config) Validate() error {
	if c.ServerAddr == "" {
		return ErrServerAddrRequired
	}
	if c.AuthToken == "" {
		return ErrAuthTokenRequired
	}
	return nil
}
