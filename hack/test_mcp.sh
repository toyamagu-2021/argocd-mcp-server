#!/bin/bash

# Test the MCP server with a list_application request
cat <<EOF | ./argocd-mcp-server 2>/dev/null
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_application","arguments":{"namespace":"argocd"}}}
EOF
