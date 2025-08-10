package client

import (
	"context"
	"testing"
	"time"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				ServerAddr: "localhost:60080",
				AuthToken:  "test-token",
			},
			wantErr: false,
		},
		{
			name: "missing server address",
			config: Config{
				AuthToken: "test-token",
			},
			wantErr: true,
		},
		{
			name: "missing auth token",
			config: Config{
				ServerAddr: "localhost:60080",
			},
			wantErr: true,
		},
		{
			name:    "empty config",
			config:  Config{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJWTCredentials_GetRequestMetadata(t *testing.T) {
	creds := newJWTCredentials("test-token")

	metadata, err := creds.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatalf("GetRequestMetadata() error = %v", err)
	}

	expected := "Bearer test-token"
	if metadata["authorization"] != expected {
		t.Errorf("GetRequestMetadata() = %v, want %v", metadata["authorization"], expected)
	}
}

func TestJWTCredentials_RequireTransportSecurity(t *testing.T) {
	creds := newJWTCredentials("test-token")

	if creds.RequireTransportSecurity() {
		t.Error("RequireTransportSecurity() = true, want false")
	}
}

func TestConfig_NewHTTPClient(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "default timeout",
			config: Config{
				ServerAddr: "localhost:60080",
				AuthToken:  "test-token",
			},
		},
		{
			name: "custom timeout",
			config: Config{
				ServerAddr: "localhost:60080",
				AuthToken:  "test-token",
				Timeout:    60 * time.Second,
			},
		},
		{
			name: "insecure mode",
			config: Config{
				ServerAddr: "localhost:60080",
				AuthToken:  "test-token",
				Insecure:   true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.config.NewHTTPClient()
			if client == nil {
				t.Error("NewHTTPClient() returned nil")
				return
			}

			expectedTimeout := tt.config.Timeout
			if expectedTimeout == 0 {
				expectedTimeout = 30 * time.Second
			}

			if client.Timeout != expectedTimeout {
				t.Errorf("NewHTTPClient() timeout = %v, want %v", client.Timeout, expectedTimeout)
			}
		})
	}
}
