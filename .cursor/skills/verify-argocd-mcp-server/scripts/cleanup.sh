#!/usr/bin/env bash
# Stop MCP processes this skill started and delete the Kind cluster it owns.
# Evidence under .cursor/skills/verify-argocd-mcp-server/evidence is kept.
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

require_cmd kind

if [[ -d "$STATE_DIR/pids" ]]; then
  for pid_file in "$STATE_DIR/pids"/*; do
    [[ -f "$pid_file" ]] || continue
    pid=$(tr -d '[:space:]' < "$pid_file")
    if [[ "$pid" =~ ^[0-9]+$ ]] && [[ -r "/proc/${pid}/cmdline" ]]; then
      cmd=$(tr '\0' ' ' < "/proc/${pid}/cmdline")
      if [[ "$cmd" == *argocd-mcp-server* ]]; then
        echo "stopping pid ${pid}"
        kill -TERM -- "-${pid}" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true
        for _ in 1 2 3 4 5 6 7 8 9 10; do
          if [[ ! -d "/proc/${pid}" ]]; then
            break
          fi
          sleep 0.2
        done
        if [[ -d "/proc/${pid}" ]]; then
          kill -KILL -- "-${pid}" 2>/dev/null || kill -KILL "$pid" 2>/dev/null || true
        fi
      else
        echo "leaving pid ${pid}; cmdline is not argocd-mcp-server"
      fi
    fi
    rm -f "$pid_file"
  done
fi

owned=0
if [[ -f "$STATE_DIR/owned-cluster" ]] && [[ "$(tr -d '[:space:]' < "$STATE_DIR/owned-cluster")" == "$CLUSTER_NAME" ]]; then
  owned=1
fi

if [[ "$owned" -eq 1 ]]; then
  if kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; then
    echo "deleting Kind cluster ${CLUSTER_NAME}"
    kind delete cluster --name "$CLUSTER_NAME"
  else
    echo "owned cluster ${CLUSTER_NAME} is already gone"
  fi
else
  if kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; then
    echo "leaving Kind cluster ${CLUSTER_NAME}; this state directory does not own it"
  fi
fi

if [[ -f "$STATE_DIR/built-binary" ]]; then
  built=$(tr -d '[:space:]' < "$STATE_DIR/built-binary")
  if [[ "$built" == "$REPO_ROOT/argocd-mcp-server" && -f "$built" ]]; then
    rm -f "$built"
  fi
fi

if [[ -d "$STATE_DIR" ]]; then
  rm -rf "$STATE_DIR"
fi

if [[ -d "$EVIDENCE_ROOT" ]]; then
  echo "EVIDENCE_KEPT ${EVIDENCE_ROOT}"
  find "$EVIDENCE_ROOT" -type f | sort
else
  echo "EVIDENCE_ABSENT ${EVIDENCE_ROOT}"
fi
