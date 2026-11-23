# Update application

`update_application` replaces the spec of an existing Application. The server loads the Application and writes only `spec`, so metadata and status are left as they are. The `spec` object replaces the whole spec: omitted fields are cleared. `spec.project` is required, and `spec.source.repoURL` or a `spec.sources` entry with `repoURL` is required. `validate` defaults to true.

## Sub-features

- `set-history-limit` replaces the guestbook spec and sets `revisionHistoryLimit` to 10.
- `spec-matches-kubectl` compares that value with `application-specs.txt`.

## How to get to it (user POV)

- An MCP client reads the Application, then calls `update_application` with `name` and the full `spec` it wants to keep.
- The tool returns the spec JSON, not the Application metadata. The saved spec is what `kubectl` shows, and what `get_application` shows when `detailed` is true.

## Driving it with verify-argocd-mcp-server

Preconditions:

- `scripts/doctor.sh` prints `DOCTOR_OK`.
- Application `verify-guestbook` already exists, as created in [sync application](sync-application.md). Do not enable auto-sync in the replacement spec. [Rollback](rollback-application.md) is refused when auto-sync is on.

- **Replace the spec.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh call --feature update-application --label history-limit update_application '{"name":"verify-guestbook","spec":{"project":"default","source":{"repoURL":"https://github.com/argoproj/argocd-example-apps.git","path":"guestbook","targetRevision":"HEAD"},"destination":{"server":"https://kubernetes.default.svc","namespace":"default"},"revisionHistoryLimit":10}}'`. The call has `is_error` false and `rpc_error` null. The text includes `"revisionHistoryLimit": 10` and `guestbook`.
- **Cluster view.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/evidence.sh --feature update-application`. The `verify-guestbook` row in `application-specs.txt` has `revisionHistoryLimit` `10`, repo URL `https://github.com/argoproj/argocd-example-apps.git`, path `guestbook`, and destination namespace `default`.

## Gotchas

- Sending a partial spec clears every field that was left out, including the source. Build the spec from `get_application` with `detailed` true, or from the known guestbook source above. The default `get_application` summary has no spec.
- `validate` false skips Argo CD's spec validation. This recipe leaves the default, true.
- `project` on the tool, as a sibling of `spec`, is the RBAC project. Omit it. `spec.project` is the spec field and is required.
- The response is the spec. `applications.txt` is only the Application table and does not show `revisionHistoryLimit`. Use `application-specs.txt`.
