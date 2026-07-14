# MaskChain

Content shield proxy — PII/PHI/financial/secrets detection, dictionary masking, tenant isolation, and LLM provider routing with circuit breaker.

## Architecture

```
Client → Auth → RateLimit → Shield Scan → Routing → Provider (OpenAI/Anthropic)
                                            ↓
                                    Fallback + Circuit Breaker
```

- **Gateway** — proxy for LLM chat completions with content scanning
- **Admin** — management API + SPA (React/Vite) for tenant/dictionary/incident management
- **PostgreSQL** — tenants, incidents, profiles, mask storage
- **Valkey** — rate limiting (sliding window), mask cache

## Quick Start

```bash
# Start full stack
docker compose -f deployments/docker-compose/docker-compose.yml up -d --build

# Or use examples/ for local dev
docker compose -f examples/docker-compose.yml up -d --build
```

See [examples/README.md](examples/README.md) for tenant setup and test flows.

## Services

| Service | Port | Description |
|---------|------|-------------|
| Gateway | 8080 | LLM proxy + shield scan |
| Admin | 8081 | Management API + UI |
| PostgreSQL | 5432 | Primary store |
| Valkey | 6379 | Rate limit + cache |
| Prometheus | 9090 | Metrics (examples stack) |
| Grafana | 3000 | Dashboards (examples stack) |

## Configuration

Three-layer config: YAML → ENV (`CONFIG_*`) → CLI flags.

Minimal `config.yaml`:

```yaml
server:
  port: 8080
routing:
  providers:
    - name: openai
      api_type: openai
      base_url: https://api.openai.com
      api_keys: ["sk-..."]
tenants:
  default:
    auth_header: "Authorization"
    api_keys: ["sk-test-default"]
```

See `examples/config.yaml` for full reference.

## Project Structure

```
src/
├── cmd/
│   ├── gateway/          # Gateway entrypoint
│   ├── admin/            # Admin API + UI entrypoint
│   └── internal/bootstrap/  # Shared init (DB, Valkey, logger)
├── internal/
│   ├── adapters/         # External integrations (providers, repos, egress)
│   ├── api/              # HTTP layer (handlers, middleware, DTOs)
│   ├── app/usecase/      # Application use cases (shield scan)
│   ├── domain/           # Domain logic (shield, routing, tenant, budget)
│   ├── infra/            # Infrastructure (config, telemetry, metrics)
│   └── ports/            # Interface definitions
└── pkg/                  # Public packages
```

## Key Features

### Content Shield
- PII/PHI/financial/secrets regex detection
- Dictionary-based entity matching per tenant
- Placeholder masking with restore via admin API
- Streaming response unmask (SSE)

### Routing
- Per-tenant per-model provider routing
- Automatic fallback + circuit breaker
- Provider health checking

### Observability
- OpenTelemetry tracing (gRPC exporter)
- Prometheus metrics (request rate, latency, shield stats, pool stats)
- Structured logging (zap) with trace IDs

## Development

```bash
make build       # Build both binaries
make test        # Run all tests
make lint        # Run golangci-lint
make security-check  # gitleaks + config audit
```

Build tags: `gateway` / `admin` — split by binary capabilities.

## API

OpenAPI 3.1 spec: `docs/openapi.yaml`
Swagger UI: embedded in admin binary at `/swagger/`.

## License

See LICENSE file.
