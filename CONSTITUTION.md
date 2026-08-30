# MaskChain Project Constitution

## Purpose

Building a production-grade platform for routing, securing (Content Shield) and managing AI traffic with tenant-level security policy management.

The mission is to provide organizations with a single gateway for AI traffic with a built-in Content Shield (AI DLP): detection and reaction to PII, secrets, and financial data in prompts and responses. Tenants are the container of detection policies: dictionaries, PII rules, and preprocessors are configured directly on the tenant.

## Core Principles

### I. Content Shield — Core Domain

Content Shield (AI DLP) is the core domain of the system, not an add-on feature. The gateway MUST intercept AI requests and responses and analyze them for:
- PII (personal data: email, phone, SSN, passport data)
- Secrets (API keys, private keys, JWT, tokens)
- Financial data (card numbers with Luhn, IBAN, SWIFT)
- Protected health information (PHI)

Reactions on detection: block, redact, mask, alert. Policies are configured through tenants.

### II. Tenant-Driven Policy Management

Content Shield policies are managed through tenants — each tenant contains dictionaries, PII rules (piiConfig), and preprocessors. Tenants are stored in PostgreSQL, managed via REST API and React UI. Dictionary profiles are removed; all policy configuration is encapsulated in the tenant.

### III. Infrastructure, Not Chatbot

The project is an infrastructure gateway, not a chatbot, prompt playground, AI IDE, or low-code workflow. Focus: networking, security, traffic management, policy enforcement, observability.

### IV. AI Traffic Is Network Traffic

AI requests are handled as HTTP/gRPC traffic with an understanding of AI semantics: token economics, prompt semantics, provider health, inference latency, model capabilities, compliance.

### V. Runtime Before Platform

The gateway runtime must exist before Kubernetes abstractions. Development order: Runtime → Routing → Shield → Policies → Egress → API → UI → Operator. Envoy mode is PostMVP.

### VI. Native-Only Data Plane (MVP)

The current data plane is the built-in Go runtime (native). One binary: Gin HTTP server + `http.Client` + egress dialers. No external dependencies for request processing. Envoy mode is PostMVP (not planned until native mode is stabilized).

### VII. Local Development Matters

Every major feature MUST be runnable locally via Docker Compose. React UI runs via dev mode (Vite/HMR) or as a compose service. Native runtime mode is the primary deployment for local development.

### VIII. Streaming — Mandatory Requirement

SSE, chunk forwarding, streaming retries, cancellation propagation, low-latency token delivery. Streaming MUST be stable across retries, failover, and observability.

### IX. Observability Is Mandatory

Every request MUST be observable: distributed traces, metrics, structured logs, token accounting, shield visibility, provider health. No black boxes.

### X. Extensibility Over Hardcoding

Preference: plugins, interfaces, adapters, declarative policies, extensible detectors for Content Shield. Avoid: provider-specific hacks, giant monolith logic, tightly coupled integrations.

## Non-Negotiable Rules

- Implementation `MUST` follow active spec/plan/tasks and stay within the stated scope.
- Work `MUST NOT` continue from ambiguous requirements or placeholder content.
- Changes to public behavior `MUST` be reflected in spec/tasks before merge.
- If implementation conflicts with the constitution, the constitution is updated first.

## Constraints

- Content Shield is a mandatory gateway capability, not opt-in.
- Tenants (dictionaries, PII rules, preprocessors) are stored in PostgreSQL; Valkey is only for caching.
- React UI is the operator control-plane console (dashboards, analytics, virtual keys, budgets, routing health, compliance, tenant policies, settings/status, audit); it is not an end-user chat/playground UI.
- Dictionary profiles are removed and MUST NOT return; all policy configuration lives at the tenant level.
- Envoy mode is PostMVP; native mode is the only option until core domains are stabilized.
- No chatbot UI, no prompt playground, no agent framework, no low-code platform.
- The system MUST run in enterprise networks with outbound proxy and air-gapped environments.
- The gateway and a possible future Operator are separate components.
- Every major feature MUST be runnable locally (Docker Compose).

## Tech Stack

