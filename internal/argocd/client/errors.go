package client

import "errors"

var (
	ErrServerAddrRequired = errors.New("server address is required")
	ErrAuthTokenRequired  = errors.New("auth token is required")
	ErrConnectionFailed   = errors.New("failed to connect to ArgoCD server")
	ErrNotImplemented     = errors.New("feature not implemented")
)
