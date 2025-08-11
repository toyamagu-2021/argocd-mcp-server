#!/bin/bash

# Create test input
cat > test_input.json << 'EOF'
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"0.1.0","capabilities":{"roots":{"listChanged":true},"sampling":{}},"clientInfo":{"name":"test","version":"1.0.0"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_application","arguments":{}}}
EOF

echo "Testing list_application with gRPC-Web..."
ARGOCD_GRPC_WEB=true ./argocd-mcp-server < test_input.json 2>/dev/null | jq -s 'map(select(.id == 2)) | .[0] | if .result then "SUCCESS: Found \(.result | length) applications" elif .error then "ERROR: \(.error.message)" else "No response for list_application" end'

rm test_input.json