package tools

import (
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// Deps holds the dependencies shared by every tool handler.
//
// The Client is owned by the process (created once in main and closed on
// shutdown). Handlers must not close it.
type Deps struct {
	Client client.Interface
}
