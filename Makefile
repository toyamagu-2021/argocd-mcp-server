# ArgoCD MCP Server Makefile

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOVET=$(GOCMD) vet
BINARY_NAME=argocd-mcp-server
BINARY_PATH=./cmd/argocd-mcp-server

# E2E Test Variables
CLUSTER_NAME := argocd-mcp-server
ARGOCD_VERSION := v2.14.14
ARGOCD_NAMESPACE := argocd
KUBECTL := kubectl
KIND := kind
EXPECTED_CONTEXT := kind-$(CLUSTER_NAME)

# Build
.PHONY: build
build:
	$(GOBUILD) -o $(BINARY_NAME) $(BINARY_PATH)

# Clean
.PHONY: clean
clean:
	$(GOCMD) clean
	rm -f $(BINARY_NAME)

# Dependencies
.PHONY: deps
deps:
	$(GOMOD) download
	$(GOMOD) verify

.PHONY: tidy
tidy:
	$(GOMOD) tidy

# Linting
.PHONY: fmt
fmt:
	$(GOFMT) -w .
	$(GOCMD) fmt ./...
	@which goimports > /dev/null || go install golang.org/x/tools/cmd/goimports@latest
	goimports -w .

.PHONY: fmt-check
fmt-check:
	@echo "Checking formatting..."
	@test -z "$$($(GOFMT) -l .)" || (echo "Files need formatting. Run 'make fmt'" && $(GOFMT) -l . && exit 1)
	@echo "Checking imports formatting..."
	@which goimports > /dev/null || go install golang.org/x/tools/cmd/goimports@latest
	@test -z "$$(goimports -l .)" || (echo "Files need import formatting. Run 'make fmt'" && goimports -l . && exit 1)

.PHONY: vet
vet:
	$(GOVET) ./...

.PHONY: lint
lint: fmt-check vet
	@echo "Running staticcheck..."
	@which staticcheck > /dev/null || go install honnef.co/go/tools/cmd/staticcheck@latest
	staticcheck ./...
	@echo "Running golint..."
	@which golint > /dev/null || go install golang.org/x/lint/golint@latest
	golint ./...
	@echo "Running ineffassign..."
	@which ineffassign > /dev/null || go install github.com/gordonklaus/ineffassign@latest
	ineffassign ./...
	@echo "Running errcheck..."
	@which errcheck > /dev/null || go install github.com/kisielk/errcheck@latest
	errcheck ./...
	@echo "Running misspell..."
	@which misspell > /dev/null || go install github.com/client9/misspell/cmd/misspell@latest
	misspell -error .

.PHONY: lint-basic
lint-basic: fmt-check vet
	@echo "Basic linting complete"

.PHONY: lint-install
lint-install:
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install golang.org/x/lint/golint@latest
	go install github.com/gordonklaus/ineffassign@latest
	go install github.com/client9/misspell/cmd/misspell@latest
	go install github.com/kisielk/errcheck@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install github.com/mgechev/revive@latest
	go install golang.org/x/tools/cmd/goimports@latest

.PHONY: lint-advanced
lint-advanced: lint
	@echo "Running gosec (security)..."
	@which gosec > /dev/null || go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec -fmt text ./...
	@echo "Running revive..."
	@which revive > /dev/null || go install github.com/mgechev/revive@latest
	revive -config .revive.toml ./... || revive ./...

# All checks
.PHONY: check
check: lint test

.PHONY: check-all
check-all: lint-advanced test-race

# Run
.PHONY: run
run: build
	./$(BINARY_NAME)

# Install
.PHONY: install
install:
	$(GOCMD) install $(BINARY_PATH)

# E2E Environment Setup
# Check if tools are installed
.PHONY: check-tools
check-tools:
	@command -v $(KIND) >/dev/null 2>&1 || { echo "kind is not installed. Please install kind first."; exit 1; }
	@command -v $(KUBECTL) >/dev/null 2>&1 || { echo "kubectl is not installed. Please install kubectl first."; exit 1; }

# Verify kubectl context is set to the expected cluster
.PHONY: check-context
check-context:
	@CURRENT_CONTEXT=$$($(KUBECTL) config current-context 2>/dev/null); \
	if [ "$$CURRENT_CONTEXT" != "$(EXPECTED_CONTEXT)" ]; then \
		echo "❌ ERROR: Current kubectl context is '$$CURRENT_CONTEXT', expected '$(EXPECTED_CONTEXT)'"; \
		echo "Please run 'kubectl config use-context $(EXPECTED_CONTEXT)' or 'make kind-create' first"; \
		exit 1; \
	fi; \
	echo "✅ Context verified: $(EXPECTED_CONTEXT)"

