package tools

import (
	"context"
	"encoding/json"
	"testing"

	sessionpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/session"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
)

// Test the tool handler with environment variables
func TestHandleGetUserInfo(t *testing.T) {
	tests := []struct {
		name          string
		request       mcp.CallToolRequest
		envVars       map[string]string
		wantError     bool
		errorContains string
	}{
		{
			name: "missing environment variables",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "get_user_info",
					Arguments: map[string]interface{}{},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "",
				"ARGOCD_SERVER":     "",
			},
			wantError:     true,
			errorContains: "server address is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Execute handler
			result, err := HandleGetUserInfo(context.Background(), tt.request)

			// Check expectations
			if tt.wantError {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
				if tt.errorContains != "" && len(result.Content) > 0 {
					content, ok := mcp.AsTextContent(result.Content[0])
					require.True(t, ok)
					assert.Contains(t, content.Text, tt.errorContains)
				}
			} else {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.False(t, result.IsError)
			}
		})
	}
}

// Test the tool schema
func TestGetUserInfoTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if GetUserInfoTool.Name != "get_user_info" {
		t.Errorf("Expected tool name 'get_user_info', got %s", GetUserInfoTool.Name)
	}

	// Verify tool has description
	if GetUserInfoTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Check input schema exists
	if GetUserInfoTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", GetUserInfoTool.InputSchema.Type)
	}

	// get_user_info has no required parameters
	if len(GetUserInfoTool.InputSchema.Required) != 0 {
		t.Errorf("Expected no required parameters, got %d", len(GetUserInfoTool.InputSchema.Required))
	}
}

// Test the handler logic with mocked client
func TestGetUserInfoHandler(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(*mock.MockInterface)
		wantError   bool
		wantMessage string
	}{
		{
			name: "successful operation - logged in user",
			setupMock: func(m *mock.MockInterface) {
				userInfo := &sessionpkg.GetUserInfoResponse{
					LoggedIn: true,
					Username: "testuser",
					Iss:      "argocd",
					Groups:   []string{"admin", "developers"},
				}
				m.EXPECT().GetUserInfo(gomock.Any()).Return(userInfo, nil)
			},
			wantError:   false,
			wantMessage: "testuser",
		},
		{
			name: "successful operation - not logged in",
			setupMock: func(m *mock.MockInterface) {
				userInfo := &sessionpkg.GetUserInfoResponse{
					LoggedIn: false,
					Username: "",
					Iss:      "",
					Groups:   nil,
				}
				m.EXPECT().GetUserInfo(gomock.Any()).Return(userInfo, nil)
			},
			wantError:   false,
			wantMessage: "false",
		},
		{
			name: "operation fails",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().GetUserInfo(gomock.Any()).Return(nil, assert.AnError)
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockInterface(ctrl)
			tt.setupMock(mockClient)

			result, err := getUserInfoHandler(context.Background(), mockClient)

			if tt.wantError {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
			} else {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.False(t, result.IsError)

				if tt.wantMessage != "" {
					require.Len(t, result.Content, 1)
					textContent, ok := mcp.AsTextContent(result.Content[0])
					require.True(t, ok)
					assert.Contains(t, textContent.Text, tt.wantMessage)

					// Verify JSON structure
					var userInfo UserInfo
					err := json.Unmarshal([]byte(textContent.Text), &userInfo)
					require.NoError(t, err)
				}
			}
		})
	}
}
