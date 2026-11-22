package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr error
		check   func(t *testing.T, c *Config)
	}{
		{
			name:    "missing token",
			env:     map[string]string{EnvServer: "argocd.example.com:443"},
			wantErr: ErrAuthTokenRequired,
		},
		{
			name:    "missing server",
			env:     map[string]string{EnvAuthToken: "tok"},
			wantErr: ErrServerRequired,
		},
		{
			name: "defaults",
			env:  map[string]string{EnvServer: "s:443", EnvAuthToken: "tok"},
			check: func(t *testing.T, c *Config) {
				assert.Equal(t, "s:443", c.Server)
				assert.Equal(t, "tok", c.AuthToken)
				assert.False(t, c.Insecure)
				assert.False(t, c.PlainText)
				assert.False(t, c.GRPCWeb)
				assert.Equal(t, DefaultGRPCMaxSizeMB*1024*1024, c.MaxGRPCMessageSize)
			},
		},
		{
			name: "all flags",
			env: map[string]string{
				EnvServer: "s:443", EnvAuthToken: "tok",
				EnvInsecure: "true", EnvPlainText: "true", EnvGRPCWeb: "true",
				EnvGRPCWebRootPath: "/argo", EnvGRPCMaxSizeMB: "16",
			},
			check: func(t *testing.T, c *Config) {
				assert.True(t, c.Insecure)
				assert.True(t, c.PlainText)
				assert.True(t, c.GRPCWeb)
				assert.Equal(t, "/argo", c.GRPCWebRootPath)
				assert.Equal(t, 16*1024*1024, c.MaxGRPCMessageSize)
			},
		},
		{
			name: "invalid max size falls back to default",
			env:  map[string]string{EnvServer: "s", EnvAuthToken: "t", EnvGRPCMaxSizeMB: "abc"},
			check: func(t *testing.T, c *Config) {
				assert.Equal(t, DefaultGRPCMaxSizeMB*1024*1024, c.MaxGRPCMessageSize)
			},
		},
		{
			name: "too small max size is clamped",
			env:  map[string]string{EnvServer: "s", EnvAuthToken: "t", EnvGRPCMaxSizeMB: "0"},
			check: func(t *testing.T, c *Config) {
				assert.Equal(t, 1*1024*1024, c.MaxGRPCMessageSize)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := load(envFrom(tt.env))
			switch {
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
			default:
				require.NoError(t, err)
				tt.check(t, c)
			}
		})
	}
}

func TestToClientConfig(t *testing.T) {
	c := &Config{
		Server: "s:443", AuthToken: "tok", Insecure: true, PlainText: true,
		GRPCWeb: true, GRPCWebRootPath: "/p", MaxGRPCMessageSize: 42,
	}
	cc := c.ToClientConfig()
	assert.Equal(t, "s:443", cc.ServerAddr)
	assert.Equal(t, "tok", cc.AuthToken)
	assert.True(t, cc.Insecure)
	assert.True(t, cc.PlainText)
	assert.True(t, cc.GRPCWeb)
	assert.Equal(t, "/p", cc.GRPCWebRootPath)
	assert.Equal(t, 42, cc.MaxGRPCMessageSize)
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv(EnvServer, "s:443")
	t.Setenv(EnvAuthToken, "tok")
	c, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "s:443", c.Server)
}