# Create Kind cluster
.PHONY: kind-create
kind-create: check-tools
	@echo "Creating Kind cluster: $(CLUSTER_NAME)"
	@$(KIND) create cluster --name $(CLUSTER_NAME) --config kind-config.yaml || echo "Cluster $(CLUSTER_NAME) already exists"
	@$(KUBECTL) cluster-info --context kind-$(CLUSTER_NAME)

# Delete Kind cluster
.PHONY: kind-delete
kind-delete:
	@echo "Deleting Kind cluster: $(CLUSTER_NAME)"
	@$(KIND) delete cluster --name $(CLUSTER_NAME)

# Install ArgoCD
.PHONY: install-argocd
install-argocd: check-context
	@echo "Creating ArgoCD namespace"
	@$(KUBECTL) create namespace $(ARGOCD_NAMESPACE) --dry-run=client -o yaml | $(KUBECTL) apply -f -
	@echo "Installing ArgoCD $(ARGOCD_VERSION)"
	@$(KUBECTL) apply -n $(ARGOCD_NAMESPACE) -f https://raw.githubusercontent.com/argoproj/argo-cd/$(ARGOCD_VERSION)/manifests/install.yaml
	@echo "Waiting for ArgoCD to be ready..."
	@$(KUBECTL) wait --for=condition=available --timeout=300s deployment/argocd-server -n $(ARGOCD_NAMESPACE)
	@$(KUBECTL) wait --for=condition=available --timeout=300s deployment/argocd-repo-server -n $(ARGOCD_NAMESPACE)
	@$(KUBECTL) wait --for=condition=available --timeout=300s deployment/argocd-applicationset-controller -n $(ARGOCD_NAMESPACE)
	@$(KUBECTL) wait --for=condition=available --timeout=300s deployment/argocd-notifications-controller -n $(ARGOCD_NAMESPACE)
	@$(KUBECTL) wait --for=condition=available --timeout=300s deployment/argocd-dex-server -n $(ARGOCD_NAMESPACE)
	@$(KUBECTL) wait --for=condition=available --timeout=300s deployment/argocd-redis -n $(ARGOCD_NAMESPACE)
	@echo "Configuring ArgoCD to allow API key for admin account..."
	@$(KUBECTL) patch configmap argocd-cm -n $(ARGOCD_NAMESPACE) --type merge -p '{"data":{"accounts.admin":"apiKey, login"}}'
	@echo "Configuring ArgoCD server service as NodePort..."
	@$(KUBECTL) patch service argocd-server -n $(ARGOCD_NAMESPACE) --type merge -p '{"spec":{"type":"NodePort","ports":[{"name":"https","port":443,"protocol":"TCP","targetPort":8080,"nodePort":30080}]}}'
	@echo "ArgoCD installed and configured successfully"
	@echo "ArgoCD server is now accessible at localhost:8080"
	@echo "Waiting for ArgoCD server to be accessible..."
	@for i in $$(seq 1 30); do \
		if curl -k -s -o /dev/null https://localhost:8080/api/version 2>/dev/null; then \
			echo "✅ ArgoCD server is accessible at localhost:8080"; \
			break; \
		fi; \
		echo "Waiting for ArgoCD server... ($$i/30)"; \
		sleep 2; \
	done

# Get ArgoCD admin password
.PHONY: argocd-password
argocd-password: check-context
	@echo "ArgoCD admin password:"
	@$(KUBECTL) -n $(ARGOCD_NAMESPACE) get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d && echo

# Generate .env file with ArgoCD credentials
.PHONY: generate-env
generate-env: check-context
	@echo "Generating .env file with ArgoCD credentials..."
	@echo "# ArgoCD Environment Variables" > .env
	@echo "ARGOCD_SERVER=localhost:8080" >> .env
	@echo "" >> .env
	@echo "ARGOCD_INSECURE=true" >> .env
	@echo "ARGOCD_PLAINTEXT=false" >> .env
	@echo "" >> .env
	@echo "# Admin credentials (for argocd CLI login)" >> .env
	@echo "ARGOCD_ADMIN_USER=admin" >> .env
	@printf "ARGOCD_ADMIN_PASSWORD=" >> .env
	@$(KUBECTL) -n $(ARGOCD_NAMESPACE) get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d >> .env
	@echo "" >> .env
	@echo ".env file generated successfully"
	@echo "Note: ArgoCD is accessible at localhost:8080 (no port-forwarding needed)"

