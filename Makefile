BINARY := bin/gateway
GOFLAGS := -ldflags="-s -w"
GOCMD := go
GOPATH := $(shell $(GOCMD) env GOPATH)

.PHONY: build test lint clean ui-build ui-dev docker-build check-structure

build: ui-build
	@mkdir -p bin
	@$(GOCMD) build $(GOFLAGS) -o $(BINARY) ./src/cmd/gateway/
	@echo "built $(BINARY)"

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

docker-build:
	@if command -v docker >/dev/null 2>&1; then \
		docker build -t maskchain/gateway:latest .; \
	else \
		echo "docker not found. Please install Docker first."; \
		exit 1; \
	fi

check-structure:
	@for dir in src/cmd/gateway src/internal/domain src/internal/app src/internal/ports src/internal/adapters src/internal/infra src/internal/api src/pkg ui specs/active specs/archived deployments/docker-compose docs examples bin; do \
		if [ ! -d "$$dir" ]; then echo "MISSING: $$dir"; exit 1; fi; \
	done
	@echo "structure OK"
