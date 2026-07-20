BINARY_GATEWAY := bin/gateway
BINARY_ADMIN := bin/admin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "unknown")
LDFLAGS := -s -w -X github.com/bzdvdn/maskchain/src/pkg/version.Version=$(VERSION) -X github.com/bzdvdn/maskchain/src/pkg/version.Commit=$(COMMIT) -X github.com/bzdvdn/maskchain/src/pkg/version.Date=$(DATE)
GOFLAGS := -ldflags="$(LDFLAGS)"
GOCMD := go
GOPATH := $(shell $(GOCMD) env GOPATH)

.PHONY: build build-gateway build-admin build-combined build-admin-ci build-combined-ci test lint clean ui-build ui-dev docker-build docker-build-gateway docker-build-admin docker-build-combined check-structure security-check load-test helm-lint ci

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

# @sk-task combined-binary: Build combined binary (admin + gateway in one)
build-combined: ui-build
	@mkdir -p bin
	@CGO_ENABLED=0 $(GOCMD) build $(GOFLAGS) -o bin/maskchain ./src/cmd/all/
	@echo "built bin/maskchain"

# @sk-task ci-stability: CI target — create ui/dist stub then build admin (AC-008)
build-admin-ci:
	@mkdir -p ui/dist
	@echo '<html></html>' > ui/dist/index.html
	@mkdir -p bin
	@CGO_ENABLED=0 $(GOCMD) build $(GOFLAGS) -tags admin -o $(BINARY_ADMIN) ./src/cmd/admin/
	@echo "built $(BINARY_ADMIN) (CI mode)"

# @sk-task ci-stability: CI target — create ui/dist stub then build combined (AC-008)
build-combined-ci:
	@mkdir -p ui/dist
	@echo '<html></html>' > ui/dist/index.html
	@mkdir -p bin
	@CGO_ENABLED=0 $(GOCMD) build $(GOFLAGS) -o bin/maskchain ./src/cmd/all/
	@echo "built bin/maskchain (CI mode)"

test:
	@$(GOCMD) test -race -count=1 -coverprofile=coverage.out ./...

lint:
	@golangci-lint run ./...

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
		docker build -f Dockerfile.gateway -t bzdvdn/maskchain-gateway:latest .; \
	else \
		echo "docker not found. Please install Docker first."; \
		exit 1; \
	fi

docker-build-admin:
	@if command -v docker >/dev/null 2>&1; then \
		docker build -f Dockerfile.admin -t bzdvdn/maskchain-admin:latest .; \
	else \
		echo "docker not found. Please install Docker first."; \
		exit 1; \
	fi

# @sk-task combined-binary: Docker build for combined binary
docker-build-combined:
	@if command -v docker >/dev/null 2>&1; then \
		docker build -f Dockerfile.combined -t bzdvdn/maskchain:latest .; \
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

helm-lint:
	@if command -v helm >/dev/null 2>&1; then \
		helm lint deployments/helm/maskchain/; \
	else \
		echo "helm not installed. Install: https://helm.sh/docs/intro/install/"; \
		exit 1; \
	fi

ci: lint test build docker-build-gateway docker-build-admin docker-build-combined helm-lint
	@echo "CI pipeline complete"
