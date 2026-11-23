# List clusters

`list_cluster` and `get_cluster` show the clusters registered in Argo CD. A fresh Kind install has the in-cluster API at `https://kubernetes.default.svc`. The MCP text and the `# api-clusters` section of `clusters.txt` both include that URL. A fresh Argo CD 3.6 install keeps that entry in the API and does not store a cluster secret for it. Cluster credentials stay out of both views.

## Sub-features

- `list-name-only` returns cluster name and server with `name_only` true.
- `get-in-cluster` loads `https://kubernetes.default.svc` with `get_cluster`.
- `list-agrees-with-secrets` matches those servers to `cluster-state/clusters.txt`.

## How to get to it (user POV)

- An MCP client calls `list_cluster` with `name_only` true.
- The client then calls `get_cluster` with `server` set to a server URL from that list. The argument name is `server`, and it is required.

## Driving it with verify-argocd-mcp-server

Preconditions:

- `scripts/doctor.sh` prints `DOCTOR_OK`.
- Argo CD has finished registering the local cluster. `evidence.sh` writes at least one server URL.

- **Name list.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh call --feature list-clusters --label name-only list_cluster '{"name_only":true}'`. The call text includes `https://kubernetes.default.svc` and `"count"`. `is_error` is false.
- **Get the in-cluster server.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh call --feature list-clusters --label get-in-cluster get_cluster '{"server":"https://kubernetes.default.svc"}'`. The call text includes `https://kubernetes.default.svc`. `is_error` is false.
- **Cluster view.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/evidence.sh --feature list-clusters`. `clusters.txt` lists persisted cluster secrets under `# cluster-secrets` (name, tab, decoded server URL only) and the clusters API under `# api-clusters` (name, tab, server). Config and certificates are omitted. The api-clusters section includes `in-cluster` and `https://kubernetes.default.svc`.
- **Proof.** The server URL is in the `name-only` text, the `get-in-cluster` text, and the api-clusters section. None of those files contain a bearer token, a kubeconfig key, or a certificate.

## Gotchas

- `get_cluster` without `server` returns a tool error. That is a missing argument, not proof that the cluster is absent.
- `clusters.txt` is names and server URLs only. Do not replace it with `kubectl get secret -o yaml`; that dump includes credentials. A fresh Argo CD 3.6 install has no cluster secret for the in-cluster entry, so the api-clusters section is the second view.
- The in-cluster name is commonly `in-cluster`. Assert the server URL, which is stable, and record the name the tool actually returned.
