# List applications

`list_application` returns the Applications Argo CD currently has. On a fresh Kind install the tool reports that none match, and kubectl reports no Application resources. When Applications exist, the tool text and kubectl name the same set.

## Sub-features

- `list-json` calls `list_application` with `output_format` `json` and no filters.
- `list-name-only` calls `list_application` with `name_only` true and `output_format` `json`.
- `list-agrees-with-kubectl` compares both responses with `cluster-state/applications.txt`.

## How to get to it (user POV)

- An MCP client calls the tool `list_application` after `initialize`.
- The client sets `output_format` to `json` for a structured body. The server default without that argument is TSV.
- The client sets `name_only` to true when it wants names and a count.

## Driving it with verify-argocd-mcp-server

Preconditions:

- `scripts/doctor.sh` prints `DOCTOR_OK`.
- No extra Application was created by this verification run.

- **JSON list.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh call --feature list-applications --label json list_application '{"output_format":"json"}'`. `DRIVE_OK` points at `summary.json`. The `list_application` call has `is_error` false and `rpc_error` null.
- **Name list.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh call --feature list-applications --label name-only list_application '{"name_only":true,"output_format":"json"}'`. The call has `is_error` false and `rpc_error` null.
- **Cluster view.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/evidence.sh --feature list-applications`. `EVIDENCE_OK` points at `cluster-state/applications.txt`.
- **Empty cluster.** When `applications.txt` contains `No resources found`, both call texts are exactly `No applications found matching the criteria.`
- **Non-empty cluster.** When kubectl prints an Application table, every name in that table appears in both call texts, and neither text names an application absent from the table.

## Gotchas

- `namespace` filters `spec.destination.namespace`. `app_namespace` filters the Application's own namespace. This recipe passes neither.
- A fresh install has no Applications. That agreement is the proof of `list-json`. It does not prove project or selector filtering. Those filters need an Application in a known project, which this baseline does not seed. Do not create one to make a filter look proven.
- `output_format` defaults to `tsv`. Pass `json` when the assertion reads a JSON body.
- The empty response is the sentence above, not a JSON array.
