# Changelog

## [Unreleased] — v1.0.0-alpha

### Added
- Content Shield: PII/PHI/financial/secrets regex detection, dictionary masking (exact/contains/regex/fuzzy), streaming SSE unmask
- LLM Provider Routing: OpenAI, Anthropic, Gemini, Bedrock, Ollama, OpenAI-compatible proxy
- Circuit breaker with automatic fallback
- Per-tenant isolation with API key auth
- Rate limiting (sliding window, Valkey-backed) + token budgets
- Cost tracking and usage analytics
- Virtual keys: DB-first scoped API keys (per-tenant, allowed/blocked models, budget cap, expiry, metadata) with admin CRUD + UI; YAML keys bootstrap into virtual keys on startup
- Budget enforcement: Budget entity (scope tenant|key|model, monthly|daily|custom), Valkey spend counters, hard-limit 429 via BudgetMiddleware, soft-limit webhook alerts, spend history + aggregation, admin CRUD + dashboard UI
- Session tracking with TTL-based cleanup
- Admin management API + React operator console (tenants, dictionaries, keys, budgets, routing, compliance, analytics, settings)
- Hot-reload configuration (YAML + ENV + CLI flags); config-vs-file diff in Settings
- OpenTelemetry tracing (gRPC exporter) + Prometheus metrics
- Helm chart for Kubernetes deployment
- Docker multi-stage images (distroless, ~18 MB gateway binary)

#### UI v2 — operator console
- Dark-first design system: token layers, status colors, reusable primitives, sidebar grouped by job (Overview/Traffic/Governance/Operations/System), workspace switcher, contextual header actions
- Operations HQ dashboard: KPI cards with delta vs previous period + sparkline, trend chart with Tokens/Cost/Requests toggle, "needs attention" panel, first-run onboarding
- Analytics: persisted time range, compare vs previous period, day/model/tenant breakdown tabs, CSV export honoring range + tenant scope
- Virtual Keys page: endpoint quickstart (base URL + curl), create-key drawer (tenant, model chips, budget, expiry presets), spend/budget progress, enable/disable switch, search & filters, one-time key reveal
- Routing page: provider health cards (status, latency, last check) with fallback indicators, routing rules and cost-rate tables
- Tenant profile: Overview / Policies & Shield / Compliance / Activity tabs, compliance-deviation banner with re-apply
- Settings page: live system status served by a new read-only `GET /api/v1/admin/status`, no hardcoded values

#### Masking & data protection
- Mask clean tokens: `/mask` defaults to `[MASK.N]` counter tokens (no document id), `format=id` keeps legacy `[MASK_<id>.<N>]`, `format=redact` is non-reversible `[REDACTED]`
- `mask_entries.reversible` column; unmask of a redact entry returns 400
- `/unmask` applies ids sequentially per document; legacy entries remain restorable
- Proxy/conversation dictionary tokens are clean per-request counters; PII remains non-reversible `[[pii.<label>.<N>]]`
- At-rest encryption (fail-closed): `MASKCHAIN_KEYS_KEY` required when DB-backed routing/tenancy is enabled; provider secrets AES-256-GCM
- Compliance packs apply/report with deviation detection
- Conversation logging: encrypted payloads with list/detail UI and mask-proof view
- Analytics tenant scoping: optional `tenant` query param on `/api/v1/analytics/*`
- Swagger UI (`/api/v1/docs`) now served by the standalone admin binary too

### Changed
- Migrated logging from `go.uber.org/zap` to `log/slog` (stdlib) with OTel enrichment
- Replaced `panic()` calls in UUID generator with proper error returns
- Replaced `log.Fatal` in library code with error returns to caller
- `/api/v1/shield/mask` default output switched to clean tokens (no document id); legacy shape available via `format=id`
- UI codebase moved to v2 tokens/primitives; dates/times use locale-neutral formatting throughout
- Constitution v1.4: React UI is the operator control-plane console (not end-user chat UI)

### Fixed
- File no longer uses `context.Background()` in critical paths
- `nil` Context passed to `Save()` in test (`mask_handler_test.go`)
- Empty branch linter warnings in `tenant_handler.go` and `provider_handler_test.go`
- Pre-allocation linter warnings in provider adapters (gemini, bedrock, OpenAI)
- `gofmt` formatting across 40+ files
- Settings showed hardcoded values; now rendered from live system status
- Swagger UI 404 in the standalone admin binary; `/api/v1/docs` registered

### Security
- `make security-check`: gitleaks secrets scan + TLS lint + config audit
- `.dockerignore` hardened (30 entries), nonroot containers
