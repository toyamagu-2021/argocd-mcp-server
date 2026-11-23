# Application resource

`get_application_resource` returns the live manifest of one Kubernetes object managed by an Application. The required arguments are `name`, `resource_name`, `kind`, and `version`. `group` is empty for core resources. `version` is the Kubernetes API version, such as `v1`, not a git revision. The body is compact JSON. `metadata.managedFields` and `kubectl.kubernetes.io/last-applied-configuration` are removed.

## Sub-features

- `get-deployment` loads Deployment `guestbook-ui` from the guestbook example.
- `manifest-matches-kubectl` compares that name with `default-workloads.txt`.

## How to get to it (user POV)

- An MCP client calls `get_application_resource` after the Application has synced and the object exists in the destination namespace.
- For the guestbook example the client sets `resource_name` to `guestbook-ui`, `resource_namespace` to `default`, `group` to `apps`, `kind` to `Deployment`, and `version` to `v1`.

## Driving it with verify-argocd-mcp-server

Preconditions:

- `scripts/doctor.sh` prints `DOCTOR_OK`.
- Application `verify-guestbook` has completed a sync, as in [sync application](sync-application.md). The sync RPC returns when the operation is accepted. The live Deployment can appear a few seconds later. Repeat this call until `is_error` is false.

- **Get the Deployment.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh call --feature get-application-resource --label guestbook-ui get_application_resource '{"name":"verify-guestbook","resource_name":"guestbook-ui","resource_namespace":"default","group":"apps","kind":"Deployment","version":"v1"}'`. The call has `is_error` false and `rpc_error` null. The text includes `guestbook-ui` and `Deployment`.
- **Cluster view.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/evidence.sh --feature get-application-resource`. `default-workloads.txt` includes `guestbook-ui` under Deployments.

## Gotchas

- Missing `name`, `resource_name`, `kind`, or `version` is a tool error (`name is required` and the same shape for the others). That proves the handler ran. It does not prove the object exists.
- `resource_namespace` is required when the same name exists in more than one namespace. This recipe sets it.
- Argo CD replaces Secret data before returning a Secret manifest. The guestbook Deployment is not a Secret, so that replacement is not proven.
- This tool does not strip `managedFields`. That stripping belongs to [managed resources](get-application-managed-resources.md).
