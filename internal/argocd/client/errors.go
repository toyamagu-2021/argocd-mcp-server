package client

import "errors"

var (
	// ErrServerAddrRequired is returned when no server address is provided in configuration
	ErrServerAddrRequired = errors.New("server address is required")
	// ErrAuthTokenRequired is returned when no authentication token is provided in configuration
	ErrAuthTokenRequired = errors.New("auth token is required")
)
