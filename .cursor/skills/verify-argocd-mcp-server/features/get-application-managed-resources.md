# Managed resources

`get_application_managed_resources` lists the resources Argo CD is managing for an Application. The default body is one compact JSON object with `items`, `totalItems`, and `hasMore`. Each default item has `group`, `kind`, `namespace`, `name`, and `modified`. `detailed` true, or any resource filter, adds the manifest strings and removes `metadata.managedFields` and `kubectl.kubernetes.io/last-applied-configuration` from those strings. `limit` defaults to 50 and `offset` defaults to 0. A page that is not the whole list includes a `notice`.

## Sub-features

- `summary` calls the tool with only `name`.
- `detailed` calls it with `detailed` true.
- `page` calls it with `limit` 1 and `offset` 0.
- `past-end` calls it with `offset` 1000.

## How to get to it (user POV)

- An MCP client calls `get_application_managed_resources` with the Application `name`.
- The client sets `detailed` when it wants manifest strings. A filter (`kind`, `resource_name`, `resource_namespace`, `group`, or `version`) also returns those strings, even when `detailed` is false.
- The client sets `limit` and `offset` to page. The next page starts at the `offset` named in `notice` when `hasMore` is true.

## Driving it with verify-argocd-mcp-server

Preconditions:

- `scripts/doctor.sh` prints `DOCTOR_OK`.
- Application `verify-guestbook` exists and a sync has reached `Succeeded`, as in [sync application](sync-application.md). The managed-resource list stays empty until the controller has compared the app. Repeat the summary call until `totalItems` is greater than 1.

- **Summary.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh call --feature get-application-managed-resources --label summary get_application_managed_resources '{"name":"verify-guestbook"}'`. The call has `is_error` false. The text is one line. It includes `"totalItems"` and `"hasMore"`, includes `"kind"` and `guestbook-ui`, and does not include `targetState`, `liveState`, `predictedLiveState`, or `normalizedLiveState`.
- **Detailed.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh call --feature get-application-managed-resources --label detailed get_application_managed_resources '{"name":"verify-guestbook","detailed":true}'`. The call has `is_error` false. `summary.json` text includes `targetState`. `responses.jsonl` is the full body: it includes `targetState` and does not include `managedFields` or `last-applied-configuration`.
- **First page.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh call --feature get-application-managed-resources --label page get_application_managed_resources '{"name":"verify-guestbook","limit":1,"offset":0}'`. The call has `is_error` false. The text includes `"hasMore":true`, `"totalItems":`, and `Showing items 0-0 of`.
- **Past the end.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh call --feature get-application-managed-resources --label past-end get_application_managed_resources '{"name":"verify-guestbook","limit":1,"offset":1000}'`. The call has `is_error` false. The text includes `"hasMore":false` and `offset 1000 is past`.
- **Cluster view.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/evidence.sh --feature get-application-managed-resources`. `default-workloads.txt` includes Deployment `guestbook-ui`. That name is in the summary `items`.

## Gotchas

- The body is compact JSON from `json.Marshal`. A pretty-printed body is a different tool. `summary.json` keeps the first 8000 characters of that text. Use `responses.jsonl` when the assertion is about the rest of a detailed body.
- `limit` must be at least 1. `offset` must be zero or greater. The defaults are 50 and 0. A response that starts at offset 0 and includes every item omits `notice`.
- There is no health field and no Kubernetes API version on each item. `version` as an argument is an API version filter, not a git revision.
- Secret data in the cached states is hidden by the Argo CD controller. This recipe uses the guestbook Deployment and Service, so that hiding is not proven.
- Do not treat an empty `items` array with `totalItems` 0 as success. The sync has not been observed yet.
