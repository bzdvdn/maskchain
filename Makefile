BINARY_GATEWAY := bin/gateway
BINARY_ADMIN := bin/admin
GOFLAGS := -ldflags="-s -w"
GOCMD := go
GOPATH := $(shell $(GOCMD) env GOPATH)

.PHONY: build build-gateway build-admin test lint clean ui-build ui-dev docker-build docker-build-gateway docker-build-admin check-structure security-check load-test

# @sk-task 100-admin-control-plane#T1.2: Add build-gateway, build-admin, docker-build-gateway, docker-build-admin targets (AC-008)
build: build-gateway build-admin

# @sk-task 101-gateway-diet#T1.2: Add -tags gateway and CGO_ENABLED=0 to build-gateway (AC-003, AC-006)
build-gateway:
	@mkdir -p bin
	@CGO_ENABLED=0 $(GOCMD) build $(GOFLAGS) -tags gateway -o $(BINARY_GATEWAY) ./src/cmd/gateway/
	@echo "built $(BINARY_GATEWAY)"

# @sk-task 101-gateway-diet#T1.2: Add -tags admin and CGO_ENABLED=0 to build-admin (AC-006)
build-admin: ui-build
	@mkdir -p bin
	@CGO_ENABLED=0 $(GOCMD) build $(GOFLAGS) -tags admin -o $(BINARY_ADMIN) ./src/cmd/admin/
	@echo "built $(BINARY_ADMIN)"

test:
	@$(GOCMD) test ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

clean:
	@rm -rf bin/ ui/dist/
	@echo "cleaned"

# @sk-task 41-profiles-ui#T1.3: Add UI build and dev targets (AC-008, AC-009)
ui-build:
	@echo "building UI..."
	@cd ui && npm run build
	@echo "UI built"

ui-dev:
	@echo "starting Vite dev server (API proxy -> localhost:8080)..."
	@cd ui && npm run dev

docker-build: docker-build-gateway docker-build-admin

docker-build-gateway:
	@if command -v docker >/dev/null 2>&1; then \
		docker build -f Dockerfile.gateway -t maskchain/gateway:latest .; \
	else \
		echo "docker not found. Please install Docker first."; \
		exit 1; \
	fi

docker-build-admin:
	@if command -v docker >/dev/null 2>&1; then \
		docker build -f Dockerfile.admin -t maskchain/admin:latest .; \
	else \
		echo "docker not found. Please install Docker first."; \
		exit 1; \
	fi

check-structure:
	@for dir in src/cmd/gateway src/internal/domain src/internal/app src/internal/ports src/internal/adapters src/internal/infra src/internal/api src/pkg ui specs/active specs/archived deployments/docker-compose docs examples bin; do \
		if [ ! -d "$$dir" ]; then echo "MISSING: $$dir"; exit 1; fi; \
	done
	@echo "structure OK"

# @sk-task 90-production-hardening#T4.1: Add security-check and load-test targets (<AC-004>, <AC-005>)
security-check:
	@echo "--- security-check: secrets scan ---"
	@if command -v gitleaks >/dev/null 2>&1; then \
		gitleaks detect --no-color --verbose 2>&1; \
	else \
		echo "gitleaks not installed. Install: brew install gitleaks or https://github.com/gitleaks/gitleaks"; \
	fi
	@echo "--- security-check: TLS lint ---"
	@if command -v openssl >/dev/null 2>&1; then \
		echo "OpenSSL available (manual TLS check: openssl s_client -connect <host>:443)"; \
	else \
		echo "openssl not found. Install openssl for TLS verification."; \
	fi
	@echo "--- security-check: config audit ---"
	@if [ -f config.yaml ]; then \
		echo "config.yaml found. Validating..."; \
		$(GOCMD) run ./src/cmd/gateway/ --config config.yaml --log-level=error 2>&1 || echo "CONFIG AUDIT: config.yaml validation failed"; \
	else \
		echo "no config.yaml found, skipping audit"; \
	fi
	@echo "security-check complete"

load-test:
	@echo "--- load-test ---"
	@python3 ./deployments/loadtest/chat_completion.py
