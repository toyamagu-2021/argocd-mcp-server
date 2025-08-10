package argocd

// Application represents ArgoCD application structure
type Application struct {
	Metadata Metadata `json:"metadata"`
	Spec     Spec     `json:"spec"`
	Status   Status   `json:"status"`
}

// Metadata holds application metadata
type Metadata struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// Spec defines the desired state of the application
type Spec struct {
	Source      Source      `json:"source"`
	Destination Destination `json:"destination"`
	Project     string      `json:"project"`
}

// Source holds repository information where application manifests are located
type Source struct {
	RepoURL        string `json:"repoURL"`
	Path           string `json:"path"`
	TargetRevision string `json:"targetRevision"`
}

// Destination defines the cluster and namespace where the application is deployed
type Destination struct {
	Server    string `json:"server"`
	Namespace string `json:"namespace"`
	Name      string `json:"name,omitempty"`
}

// Status represents the current state of the application
type Status struct {
	Sync   SyncStatus   `json:"sync"`
	Health HealthStatus `json:"health"`
}

// SyncStatus holds the sync state of the application
type SyncStatus struct {
	Status string `json:"status"`
}

// HealthStatus holds the health state of the application
type HealthStatus struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}
