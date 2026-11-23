#!/usr/bin/env python3
"""Drive argocd-mcp-server over stdio the way an MCP client does.

Sends initialize, then notifications/initialized, then each JSON-RPC request
from a JSONL file. Pairs each request that has an id with one stdout response.
Writes redacted transcripts under --evidence-dir and removes --pid-file on exit.

The Argo CD token is read from the environment (ARGOCD_AUTH_TOKEN), never from
argv. JWTs are stripped from every file this process writes.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import select
import signal
import subprocess
import sys
import time
from pathlib import Path

JWT_RE = re.compile(r"eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}")

INITIALIZE = {
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
        "protocolVersion": "2024-11-05",
        "capabilities": {"roots": {"listChanged": True}},
        "clientInfo": {"name": "verify-argocd-mcp-server", "version": "1.0.0"},
    },
}

INITIALIZED = {"jsonrpc": "2.0", "method": "notifications/initialized"}


def redact(text: str) -> str:
    return JWT_RE.sub("[redacted-jwt]", text)


def write_json(path: Path, payload: object) -> None:
    path.write_text(redact(json.dumps(payload, indent=2) + "\n"), encoding="utf-8")


def tool_text(message: dict) -> str:
    result = message.get("result")
    if not isinstance(result, dict):
        return ""
    parts: list[str] = []
    content = result.get("content")
    if isinstance(content, list):
        for block in content:
            if isinstance(block, dict) and block.get("type") == "text":
                parts.append(str(block.get("text") or ""))
    return "\n".join(parts)


class LineReader:
    def __init__(self, stream) -> None:
        self.stream = stream
        self.buf = b""

    def readline(self, proc: subprocess.Popen, timeout: float) -> str | None:
        deadline = time.time() + timeout
        while True:
            if b"\n" in self.buf:
                raw, self.buf = self.buf.split(b"\n", 1)
                return raw.decode("utf-8", errors="replace")
            remaining = deadline - time.time()
            if remaining <= 0:
                return None
            ready, _, _ = select.select([self.stream], [], [], min(remaining, 0.5))
            if not ready:
                if proc.poll() is not None and not self.buf:
                    return None
                continue
            chunk = os.read(self.stream.fileno(), 65536)
            if not chunk:
                if self.buf:
                    raw = self.buf
                    self.buf = b""
                    return raw.decode("utf-8", errors="replace")
                return None
            self.buf += chunk


def kill_proc(proc: subprocess.Popen | None) -> None:
    if proc is None or proc.poll() is not None:
        return
    try:
        os.killpg(proc.pid, signal.SIGTERM)
    except ProcessLookupError:
        return
    deadline = time.time() + 5
    while time.time() < deadline and proc.poll() is None:
        time.sleep(0.1)
    if proc.poll() is None:
        try:
            os.killpg(proc.pid, signal.SIGKILL)
        except ProcessLookupError:
            return
        proc.wait(timeout=5)


def load_requests(path: Path) -> list[dict]:
    requests: list[dict] = []
    next_id = 2
    for line_no, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        try:
            message = json.loads(stripped)
        except json.JSONDecodeError as exc:
            raise SystemExit(f"request line {line_no} is not JSON: {exc}") from exc
        if not isinstance(message, dict) or "method" not in message:
            raise SystemExit(f"request line {line_no} needs a method")
        if message.get("method") == "initialize":
            raise SystemExit("request file must not repeat initialize; the harness sends it")
        if "id" not in message and not str(message["method"]).startswith("notifications/"):
            message["id"] = next_id
            next_id += 1
        elif isinstance(message.get("id"), int):
            next_id = max(next_id, message["id"] + 1)
        requests.append(message)
    return requests


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", required=True)
    parser.add_argument("--evidence-dir", required=True)
    parser.add_argument("--requests", required=True, help="JSONL file of requests after initialize")
    parser.add_argument("--pid-file", required=True)
    parser.add_argument("--feature", required=True)
    parser.add_argument("--label", required=True)
    parser.add_argument("--timeout", type=float, default=120.0)
    parser.add_argument("--read-only", choices=("true", "false"), required=True)
    args = parser.parse_args()

    evidence = Path(args.evidence_dir)
    evidence.mkdir(parents=True, exist_ok=True)
    pid_path = Path(args.pid_file)
    pid_path.parent.mkdir(parents=True, exist_ok=True)

    binary = Path(args.binary)
    if not binary.is_file():
        raise SystemExit(f"binary not found: {binary}")

    requests = load_requests(Path(args.requests))
    stderr_path = evidence / "server.stderr.log"
    request_log = evidence / "requests.jsonl"
    response_log = evidence / "responses.jsonl"
    transcript_path = evidence / "transcript.jsonl"

    env = os.environ.copy()
    env["LOG_FORMAT"] = "text"
    env.setdefault("LOG_LEVEL", "info")
    if args.read_only == "true":
        env["ARGOCD_MCP_READ_ONLY"] = "true"
    else:
        env.pop("ARGOCD_MCP_READ_ONLY", None)
    if not env.get("ARGOCD_AUTH_TOKEN") or not env.get("ARGOCD_SERVER"):
        raise SystemExit("ARGOCD_AUTH_TOKEN and ARGOCD_SERVER must be set in the environment")

    proc: subprocess.Popen | None = None
    stderr_file = None

    def handle_signal(signum, _frame) -> None:
        kill_proc(proc)
        if pid_path.exists():
            pid_path.unlink()
        raise SystemExit(128 + signum)

    signal.signal(signal.SIGTERM, handle_signal)
    signal.signal(signal.SIGINT, handle_signal)

    sent: list[dict] = []
    received: list[dict] = []
    transcript: list[dict] = []
    failure: BaseException | None = None
    init_response: dict = {}

    def record(direction: str, message: dict) -> None:
        transcript.append({"direction": direction, "message": message})

    try:
        stderr_file = stderr_path.open("w", encoding="utf-8")
        proc = subprocess.Popen(
            [str(binary)],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=stderr_file,
            env=env,
            start_new_session=True,
        )
        pid_path.write_text(f"{proc.pid}\n", encoding="utf-8")
        if proc.stdin is None or proc.stdout is None:
            raise SystemExit("failed to attach stdio pipes")
        reader = LineReader(proc.stdout)

        def send(message: dict) -> None:
            raw = json.dumps(message, separators=(",", ":"))
            proc.stdin.write((raw + "\n").encode("utf-8"))
            proc.stdin.flush()
            sent.append(message)
            record("client", message)

        def wait_response(expect_id, timeout: float) -> dict:
            deadline = time.time() + timeout
            while True:
                remaining = deadline - time.time()
                if remaining <= 0:
                    tail = ""
                    if stderr_path.exists():
                        tail = redact(stderr_path.read_text(encoding="utf-8", errors="replace")[-2000:])
                    raise SystemExit(
                        f"timed out waiting for response id={expect_id}. server stderr tail:\n{tail}"
                    )
                line = reader.readline(proc, remaining)
                if line is None:
                    tail = ""
                    if stderr_path.exists():
                        tail = redact(stderr_path.read_text(encoding="utf-8", errors="replace")[-2000:])
                    code = proc.poll()
                    raise SystemExit(
                        f"server closed stdout before response id={expect_id} exit={code}. stderr tail:\n{tail}"
                    )
                line = line.strip()
                if not line:
                    continue
                try:
                    message = json.loads(line)
                except json.JSONDecodeError:
                    record("server-non-json", {"text": redact(line)})
                    continue
                record("server", message)
                if isinstance(message, dict) and message.get("id") == expect_id:
                    received.append(message)
                    return message
                # A notification or a different id stays in the transcript.
                # Keep waiting so one unexpected line cannot desynchronize the session.

        send(INITIALIZE)
        init_response = wait_response(1, args.timeout)
        send(INITIALIZED)
        for message in requests:
            send(message)
            if "id" in message and not str(message.get("method", "")).startswith("notifications/"):
                wait_response(message["id"], args.timeout)

        proc.stdin.close()
        deadline = time.time() + 10
        while time.time() < deadline and proc.poll() is None:
            time.sleep(0.1)
    except SystemExit as exc:
        failure = exc
    finally:
        kill_proc(proc)
        if stderr_file is not None and not stderr_file.closed:
            stderr_file.close()
        if pid_path.exists():
            pid_path.unlink()

    if stderr_path.exists():
        stderr_path.write_text(redact(stderr_path.read_text(encoding="utf-8", errors="replace")), encoding="utf-8")

    request_log.write_text(
        "".join(redact(json.dumps(message) + "\n") for message in sent),
        encoding="utf-8",
    )
    response_log.write_text(
        "".join(redact(json.dumps(message) + "\n") for message in received),
        encoding="utf-8",
    )
    transcript_path.write_text(
        "".join(redact(json.dumps(entry) + "\n") for entry in transcript),
        encoding="utf-8",
    )

    init_result = init_response.get("result") if isinstance(init_response, dict) else None
    server_info = init_result.get("serverInfo", {}) if isinstance(init_result, dict) else {}
    tool_names: list[str] = []
    calls: list[dict] = []
    by_id = {message.get("id"): message for message in received if isinstance(message, dict)}
    for message in requests:
        response = by_id.get(message.get("id"))
        if not isinstance(response, dict):
            continue
        if message.get("method") == "tools/list":
            result = response.get("result")
            tools = result.get("tools") if isinstance(result, dict) else None
            if isinstance(tools, list):
                for tool in tools:
                    if isinstance(tool, dict) and isinstance(tool.get("name"), str):
                        tool_names.append(tool["name"])
        if message.get("method") == "tools/call":
            params = message.get("params") if isinstance(message.get("params"), dict) else {}
            error = response.get("error")
            result = response.get("result") if isinstance(response.get("result"), dict) else {}
            calls.append(
                {
                    "id": message.get("id"),
                    "tool": params.get("name"),
                    "is_error": bool(result.get("isError")) if isinstance(result, dict) else False,
                    "rpc_error": error if isinstance(error, dict) else None,
                    "text": redact(tool_text(response))[:8000],
                }
            )

    stderr_text = stderr_path.read_text(encoding="utf-8", errors="replace") if stderr_path.exists() else ""
    summary = {
        "feature": args.feature,
        "label": args.label,
        "read_only": args.read_only == "true",
        "binary": str(binary),
        "server_pid_recorded": True,
        "initialize": {
            "server_name": server_info.get("name"),
            "protocol_version": init_result.get("protocolVersion") if isinstance(init_result, dict) else None,
            "ok": isinstance(init_result, dict) and server_info.get("name") == "argocd-mcp-server",
        },
        "tool_names": tool_names,
        "calls": calls,
        "stderr_log": str(stderr_path),
        "responses": str(response_log),
        "requests": str(request_log),
        "transcript": str(transcript_path),
        "stderr_mentions_read_only": "Read-only mode: state-changing tools are not registered" in stderr_text,
        "stderr_mentions_registered": "All tools registered successfully" in stderr_text,
        "stderr_mentions_waiting": "Waiting for requests on stdin" in stderr_text,
    }
    summary_path = evidence / "summary.json"
    write_json(summary_path, summary)

    if failure is not None or not summary["initialize"]["ok"]:
        print(f"DRIVE_FAIL summary={summary_path} reason={failure}", file=sys.stderr)
        return 1
    print(f"DRIVE_OK feature={args.feature} label={args.label} summary={summary_path}", flush=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
