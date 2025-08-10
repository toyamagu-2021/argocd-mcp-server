package errors

import (
	"errors"
	"testing"
)

func TestAppError(t *testing.T) {
	tests := []struct {
		name    string
		err     *AppError
		wantMsg string
	}{
		{
			name:    "validation error without cause",
			err:     NewValidationError("invalid input", nil),
			wantMsg: "[validation] invalid input",
		},
		{
			name:    "authentication error with cause",
			err:     NewAuthenticationError("auth failed", errors.New("token expired")),
			wantMsg: "[authentication] auth failed: token expired",
		},
		{
			name: "CLI error with details",
			err: NewCLIError("command failed", errors.New("exit code 1"), map[string]interface{}{
				"command": "app list",
				"stderr":  "connection refused",
			}),
			wantMsg: "[cli] command failed: exit code 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("AppError.Error() = %v, want %v", got, tt.wantMsg)
			}
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := NewCLIError("wrapper", cause, nil)

	if got := err.Unwrap(); got != cause {
		t.Errorf("AppError.Unwrap() = %v, want %v", got, cause)
	}
}

func TestAppError_Is(t *testing.T) {
	err1 := NewValidationError("error1", nil)
	err2 := NewValidationError("error2", nil)
	err3 := NewCLIError("error3", nil, nil)

	if !err1.Is(err2) {
		t.Error("Two validation errors should match")
	}

	if err1.Is(err3) {
		t.Error("Validation error should not match CLI error")
	}
}

func TestErrorTypeHelpers(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		checkFunc func(error) bool
		want      bool
	}{
		{
			name:      "IsValidationError with validation error",
			err:       NewValidationError("test", nil),
			checkFunc: IsValidationError,
			want:      true,
		},
		{
			name:      "IsValidationError with non-validation error",
			err:       NewCLIError("test", nil, nil),
			checkFunc: IsValidationError,
			want:      false,
		},
		{
			name:      "IsAuthenticationError with auth error",
			err:       NewAuthenticationError("test", nil),
			checkFunc: IsAuthenticationError,
			want:      true,
		},
		{
			name:      "IsCLIError with CLI error",
			err:       NewCLIError("test", nil, nil),
			checkFunc: IsCLIError,
			want:      true,
		},
		{
			name:      "IsNotFoundError with not found error",
			err:       NewNotFoundError("test", nil),
			checkFunc: IsNotFoundError,
			want:      true,
		},
		{
			name:      "Helper with standard error",
			err:       errors.New("standard error"),
			checkFunc: IsValidationError,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.checkFunc(tt.err); got != tt.want {
				t.Errorf("Error type check = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetErrorDetails(t *testing.T) {
	details := map[string]interface{}{
		"field": "username",
		"value": "test",
	}

	err := NewValidationError("invalid", details)
	got := GetErrorDetails(err)

	if got["field"] != "username" || got["value"] != "test" {
		t.Errorf("GetErrorDetails() = %v, want %v", got, details)
	}

	// Test with non-AppError
	standardErr := errors.New("standard error")
	if got := GetErrorDetails(standardErr); got != nil {
		t.Errorf("GetErrorDetails(standardError) = %v, want nil", got)
	}
}

func TestAllErrorTypes(t *testing.T) {
	// Test all error constructors
	errors := []struct {
		name string
		err  *AppError
		typ  ErrorType
	}{
		{"validation", NewValidationError("test", nil), ErrorTypeValidation},
		{"authentication", NewAuthenticationError("test", nil), ErrorTypeAuthentication},
		{"cli", NewCLIError("test", nil, nil), ErrorTypeCLI},
		{"parsing", NewParsingError("test", nil, nil), ErrorTypeParsing},
		{"not found", NewNotFoundError("test", nil), ErrorTypeNotFound},
		{"timeout", NewTimeoutError("test", nil), ErrorTypeTimeout},
		{"internal", NewInternalError("test", nil), ErrorTypeInternal},
	}

	for _, e := range errors {
		t.Run(e.name, func(t *testing.T) {
			if e.err.Type != e.typ {
				t.Errorf("Error type = %v, want %v", e.err.Type, e.typ)
			}
			if e.err.Message != "test" {
				t.Errorf("Error message = %v, want 'test'", e.err.Message)
			}
		})
	}
}
