package argocde2e

import (
	"os"
	"testing"
)

// TestRealArgoCD_Suite runs E2E tests with parallel execution for independent tests
func TestRealArgoCD_Suite(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	// Skip if environment variables are not set
	if os.Getenv("ARGOCD_SERVER") == "" || os.Getenv("ARGOCD_AUTH_TOKEN") == "" {
		t.Skip("Skipping E2E tests: ARGOCD_SERVER or ARGOCD_AUTH_TOKEN not set")
	}

	// Define test configurations
	testConfigs := []struct {
		name       string
		useGRPCWeb bool
		skipCheck  func() bool
	}{
		{
			name:       "gRPC",
			useGRPCWeb: false,
			skipCheck:  func() bool { return false },
		},
		{
			name:       "gRPC-Web",
			useGRPCWeb: true,
			skipCheck:  func() bool { return os.Getenv("ARGOCD_MCP_E2E_GRPC_WEB") == "" },
		},
	}

	for _, config := range testConfigs {
		config := config // capture range variable

		// Skip if this configuration should be skipped
		if config.skipCheck() {
			continue
		}

		t.Run(config.name, func(t *testing.T) {
			// Set environment variable for gRPC-Web if needed
			if config.useGRPCWeb {
				t.Setenv("ARGOCD_GRPC_WEB", "true")
			}
			// Independent tests that can run in parallel
			t.Run("IndependentTests", func(t *testing.T) {
				t.Run("Initialize", func(t *testing.T) {
					t.Parallel()
					testInitialize(t)
				})
				t.Run("ListTools", func(t *testing.T) {
					t.Parallel()
					testListTools(t)
				})
				t.Run("ListApplications", func(t *testing.T) {
					t.Parallel()
					testListApplications(t)
				})
				t.Run("InvalidAppName", func(t *testing.T) {
					t.Parallel()
					testInvalidAppName(t)
				})
				t.Run("WithTimeout", func(t *testing.T) {
					t.Parallel()
					testWithTimeout(t)
				})

				// Project tests (read-only)
				t.Run("ListProjects", func(t *testing.T) {
					t.Parallel()
					testListProjects(t)
				})
				t.Run("GetProject", func(t *testing.T) {
					t.Parallel()
					testGetProject(t)
				})
				t.Run("ListClusters", func(t *testing.T) {
					t.Parallel()
					testListClusters(t)
				})
				t.Run("GetCluster", func(t *testing.T) {
					t.Parallel()
					testGetCluster(t)
				})
				t.Run("GetClusterNotFound", func(t *testing.T) {
					t.Parallel()
					testGetClusterNotFound(t)
				})
				t.Run("InvalidProjectName", func(t *testing.T) {
					t.Parallel()
					testInvalidProjectName(t)
				})

				// Repository tests (read-only)
				t.Run("ListRepository", func(t *testing.T) {
					t.Parallel()
					testListRepository(t)
				})
				t.Run("GetRepository", func(t *testing.T) {
					t.Parallel()
					testGetRepository(t)
				})

				// Session tests (read-only)
				t.Run("GetUserInfo", func(t *testing.T) {
					t.Parallel()
					testGetUserInfo(t)
				})

				// ApplicationSet tests (read-only)
				t.Run("ListApplicationSets", func(t *testing.T) {
					t.Parallel()
					testListApplicationSets(t)
				})
				t.Run("ListApplicationSetsWithProject", func(t *testing.T) {
					t.Parallel()
					testListApplicationSetsWithProject(t)
				})
				t.Run("GetApplicationSet", func(t *testing.T) {
					t.Parallel()
					testGetApplicationSet(t)
				})
				t.Run("GetApplicationSetMissingName", func(t *testing.T) {
					t.Parallel()
					testGetApplicationSetMissingName(t)
				})
				t.Run("DeleteApplicationSetMissingName", func(t *testing.T) {
					t.Parallel()
					testDeleteApplicationSetMissingName(t)
				})
			})

			// Tests that require existing application
			if appName := os.Getenv("TEST_APP_NAME"); appName != "" {
				t.Run("ExistingAppTests", func(t *testing.T) {
					t.Run("GetExistingApplication", func(t *testing.T) {
						t.Parallel()
						testGetExistingApplication(t)
					})
					t.Run("SyncExistingApplication_DryRun", func(t *testing.T) {
						t.Parallel()
						testSyncExistingApplicationDryRun(t)
					})
					t.Run("TerminateOperation", func(t *testing.T) {
						t.Parallel()
						if config.useGRPCWeb {
							testTerminateOperationGRPCWeb(t)
						} else {
							testTerminateOperation(t)
						}
					})
				})
			}

			// Lifecycle tests that must run sequentially
			t.Run("SequentialTests", func(t *testing.T) {
				// Create project test (must run before lifecycle tests)
				t.Run("CreateProject", testCreateProject)

				// ApplicationSet lifecycle tests (must run in order)
				t.Run("ApplicationSetLifecycle", func(t *testing.T) {
					// These subtests will run sequentially in order
					t.Run("01_CreateApplicationSet", testApplicationSetLifecycle01Create)
					t.Run("02_ListApplicationSets", testApplicationSetLifecycle02List)
					t.Run("03_GetApplicationSet", testApplicationSetLifecycle03Get)
					t.Run("04_SyncGeneratedApp", testApplicationSetLifecycle04SyncGeneratedApp)
					t.Run("05_DeleteApplicationSet", testApplicationSetLifecycle05Delete)
				})

				// Application lifecycle tests (must run in order)
				t.Run("ApplicationLifecycle", func(t *testing.T) {
					// These subtests will run sequentially in order
					t.Run("01_CreateApplication", testCreateApplication)
					t.Run("02_GetCreatedApplication", testGetCreatedApplication)
					t.Run("03_ListApplicationsWithCreated", testListApplicationsWithCreated)
					t.Run("04_RefreshCreatedApplication", testRefreshCreatedApplication)
					t.Run("05_SyncCreatedApplication", testSyncCreatedApplication)
					t.Run("06_DeleteCreatedApplication", testDeleteCreatedApplication)
				})

				// ApplicationSet CRUD tests (create and delete operations)
				t.Run("ApplicationSetCRUD", func(t *testing.T) {
					t.Run("CreateApplicationSet", testCreateApplicationSet)
					t.Run("DeleteApplicationSet", testDeleteApplicationSet)
					t.Run("DeleteApplicationSetWithNamespace", testDeleteApplicationSetWithNamespace)
				})
			})
		}) // End of test configuration
	} // End of for loop
}