- **Language:** Go (backend)
- **Web framework:** Gin
- **Config:** viper + cobra
- **Architecture:** DDD, Clean Architecture (ports/adapters)
- **UI:** React (TypeScript, Vite) — operator control-plane console
- **Data Plane:** Native (Go, in-process). Envoy — PostMVP.
- **Content Shield / AI DLP:** Microsoft Presidio (PII detection), custom patterns engine (secrets, API keys, financial data)
- **Observability:** OpenTelemetry, Prometheus, Grafana, Loki, Tempo
- **Persistence:** PostgreSQL (tenants, audit, incidents)
- **Cache:** Valkey (Redis-compatible)
- **Local development:** Docker Compose, mock providers

## Core Architecture

```
Client → Gateway Runtime → Shield Engine → Routing → Egress → AI Providers
                            ↕
                   Tenant Repository (PG)
                            ↕
                    React UI — Operator Console
```

### Gateway Runtime
HTTP API, streaming, retries, failover, routing execution, observability. In-process Go: Gin HTTP server, `http.Client` with egress dialer wrappers, in-process SSE streaming. Single binary, zero external dependencies.

### Shield Engine
Content inspection pipeline: PII redaction, secrets detection, AI DLP. Inspects inbound prompts and outbound responses. Driven by tenant configuration (dictionaries, PII rules) through the Tenant Repository.

### Tenant Repository
Tenant storage for Content Shield:
- Tenants: named policy containers
- Dictionaries: entity lists for precise detection
- PII Config: regex rules for PII, secrets, financial, PHI detection
- Preprocessors: CSV/JSON preprocessors for structured data
- Reactions: block, redact, mask, alert

### Routing Engine
Model selection, provider selection, fallback decisions, semantic routing.

### Egress Engine
Outbound proxy routing, retry orchestration, timeout management.

### Observability Layer
OpenTelemetry, Prometheus, structured logging, distributed tracing.

### React UI — Operator Control-Plane Console
Operator console for LLM gateway operations: Operations HQ (KPI + needs-attention), analytics, virtual keys, budgets, routing health/fallbacks, compliance packs + deviation, tenant policies and Dictionaries, live system status, config diff, audit logs.

## Language Policy

- Documentation language: English
- Agent communication language: English
- Code comment language: English

## Development Workflow

- Each feature MUST be developed in a separate git branch.
- Branch naming SHOULD follow `feature/<slug>`.
- Implementation SHOULD start with an explicit specification before coding.
- Plans and tasks SHOULD derive from the current specification and stay consistent with it.
- Implementation, specifications, plans, and tasks MUST comply with this constitution.
- If work reveals a conflict with this constitution, the constitution MUST be changed before continuing incompatible implementation.

## Definition of Done

- A task is complete only with observable proof: changed files, output of targeted tests, or command result.
- For non-trivial edits, traceability markers are mandatory:
  - code: `@sk-task <slug>#<TASK_ID>: <short> (<AC_ID>)`
  - tests: `@sk-test <slug>#<TASK_ID>: <TestName> (<AC_ID>)`
  - if several tests/cases confirm one task, `@sk-test <slug>#<TASK_ID>` MUST be placed on each such test/case, not only on one representative test.
- Marker placement rules:
  - The marker is ALWAYS placed **above the declaration** of the symbol it belongs to.
  - If the symbol has a GoDoc, marker(s) come **first**, then a **blank line** (`//`), then GoDoc, then the declaration.
  - If there is no GoDoc, the marker is placed directly above the declaration.
  - Placing trace markers at `package`, `import`, or file-header level is forbidden.
  - A marker always refers to the symbol below; if there are several markers, all refer to the same symbol.
