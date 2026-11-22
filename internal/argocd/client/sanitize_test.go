package client

import (
	"encoding/json"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const secret = "s3cr3t-value"

func rawCluster() *v1alpha1.Cluster {
	return &v1alpha1.Cluster{
		Server: "https://kubernetes.default.svc",
		Name:   "in-cluster",
		Config: v1alpha1.ClusterConfig{
			Username:    secret,
			Password:    secret,
			BearerToken: secret,
			TLSClientConfig: v1alpha1.TLSClientConfig{
				Insecure:   true,
				ServerName: "kube",
				CertData:   []byte(secret),
				KeyData:    []byte(secret),
				CAData:     []byte(secret),
			},
			ExecProviderConfig: &v1alpha1.ExecProviderConfig{
				Command: "aws",
				Args:    []string{"--token=" + secret},
				Env:     map[string]string{"PASSWORD": secret},
			},
		},
		Info: v1alpha1.ClusterInfo{ServerVersion: "1.37"},
		//lint:ignore SA1019 deprecated field still populated by the Argo CD server for backward compatibility
		ServerVersion: "1.37",
		//lint:ignore SA1019 deprecated field still populated by the Argo CD server for backward compatibility
		ConnectionState: v1alpha1.ConnectionState{Status: v1alpha1.ConnectionStatusSuccessful},
	}
}

func rawRepo() *v1alpha1.Repository {
	return &v1alpha1.Repository{
		Repo:                    "https://github.com/example/repo",
		Username:                "git",
		Password:                secret,
		SSHPrivateKey:           secret,
		TLSClientCertData:       secret,
		TLSClientCertKey:        secret,
		GithubAppPrivateKey:     secret,
		GCPServiceAccountKey:    secret,
		BearerToken:             secret,
		GithubAppId:             42,
		GithubAppInstallationId: 7,
		ConnectionState:         v1alpha1.ConnectionState{Status: v1alpha1.ConnectionStatusSuccessful},
	}
}

func assertNoSecret(t *testing.T, v interface{}) {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	assert.NotContains(t, string(b), secret)
	assert.NotContains(t, string(b), "czNjcjN0") // base64 of the secret prefix
}

func TestSanitizeCluster(t *testing.T) {
	in := rawCluster()
	out := sanitizeCluster(in)
	assertNoSecret(t, out)
	assert.Equal(t, in.Server, out.Server)
	assert.Equal(t, in.Name, out.Name)
	assert.True(t, out.Config.Insecure)
	assert.Equal(t, "kube", out.Config.ServerName)
	assert.Equal(t, "1.37", out.Info.ServerVersion)
	//lint:ignore SA1019 deprecated field still populated by the Argo CD server for backward compatibility
	assert.Equal(t, "1.37", out.ServerVersion)
	//lint:ignore SA1019 deprecated field still populated by the Argo CD server for backward compatibility
	assert.Equal(t, v1alpha1.ConnectionStatusSuccessful, out.ConnectionState.Status)
	// input is not mutated
	assert.Equal(t, secret, in.Config.BearerToken)
	assert.Nil(t, sanitizeCluster(nil))
}

func TestSanitizeClusterList(t *testing.T) {
	l := &v1alpha1.ClusterList{Items: []v1alpha1.Cluster{*rawCluster(), *rawCluster()}}
	out := sanitizeClusterList(l)
	require.Len(t, out.Items, 2)
	assertNoSecret(t, out)
	assert.Nil(t, sanitizeClusterList(nil))
}

func TestSanitizeRepository(t *testing.T) {
	in := rawRepo()
	out := sanitizeRepository(in)
	assertNoSecret(t, out)
	assert.Equal(t, in.Repo, out.Repo)
	assert.Equal(t, "git", out.Username)
	assert.Equal(t, int64(42), out.GithubAppId)
	assert.Equal(t, v1alpha1.ConnectionStatusSuccessful, out.ConnectionState.Status)
	assert.Equal(t, secret, in.Password)
	assert.Nil(t, sanitizeRepository(nil))
}

func TestSanitizeRepositoryList(t *testing.T) {
	l := &v1alpha1.RepositoryList{Items: v1alpha1.Repositories{rawRepo(), rawRepo()}}
	out := sanitizeRepositoryList(l)
	require.Len(t, out.Items, 2)
	assertNoSecret(t, out)
	assert.Nil(t, sanitizeRepositoryList(nil))
}
