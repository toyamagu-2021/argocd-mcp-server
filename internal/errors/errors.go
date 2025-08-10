package errors

import (
	"errors"
	"fmt"
)

// ErrorType represents the type of error
type ErrorType string

const (
	// ErrorTypeValidation indicates a validation error
	ErrorTypeValidation ErrorType = "validation"
	// ErrorTypeAuthentication indicates an authentication error
	ErrorTypeAuthentication ErrorType = "authentication"
	// ErrorTypeCLI indicates a CLI execution error
	ErrorTypeCLI ErrorType = "cli"
	// ErrorTypeParsing indicates a parsing error
	ErrorTypeParsing ErrorType = "parsing"
	// ErrorTypeNotFound indicates a resource not found error
	ErrorTypeNotFound ErrorType = "not_found"
	// ErrorTypeTimeout indicates a timeout error
	ErrorTypeTimeout ErrorType = "timeout"
	// ErrorTypeInternal indicates an internal error
	ErrorTypeInternal ErrorType = "internal"
)

// AppError represents a structured application error
type AppError struct {
	Type    ErrorType
	Message string
	Cause   error
	Details map[string]interface{}
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Cause
}

// Is checks if the error is of the same type
func (e *AppError) Is(target error) bool {
	var appErr *AppError
	if errors.As(target, &appErr) {
		return e.Type == appErr.Type
	}
	return false
}

// NewValidationError creates a new validation error
func NewValidationError(message string, details map[string]interface{}) *AppError {
	return &AppError{
		Type:    ErrorTypeValidation,
		Message: message,
		Details: details,
	}
}

// NewAuthenticationError creates a new authentication error
func NewAuthenticationError(message string, cause error) *AppError {
	return &AppError{
		Type:    ErrorTypeAuthentication,
		Message: message,
		Cause:   cause,
	}
}

// NewCLIError creates a new CLI execution error
func NewCLIError(message string, cause error, details map[string]interface{}) *AppError {
	return &AppError{
		Type:    ErrorTypeCLI,
		Message: message,
		Cause:   cause,
		Details: details,
	}
}

// NewParsingError creates a new parsing error
func NewParsingError(message string, cause error, details map[string]interface{}) *AppError {
	return &AppError{
		Type:    ErrorTypeParsing,
		Message: message,
		Cause:   cause,
		Details: details,
	}
}

// NewNotFoundError creates a new not found error
func NewNotFoundError(message string, details map[string]interface{}) *AppError {
	return &AppError{
		Type:    ErrorTypeNotFound,
		Message: message,
		Details: details,
	}
}

// NewTimeoutError creates a new timeout error
func NewTimeoutError(message string, cause error) *AppError {
	return &AppError{
		Type:    ErrorTypeTimeout,
		Message: message,
		Cause:   cause,
	}
}

// NewInternalError creates a new internal error
func NewInternalError(message string, cause error) *AppError {
	return &AppError{
		Type:    ErrorTypeInternal,
		Message: message,
		Cause:   cause,
	}
}

// IsValidationError checks if the error is a validation error
func IsValidationError(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Type == ErrorTypeValidation
	}
	return false
}

// IsAuthenticationError checks if the error is an authentication error
func IsAuthenticationError(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Type == ErrorTypeAuthentication
	}
	return false
}

// IsCLIError checks if the error is a CLI error
func IsCLIError(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Type == ErrorTypeCLI
	}
	return false
}

// IsNotFoundError checks if the error is a not found error
func IsNotFoundError(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Type == ErrorTypeNotFound
	}
	return false
}

// GetErrorDetails extracts details from an AppError
func GetErrorDetails(err error) map[string]interface{} {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Details
	}
	return nil
}