- Placement and style examples by language:
  - Go:
    ```go
    // @sk-task slug#T1: description (AC-001)
    //
    // FunctionDoc describes what this function does.
    func DoSomething() { ... }

    // @sk-task slug#T2: description (AC-002)
    type Config struct { ... }

    // @sk-test slug#T1: TestSomething (AC-001)
    //
    // TestSomething verifies behavior X.
    func TestSomething(t *testing.T) { ... }
    ```
    Without GoDoc:
    ```go
    // @sk-task slug#T1: description (AC-001)
    func Short() { ... }
    ```
    If several `Test...` verify one task, `@sk-test` is required on each such test.
  - Python: `#` as the first line inside the body of `def` / `async def` / `class` / `def test_*`; not in module docstring and not above `import`. If one task is covered by multiple test functions, the marker is required inside each of them.
  - JavaScript / TypeScript: `//` above `function`, `async function`, class method, `class`; for `test(...)`/`it(...)` — as the first line inside the callback/body. If there are multiple cases, the marker is required in each `test/it`.
  - Shell / Bash: `#` above `function name()` or the first line of a named behavior/test block; not in a file header solely for trace purposes.
  - SQL / migrations: `--` or `/* */` directly above `CREATE FUNCTION|PROCEDURE|TRIGGER|VIEW` or the first line of an explicitly named migration block; not in a file-header comment without a tie to the change.
- Existing trace markers are preserved; coverage of a new task is added with additional markers (without overwriting).
- If one method/test covers multiple tasks, multiple markers remain on it simultaneously.
- Before archive, coverage of acceptance criteria MUST be confirmed in verify.

## Repository Map Policy

- `REPOSITORY_MAP.md` is a compact code navigation index, not a process document.
- The map is updated only on significant changes to code structure/navigation.
- Updates are made in-place with a minimal diff; unchanged sections are not rewritten.
- Operational/spec artifacts are excluded from indexing per project policy.

## DDD and Clean Architecture

### Domain Boundaries

Core domains: shield (content security, PII detection, secrets detection, dictionaries, preprocessors), routing, providers, egress, streaming, observability, tenants. Dictionary profiles are removed; all policies are configured on the tenant. Architecture is organized around domains, not around providers.

### Clean Architecture

Follow ports/adapters, dependency inversion, explicit boundaries, isolated domain logic. Domain logic MUST NOT depend on HTTP frameworks, databases, Redis, or React. All of that lives behind ports.

## Repository Structure

```
/src
    cmd/
        gateway/      — gateway runtime entrypoint
    internal/
        domain/       — domain logic (business entities, value objects, domain services)
        app/          — application layer (use cases, application services)
        ports/        — port interfaces (inbound/outbound)
        adapters/     — adapter implementations (providers, persistence, shield detectors)
        infra/        — infrastructure (config, logging, metrics, tracing)
        api/          — API handlers and middleware
    pkg/              — reusable public libraries

/ui                   — React frontend (Vite + TypeScript)

/specs
    active/           — active specifications
    archived/         — archived specifications

/deployments
    docker-compose/   — local environments
    kubernetes/       — manifests (PostMVP)

/docs
    en/               — documentation in English
    ru/               — documentation in Russian
    architecture/     — architecture documents
    adr/              — Architecture Decision Records

/examples             — usage examples
/bin                  — build artifacts
```

## Governance

- This constitution is the authoritative source for project decisions.
- Changes to architecture, specifications, plans, and tasks MUST comply with these principles.
- If implementation conflicts with the constitution, the constitution takes priority until explicitly changed.
- Change this file via patch updates, preserving mandatory sections and keeping rules concrete and testable.

## Constitution Metadata

- Version: 1.4.0
- Ratified: 2026-07-10
- Last Amended: 2026-08-30

## Last Updated

2026-07-10 — initial MaskChain constitution. Focus: Content Shield (AI DLP), dictionary profiles, native-only data plane, React UI.
2026-07-15 — v1.1.0: dictionary profiles removed, policies configured on the tenant (dictionaries, PII rules, preprocessors).
2026-07-20 — v1.2.0: GoDoc clean — marker placement rule: `@sk-task` → blank line → GoDoc → declaration. Marker always above the declaration, GoDoc separated by a blank line below.
2026-08-13 — v1.3.0: full English translation per language policy (docs=en, agent=en, comments=en).
2026-08-30 — v1.4.0: React UI is the operator control-plane console (dashboards, analytics, keys, budgets, routing, compliance, settings, audit), not an end-user chat/playground UI.