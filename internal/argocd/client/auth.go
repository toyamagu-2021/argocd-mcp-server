package client

import (
	"context"

	"google.golang.org/grpc/credentials"
)

type jwtCredentials struct {
	token string
}

func newJWTCredentials(token string) credentials.PerRPCCredentials {
	return &jwtCredentials{token: token}
}

func (c *jwtCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "Bearer " + c.token,
	}, nil
}

func (c *jwtCredentials) RequireTransportSecurity() bool {
	return false
}
