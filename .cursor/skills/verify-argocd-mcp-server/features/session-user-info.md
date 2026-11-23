# Session user info

`get_user_info` reports the account behind `ARGOCD_AUTH_TOKEN`. Launch mints that token for `admin`. The tool text and `cluster-state/userinfo.json` both show `loggedIn` true and username `admin`.

## Sub-features

- `get-user` calls `get_user_info` with an empty arguments object.
- `user-matches-api` compares the tool text with the session userinfo document written by `evidence.sh`.

## How to get to it (user POV)

- An MCP client calls `get_user_info`. The tool takes no arguments.
- The client is whoever holds the token the server process was started with. This skill's process uses the admin API token from launch.

## Driving it with verify-argocd-mcp-server

Preconditions:

- `scripts/doctor.sh` prints `DOCTOR_OK` and `user=admin`.

- **Call the tool.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh call --feature session-user-info --label get-user get_user_info '{}'`. The call has `is_error` false and `rpc_error` null. The text includes `"loggedIn": true` and `"username": "admin"`.
- **API view.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/evidence.sh --feature session-user-info`. `userinfo.json` is `{"loggedIn": true, "username": "admin", ...}` with no token field.
- **Proof.** The username in the tool text equals `username` in `userinfo.json`. The transcript does not contain the raw token.

## Gotchas

- Doctor already calls userinfo. That doctor line is not the feature proof. The proof is a `tools/call` for `get_user_info` plus `userinfo.json` from `evidence.sh` after that call.
- The tool field is `issuer`. The evidence file omits the issuer on purpose and keeps `loggedIn`, `username`, and `groups`. Compare username and logged-in state.
- A 401 from userinfo means the token file is unusable. Run launch again only if this state directory still owns the cluster. Do not mint a token into `.env`.
