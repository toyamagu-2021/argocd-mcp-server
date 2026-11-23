# Rollback application

`rollback_application` rolls an Application back to a previous deployment recorded in `status.history`. `id` is that history entry's id, and zero is valid. It is not a git revision. `dry_run` and `prune` default to false. Argo CD refuses the call when auto-sync is enabled. `prune` false does not delete resources that the target revision no longer defines.

## Sub-features

- `read-history` reads `status.history` from `get_application` after a completed sync.
- `dry-run` calls `rollback_application` with that id and `dry_run` true.
- `history-unchanged` compares the history ids in `application-specs.txt` with the ids from before the dry run.

## How to get to it (user POV)

- An MCP client calls `get_application` with `detailed` true and reads `status.history[].id`. The default summary has `historyCount` and no history ids. History is written when a sync finishes, not when the sync RPC returns.
- The client calls `rollback_application` with `name`, that `id`, and `dry_run` true to preview the rollback.

## Driving it with verify-argocd-mcp-server

Preconditions:

- `scripts/doctor.sh` prints `DOCTOR_OK`.
- Application `verify-guestbook` has no automated sync policy and `status.operationState.phase` has reached `Succeeded` at least once, as in [sync application](sync-application.md). If `status.history` is empty, the sync has not finished. Repeat `get_application` with `detailed` true until the text includes `"history"`.

- **Read history.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh call --feature rollback-application --label history get_application '{"name":"verify-guestbook","detailed":true}'`. The call has `is_error` false. The text includes `"history"` and an `"id"` inside it. Record that id. Zero is a valid id.
- **Dry-run rollback.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh call --feature rollback-application --label dry-run rollback_application '{"name":"verify-guestbook","id":HISTORY_ID,"dry_run":true}'`, with `HISTORY_ID` replaced by the recorded number. The call has `is_error` false and `rpc_error` null. The text includes `verify-guestbook`.
- **Cluster view.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/evidence.sh --feature rollback-application`. The `historyIds` column for `verify-guestbook` in `application-specs.txt` contains the same id. `applications.txt` still lists `verify-guestbook`.

## Gotchas

- `dry_run` true previews the rollback and does not start a new deployment. This recipe does not prove a live rollback and does not prove `prune`.
- An id that is not in `status.history` is a tool error, `Failed to rollback application`. That is not a successful rollback.
- Auto-sync on the Application makes Argo CD refuse the call. The guestbook app in this map is created with `auto_sync` left off. Do not turn it on in [update application](update-application.md).
- `id` is a JSON number. A string id is rejected before Argo CD sees it.