# Generate ArgoCD token and update .env using CLI
.PHONY: generate-token
generate-token: check-context
	@echo "Generating ArgoCD token using CLI..."
	@echo "Note: This requires ArgoCD CLI and ArgoCD to be accessible at localhost:8080"
	@echo "Installing argocd CLI if not present..."
	@command -v argocd >/dev/null 2>&1 || brew install argocd
	@echo "Logging in to ArgoCD..."
	@ARGOCD_PASSWORD=$$($(KUBECTL) -n $(ARGOCD_NAMESPACE) get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d); \
	argocd login localhost:8080 --username admin --password $$ARGOCD_PASSWORD --insecure
	@echo "Generating token..."
	@TOKEN=$$(argocd account generate-token --account admin --insecure); \
	if [ -n "$$TOKEN" ]; then \
		cp .env .env.bak 2>/dev/null || true; \
		if grep -q "^ARGOCD_AUTH_TOKEN=" .env 2>/dev/null; then \
			sed -i.tmp "s/^ARGOCD_AUTH_TOKEN=.*/ARGOCD_AUTH_TOKEN=$$TOKEN/" .env && rm -f .env.tmp; \
		else \
			echo "ARGOCD_AUTH_TOKEN=$$TOKEN" >> .env; \
		fi; \
		echo "✅ Token generated and saved to .env file"; \
	else \
		echo "❌ Failed to generate token"; \
		exit 1; \
	fi

# Generate ArgoCD token using API (alternative method)
.PHONY: generate-token-api
generate-token-api: check-context
	@echo "Generating ArgoCD token using API..."
	@echo "Note: This requires ArgoCD to be accessible at localhost:8080"
	@ARGOCD_PASSWORD=$$($(KUBECTL) -n $(ARGOCD_NAMESPACE) get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d); \
	echo "Getting session token..."; \
	SESSION_TOKEN=$$(curl -s -X POST localhost:8080/api/v1/session \
		-H "Content-Type: application/json" \
		-d "{\"username\":\"admin\",\"password\":\"$$ARGOCD_PASSWORD\"}" \
		-k | grep -o '"token":"[^"]*' | sed 's/"token":"//'); \
	if [ -n "$$SESSION_TOKEN" ]; then \
		echo "Generating API token..."; \
		API_TOKEN=$$(curl -s -X POST localhost:8080/api/v1/account/admin/token \
			-H "Authorization: Bearer $$SESSION_TOKEN" \
			-H "Content-Type: application/json" \
			-d '{"name":"mcp-server-token"}' \
			-k | grep -o '"token":"[^"]*' | sed 's/"token":"//'); \
		if [ -n "$$API_TOKEN" ]; then \
			cp .env .env.bak 2>/dev/null || true; \
			if grep -q "^ARGOCD_AUTH_TOKEN=" .env 2>/dev/null; then \
				sed -i.tmp "s/^ARGOCD_AUTH_TOKEN=.*/ARGOCD_AUTH_TOKEN=$$API_TOKEN/" .env && rm -f .env.tmp; \
			else \
				echo "ARGOCD_AUTH_TOKEN=$$API_TOKEN" >> .env; \
			fi; \
			echo "✅ API token generated and saved to .env file"; \
		else \
			echo "❌ Failed to generate API token"; \
			exit 1; \
		fi; \
	else \
		echo "❌ Failed to get session token"; \
		exit 1; \
	fi

# Port forward ArgoCD server (DEPRECATED - now using NodePort)
.PHONY: argocd-port-forward
argocd-port-forward: check-context
	@echo "⚠️  WARNING: Port forwarding is deprecated. ArgoCD is now exposed via NodePort on localhost:8080"
	@echo "    This target is maintained for compatibility but is no longer needed."
	@echo "    ArgoCD should already be accessible at localhost:8080 after 'make install-argocd'"

# Setup E2E test environment (create cluster and install ArgoCD)
.PHONY: e2e-setup
e2e-setup: kind-create install-argocd generate-env generate-token
	@echo "E2E test environment setup complete"
	@echo "Environment variables saved to .env file"
	@echo "ArgoCD is accessible at localhost:8080"
	@echo "To generate a new token, run: make generate-token"

# Teardown E2E test environment
.PHONY: e2e-teardown
e2e-teardown: kind-delete
	@echo "E2E test environment teardown complete"

# Mock generation
.PHONY: mockgen-install
mockgen-install:
	go install go.uber.org/mock/mockgen@latest

.PHONY: generate-mocks
generate-mocks:
	@echo "Generating mocks..."
	@which mockgen > /dev/null || $(MAKE) mockgen-install
	@mkdir -p internal/argocd/client/mock
	@mockgen -source=internal/argocd/client/interface.go -destination=internal/argocd/client/mock/mock_client.go -package=mock
	@echo "Mocks generated successfully"

.PHONY: clean-mocks
clean-mocks:
	@echo "Cleaning generated mocks..."
	@rm -rf internal/argocd/client/mock
	@echo "Mocks cleaned"

# Testing
.PHONY: test
test:
	$(GOTEST) ./...

