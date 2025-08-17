# ArgoCD MCP Server - Tools Implementation Checklist

## ✅ Completed Tools

### Applications
- [x] list_application - Lists ArgoCD applications with filtering options
- [x] get_application - Retrieves detailed application information  
- [x] get_application_manifests - Gets rendered manifests for an application
- [x] get_application_events - Gets Kubernetes events for resources
- [x] get_application_resource_tree - Gets resource hierarchy
- [x] create_application - Creates a new ArgoCD application
- [x] sync_application - Triggers application sync with prune/dry-run options
- [x] refresh_application - Refreshes application without syncing
- [x] delete_application - Deletes applications with cascade control

### Projects
- [x] list_project - Lists all ArgoCD projects
- [x] get_project - Retrieves detailed project information by name
- [x] create_project - Creates new ArgoCD project with access controls

### ApplicationSets
- [x] list_applicationset - Lists ArgoCD ApplicationSets with filtering options
- [x] get_applicationset - Retrieves detailed ApplicationSet information
- [x] create_applicationset - Creates a new ApplicationSet with generators and template

### Clusters
- [x] list_cluster - Lists managed Kubernetes clusters
- [x] get_cluster - Gets cluster details and connection info

### Repositories
- [x] list_repository - Lists configured Git repositories
- [x] get_repository - Gets repository details and connection status

## 📋 TODO - Priority 1 (Core Functionality)

### Applications (Extended)
- [ ] update_application - Updates existing application configuration
- [ ] patch_application - Partial updates to application spec
- [ ] rollback_application - Rollbacks to previous sync state
- [ ] terminate_operation - Terminates running sync/refresh operations
- [ ] get_application_logs - Gets logs for application resources

### Projects (Complete)
- [ ] update_project - Updates existing project
- [ ] delete_project - Deletes a project
- [ ] patch_project - Partial updates to project spec

### ApplicationSets (Extended)
- [ ] update_applicationset - Updates existing ApplicationSet configuration
- [ ] delete_applicationset - Deletes an ApplicationSet

## 📋 TODO - Priority 2 (Essential Management)

### Repositories (Extended)
- [ ] create_repository - Adds new repository connection
- [ ] update_repository - Updates repository configuration
- [ ] delete_repository - Removes repository connection
- [ ] validate_repository - Tests repository connection

### Clusters (Extended)
- [ ] create_cluster - Registers new cluster
- [ ] update_cluster - Updates cluster configuration
- [ ] delete_cluster - Removes cluster registration
- [ ] get_cluster_info - Gets server version and capabilities

## 📋 TODO - Priority 3 (Advanced Features)

### Certificates
- [ ] list_certificates - Lists TLS certificates
- [ ] create_certificate - Adds new certificate
- [ ] delete_certificate - Removes certificate

### Settings
- [ ] get_settings - Gets ArgoCD server settings
- [ ] update_settings - Updates server settings
- [ ] get_resource_overrides - Gets resource behavior customizations

### RBAC & Accounts
- [ ] list_accounts - Lists user accounts
- [ ] get_account - Gets account details
- [ ] create_token - Generates authentication token
- [ ] revoke_token - Revokes authentication token
- [ ] get_rbac_policies - Lists RBAC policies
- [ ] validate_rbac - Tests RBAC permissions

### GPG Keys
- [ ] list_gpg_keys - Lists configured GPG keys
- [ ] create_gpg_key - Adds GPG key for commit verification
- [ ] delete_gpg_key - Removes GPG key

## 📋 TODO - Priority 4 (Monitoring & Observability)

### Metrics & Health
- [ ] get_metrics - Gets Prometheus metrics
- [ ] get_health - Gets health status
- [ ] get_version - Gets ArgoCD version info

### Notifications
- [ ] list_notifications - Lists notification configurations
- [ ] test_notification - Tests notification delivery

## Implementation Notes

1. **Current Status**: Project tools (list_projects, get_project) are created but not yet integrated into tools.go
2. **Testing**: Each tool should have comprehensive unit tests
3. **Error Handling**: Use the custom error types from internal/errors/
4. **Authentication**: All tools use the gRPC client with JWT token authentication
5. **Safety**: Include dry-run options where applicable (sync, delete operations)
6. **Response Format**: Follow MCP protocol for structured responses

## Next Steps

1. Implement remaining ApplicationSet CRUD operations (create, update, delete)
2. Complete remaining project operations (update, delete, patch)
3. Move to application extended operations (update, patch, refresh, rollback)
4. Implement repository management tools
5. Complete cluster management operations