#!/bin/bash

# Send MCP requests and capture all responses
(
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-18","capabilities":{"roots":{"listChanged":true},"sampling":{}},"clientInfo":{"name":"test-client","version":"1.0.0"}}}'
sleep 0.5
echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_application","arguments":{}}}'
sleep 1
) | ./argocd-mcp-server 2>&1