.PHONY: test-verbose
test-verbose:
	$(GOTEST) -v ./...

.PHONY: test-race
test-race:
	$(GOTEST) -race ./...

.PHONY: test-cover
test-cover:
	$(GOTEST) -cover ./...

.PHONY: test-coverprofile
test-coverprofile:
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

.PHONY: test-pretty
test-pretty:
	@which gotestsum > /dev/null || go install gotest.tools/gotestsum@latest
	gotestsum --format testname -- ./...

.PHONY: test-watch
test-watch:
	@which gotestsum > /dev/null || go install gotest.tools/gotestsum@latest
	gotestsum --watch --format testname -- ./...

.PHONY: test-coverage-pretty
test-coverage-pretty:
	@which gotestsum > /dev/null || go install gotest.tools/gotestsum@latest
	gotestsum --format testname -- -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run E2E tests with parallel execution for independent tests
.PHONY: e2e-test
e2e-test: check-env
	@echo "Running E2E tests with parallel execution (up to 8 concurrent tests)..."
	ARGOCD_SERVER=$(ARGOCD_SERVER) \
	ARGOCD_AUTH_TOKEN=$(ARGOCD_AUTH_TOKEN) \
	ARGOCD_INSECURE=$(ARGOCD_INSECURE) \
	go test -v -run TestRealArgoCD_Suite ./test/argocd_e2e -parallel 8 -timeout 30m

# Complete E2E test flow: setup, test, teardown
.PHONY: e2e
e2e: e2e-setup e2e-test e2e-teardown

# Show cluster info
.PHONY: cluster-info
cluster-info: check-context
	@echo "Cluster: $(CLUSTER_NAME)"
	@$(KUBECTL) cluster-info --context kind-$(CLUSTER_NAME)
	@echo "\nArgoCD pods:"
	@$(KUBECTL) get pods -n $(ARGOCD_NAMESPACE)

# Help
.PHONY: help
help:
	@echo "ArgoCD MCP Server Makefile"
	@echo ""
	@echo "Build & Run:"
	@echo "  build              - Build the binary"
	@echo "  clean              - Remove binary and clean cache"
	@echo "  deps               - Download and verify dependencies"
	@echo "  tidy               - Run go mod tidy"
	@echo "  run                - Build and run the binary"
	@echo "  install            - Install the binary"
	@echo ""
	@echo "Linting:"
	@echo "  fmt                - Format code"
	@echo "  fmt-check          - Check if code needs formatting"
	@echo "  vet                - Run go vet"
	@echo "  lint               - Run all linters (fmt-check, vet, staticcheck, golint, etc.)"
	@echo "  lint-basic         - Run basic linters (fmt-check, vet)"
	@echo "  lint-install       - Install all linter tools"
	@echo "  lint-advanced      - Run all linters including security checks"
	@echo ""
	@echo "Testing:"
	@echo "  test               - Run tests"
	@echo "  test-verbose       - Run tests with verbose output"
	@echo "  test-race          - Run tests with race detector"
	@echo "  test-cover         - Run tests with coverage"
	@echo "  test-coverprofile  - Generate coverage report"
	@echo "  test-pretty        - Run tests with pretty output (gotestsum)"
	@echo "  test-watch         - Run tests in watch mode"
	@echo "  test-coverage-pretty - Generate coverage report with gotestsum"
	@echo ""
	@echo "Quality Checks:"
	@echo "  check              - Run lint and test"
	@echo "  check-all          - Run all checks including advanced linting and race detection"
	@echo ""
	@echo "E2E Testing:"
	@echo "  e2e-setup          - Create Kind cluster, install ArgoCD, and generate .env"
	@echo "  e2e-teardown       - Delete Kind cluster"
	@echo "  e2e-test           - Run E2E tests with parallel execution (default)"
	@echo "  e2e                - Run complete E2E flow (setup, test, teardown)"
	@echo ""
	@echo "E2E Utilities:"
	@echo "  kind-create        - Create Kind cluster named '$(CLUSTER_NAME)'"
	@echo "  kind-delete        - Delete Kind cluster"
	@echo "  install-argocd     - Install ArgoCD $(ARGOCD_VERSION) in the cluster"
	@echo "  generate-env       - Generate .env file with ArgoCD credentials"
	@echo "  generate-token     - Generate ArgoCD token via CLI"
	@echo "  generate-token-api - Generate ArgoCD token via API"
	@echo "  argocd-password    - Get ArgoCD admin password"
	@echo "  argocd-port-forward - (DEPRECATED) Port forward ArgoCD to localhost:8080"
	@echo "  cluster-info       - Show cluster and ArgoCD pod information"
	@echo ""
	@echo "  help               - Show this help message"

.DEFAULT_GOAL := help
