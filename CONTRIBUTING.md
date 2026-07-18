# Contributing to MaskChain

## Local Development Setup

Requirements:
- Go 1.26.3+
- Docker & Docker Compose
- Node.js 20+ (for admin UI)
- golangci-lint (optional, used by `make lint`)

```bash
# Clone and build
git clone https://github.com/bzdvdn/maskchain.git
cd maskchain
make build

# Run tests
make test

# Start dev stack
docker compose -f examples/docker-compose.yml up -d
```

## Running Tests

```bash
make test          # go test -race -count=1 -coverprofile=coverage.out ./...
make lint          # golangci-lint run ./...
```

All tests run with the race detector enabled. Integration tests require a running Docker stack:

```bash
# Start dependencies, then run:
docker compose -f deployments/docker-compose/docker-compose.yml up -d postgres valkey
go test -race -count=1 -tags=integration ./test/integration/...
```

## Build Tags

| Tag      | Binary     | Includes                         |
|---------|------------|----------------------------------|
| `gateway` | `bin/gateway` | LLM proxy, shield scan, routing |
| `admin`   | `bin/admin`   | Admin API, UI embed, CRUD       |
| (none)    | `bin/maskchain` | Combined (gateway + admin)    |

Build with: `CGO_ENABLED=0 go build -tags gateway -o bin/gateway ./src/cmd/gateway/`

The admin binary requires the UI to be built first (`make ui-build`).

## Adding a New Provider

1. Define provider config struct in `src/internal/domain/routing/`
2. Implement the `ProviderClient` interface in `src/internal/adapters/provider/`
3. Register the provider factory in the routing adapter switch statement
4. Add provider config to the YAML config spec in `examples/config.yaml`
5. Add tests in `src/internal/adapters/provider/` and `src/internal/app/usecase/`

Example: see existing OpenAI and Anthropic implementations in `src/internal/adapters/provider/`.

## Code Style

- **Domain-Driven Design** — domain logic lives in `src/internal/domain/` with zero external dependencies
- **Clean Architecture** — dependency direction: domain <- usecase <- adapters <- infrastructure
- **Ports & Adapters** — interfaces in `src/internal/ports/`, implementations in `src/internal/adapters/`
- Error wrapping with `fmt.Errorf("context: %w", err)`
- Structured logging via zap, no `fmt.Print` in production code
- Tests follow table-driven pattern with `t.Parallel()`

## PR Checklist

- [ ] `make lint` passes (golangci-lint + go vet)
- [ ] `make test` passes with race detector
- [ ] New code includes tests
- [ ] API changes include OpenAPI spec update in `src/internal/api/swagger/openapi.yaml`
- [ ] Config changes documented in `examples/config.yaml`
- [ ] Migration added for new DB entities (if applicable)
- [ ] Commits follow conventional commit style (`feat:`, `fix:`, `chore:`, etc.)

## Building Docker Images

```bash
make docker-build-gateway   # maskchain/gateway:latest
make docker-build-admin     # maskchain/admin:latest
make docker-build-combined  # maskchain/combined:latest
```

Multi-stage distroless builds. See `Dockerfile.gateway`, `Dockerfile.admin`, `Dockerfile.combined`.

## Security Reporting

Report security vulnerabilities by opening a GitHub Issue (do not include sensitive details in public issues — email the maintainer directly if the issue is sensitive). See [SECURITY.md](SECURITY.md).

## Production Runbook

See [deployments/runbook.md](deployments/runbook.md) for production operations, debugging procedures, and recovery steps.

## Code of Conduct

Be respectful, constructive, and inclusive. Focus on the code, not the person.
