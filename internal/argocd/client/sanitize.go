package client

import (
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// Defence in depth (design D6/D13): the Argo CD API server already strips
// credentials from Cluster and Repository responses, but this client must
// never hand raw secrets to the MCP layer even if a server (or proxy in
// between) misbehaves. We delegate to the upstream Sanitized() methods
// instead of maintaining our own deny-list, and only carry over the
// non-secret fields the server fills in after sanitizing.

// sanitizeCluster returns a copy of c with credentials removed.
func sanitizeCluster(c *v1alpha1.Cluster) *v1alpha1.Cluster {
	if c == nil {
		return nil
	}
	s := c.Sanitized()
	// Populated by the server after Sanitized() (server/cluster toAPIResponse).
	//lint:ignore SA1019 deprecated field still populated by the Argo CD server for backward compatibility
	s.ServerVersion = c.ServerVersion
	//lint:ignore SA1019 deprecated field still populated by the Argo CD server for backward compatibility
	s.ConnectionState = c.ConnectionState
	return s
}

// sanitizeClusterList sanitizes every item of l in place and returns l.
func sanitizeClusterList(l *v1alpha1.ClusterList) *v1alpha1.ClusterList {
	if l == nil {
		return nil
	}
	for i := range l.Items {
		l.Items[i] = *sanitizeCluster(&l.Items[i])
	}
	return l
}

// sanitizeRepository returns a copy of r with credentials removed.
func sanitizeRepository(r *v1alpha1.Repository) *v1alpha1.Repository {
	if r == nil {
		return nil
	}
	s := r.Sanitized()
	// Populated by the server after Sanitized() (server/repository List/Get).
	s.ConnectionState = r.ConnectionState
	return s
}

// sanitizeRepositoryList sanitizes every item of l in place and returns l.
func sanitizeRepositoryList(l *v1alpha1.RepositoryList) *v1alpha1.RepositoryList {
	if l == nil {
		return nil
	}
	for i, r := range l.Items {
		l.Items[i] = sanitizeRepository(r)
	}
	return l
}
