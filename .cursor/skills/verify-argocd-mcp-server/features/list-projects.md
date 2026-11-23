# List projects

`list_project` returns Argo CD AppProjects. A fresh install includes the project `default`. The MCP text and `kubectl get appprojects.argoproj.io -n argocd` both show that name.

## Sub-features

- `list-name-only` returns project names and a count.
- `list-includes-default` requires `default` in that payload.
- `list-agrees-with-kubectl` matches the names to `cluster-state/appprojects.txt`.

## How to get to it (user POV)

- An MCP client calls `list_project`.
- The client sets `name_only` to true for names. The JSON body is a page of name strings. `totalItems` is the count. Without it, the tool returns project summaries.

## Driving it with verify-argocd-mcp-server

Preconditions:

- `scripts/doctor.sh` prints `DOCTOR_OK`.
- The `default` AppProject exists. `install-argocd` leaves it in place. If it is missing, this feature fails; do not recreate it with `create_project` as part of the proof.

- **Name list.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh call --feature list-projects --label name-only list_project '{"name_only":true}'`. The call has `is_error` false and `rpc_error` null. The text includes `"default"` and `"totalItems"`.
- **Project view.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/evidence.sh --feature list-projects`. `appprojects.txt` includes a row named `default`.
- **Proof.** Every project name in the kubectl table appears in the tool text, and the tool text includes `default`.

## Gotchas

- An empty tool body `No projects found.` means the baseline is wrong. Report the failure. Do not switch the assertion to "empty is fine".
- `name_only` true returns a page whose `items` are name strings. `totalItems` replaces `count`. The summary list is a different shape. This recipe uses `name_only`.
