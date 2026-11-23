# Sync application

`sync_application` starts a sync of an existing Application. `revision` is the git revision, tag, or chart version for a single-source Application; omit it to use `spec.source.targetRevision`. `revisions` is the multi-source list, one entry per `spec.sources` item, and an empty entry keeps that source's target. `resources` limits the sync to those objects. `sync_options` replaces the Application's sync options for this operation and does not merge with them. `prune` and `dry_run` still default to false.

## Sub-features

- `create-guestbook` creates Application `verify-guestbook` with auto-sync left off. This is the precondition, not a sync proof.
- `revision-and-options` syncs `revision` `HEAD` with `sync_options` `CreateNamespace=true`.
- `partial-resources` syncs only Deployment `guestbook-ui`.
- `revisions-ignored-for-single-source` sends a bogus `revisions` entry together with `revision` `HEAD`. Argo CD resolves `revision` and does not use `revisions` for a single-source Application.

## How to get to it (user POV)

- An MCP client calls `create_application` for a single-source app and does not set `auto_sync`.
- The client calls `sync_application` with `name` plus the optional arguments above.
- The tool returns the Application as compact JSON with `metadata.managedFields` and `kubectl.kubernetes.io/last-applied-configuration` removed. The accepted operation is `operation.sync`: `revision` is the resolved commit, `syncOptions` lists the strings that were sent, and `resources` lists the partial-sync objects. A single-source operation leaves `revisions` empty.

## Driving it with verify-argocd-mcp-server

Preconditions:

- `scripts/doctor.sh` prints `DOCTOR_OK`.
- Run this after [list applications](list-applications.md). That recipe requires a cluster with no Applications.
- Do not set `auto_sync`. A revision other than the spec target is rejected when auto-sync is on, unless `dry_run` is true.

- **Create the app.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh call --feature sync-application --label create --timeout 180 create_application '{"name":"verify-guestbook","repo_url":"https://github.com/argoproj/argocd-example-apps.git","path":"guestbook","dest_namespace":"default"}'`. The call has `is_error` false. The text includes `verify-guestbook` and `guestbook`. It does not include `"automated"`.
- **Revision and sync options.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh call --feature sync-application --label revision-and-options --timeout 300 sync_application '{"name":"verify-guestbook","revision":"HEAD","sync_options":["CreateNamespace=true"]}'`. The call has `is_error` false. The text includes `CreateNamespace=true` and `"revision":"` followed by a 40-character hex commit. The first sync also clones the repository, so this call is the slow one.
- **Wait until that operation finishes.** Repeat `kubectl --context kind-argocd-mcp-server -n argocd get application verify-guestbook -o jsonpath='{.status.operationState.phase}{"\n"}'` until it prints `Succeeded`. Do not start another sync while the phase is `Running`.
- **Partial sync.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh call --feature sync-application --label partial-resources --timeout 180 sync_application '{"name":"verify-guestbook","resources":[{"group":"apps","kind":"Deployment","name":"guestbook-ui","namespace":"default"}]}'`. The call has `is_error` false. The text includes `"kind":"Deployment"` and `"name":"guestbook-ui"`.
- **Wait again** until `operationState.phase` is `Succeeded`.
- **Single-source revisions.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh call --feature sync-application --label revisions --timeout 180 sync_application '{"name":"verify-guestbook","revision":"HEAD","revisions":["not-a-real-revision"]}'`. The call has `is_error` false. The text includes a 40-character `"revision"` and does not include `not-a-real-revision`.
- **Cluster view.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/evidence.sh --feature sync-application`. `applications.txt` includes `verify-guestbook`. `application-specs.txt` has repo URL `https://github.com/argoproj/argocd-example-apps.git`, path `guestbook`, destination namespace `default`, and a `syncRevision` equal to the hex revision from the sync text. `default-workloads.txt` includes Deployment `guestbook-ui`.

## Gotchas

- `revision` in the response is the commit Argo CD resolved, not the string `HEAD`. `spec.source.targetRevision` stays `HEAD`.
- `revisions` is how a multi-source Application is pinned, one string per `spec.sources` entry. This recipe's app has `spec.source` only. A successful multi-source sync is not proven here.
- `resources` requires `kind` and `name`. `group` is empty for core objects such as ConfigMap. `namespace` empty matches the name in any namespace.
- `sync_options` replaces `spec.syncPolicy.syncOptions` for this operation. Omitting the argument keeps the Application's own options. This app is created with none.
- A second sync while `operationState.phase` is `Running` fails. Wait for `Succeeded`.
- Run the empty-cluster recipes before this one. Creating `verify-guestbook` is the mutation those recipes must not see.
