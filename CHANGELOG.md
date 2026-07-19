# Changelog

## [Unreleased] — v1.0.0-alpha

### Added
- Content Shield: PII/PHI/financial/secrets regex detection, dictionary masking (exact/contains/regex/fuzzy), streaming SSE unmask
- LLM Provider Routing: OpenAI, Anthropic, Gemini, Bedrock, Ollama, OpenAI-compatible proxy
- Circuit breaker with automatic fallback
- Per-tenant isolation with API key auth
- Rate limiting (sliding window, Valkey-backed) + token budgets
- Cost tracking and usage analytics
- Session tracking with TTL-based cleanup
- Admin management API + React SPA (profiles, incidents, tenants, dictionaries)
- Hot-reload configuration (YAML + ENV + CLI flags)
- OpenTelemetry tracing (gRPC exporter) + Prometheus metrics
- Helm chart for Kubernetes deployment
- Docker multi-stage images (distroless, ~18 MB gateway binary)

### Changed
- Migrated logging from `go.uber.org/zap` to `log/slog` (stdlib) with OTel enrichment
- Replaced `panic()` calls in UUID generator with proper error returns
- Replaced `log.Fatal` in library code with error returns to caller

### Fixed
- File no longer uses `context.Background()` in critical paths
- `nil` Context passed to `Save()` in test (`mask_handler_test.go`)
- Empty branch linter warnings in `tenant_handler.go` and `provider_handler_test.go`
- Pre-allocation linter warnings in provider adapters (gemini, bedrock, OpenAI)
- `gofmt` formatting across 40+ files

### Security
- `make security-check`: gitleaks secrets scan + TLS lint + config audit
- `.dockerignore` hardened (30 entries), nonroot containers
