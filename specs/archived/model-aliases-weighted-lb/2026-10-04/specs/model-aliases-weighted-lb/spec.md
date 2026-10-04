# Model Aliases & Weighted Load Balancing

## Scope Snapshot

- In scope: per-tenant model aliases (a requested model name resolves to a routed model) and load balancing across healthy providers by priority tier and weight.
- Out of scope: key-level aliases, request-level routing overrides, latency/least-busy/adaptive strategies, A/B tests and traffic mirroring, provider-format translation.

## Goal

Operators can (a) publish stable model names that map to whatever backend model a tenant should actually use, and (b) spread traffic across several equivalent providers instead of always using the first one. Today aliases are not applied anywhere and the routing selector is strictly ordered: only the first healthy provider is ever chosen, so `priority` and any weight have no effect. Success is visible when a request for an alias reaches its mapped model, and traffic across equally-prioritized providers with weights is distributed in proportion to those weights.

## Primary User Flow

1. Starting point: a tenant alias `gpt-4o → openai/gpt-4o-2024` and providers `openai`/`azure` with weights configured.
2. Main interaction: a client calls a model-bearing endpoint with `model: "gpt-4o"`.
3. Outcome: the alias resolves to `openai/gpt-4o-2024`, and the request is served by one of the healthy providers in the highest-priority tier, chosen by weight.
4. Failure/fallback path: if the chosen provider fails (retryable/5xx), the request advances to the next provider in the chain; if the alias target has no route, `NO_ROUTE` is returned.

## User Stories

- P1 Story: as an operator, I expose one stable model name per tenant and change the backend model without touching clients.
- P2 Story: as an operator, I run two equivalent providers and let the gateway spread load between them by weight.
- P3 Story: as an operator, I mark a provider as a lower-priority backup so it is used only when the primary tier is unhealthy.

## MVP Slice

- Tenant alias resolution (single hop) applied to the model before routing, and priority-tier + weighted selection among healthy providers. Must satisfy `AC-001`, `AC-003`, `AC-005`, `AC-006`.

## First Deployable Outcome

- A request for an aliased model reaches the mapped upstream model.
- With two same-priority providers weighted 3:1, a controlled selection test shows the weighted split.
- Demonstrable via `GET/PUT` config + a recording mock provider.

## Scope

- Tenant-scoped alias map: `requested model → routed model`, applied before route selection.
- Provider `weight` (non-negative) alongside the existing `priority`.
- Selection across the healthy providers of the minimum `priority` tier using weighted-random by `weight`.
- Alias + weight configurable in YAML at startup and editable in the admin UI.
- Existing ordered behavior preserved when no weights are set.

## Context

- `RouteSelector.Select` (`src/internal/domain/routing/service/selector.go`) returns the first healthy provider from the resolved chain; it ignores the provider `Priority` field.
- The domain `Provider` already carries `Priority` (`health_status.go`); there is no `Weight`.
- A tenant→{requested→routed} mapping field exists but is unused (`ShieldConfig.TenantModelMapping` in `src/internal/infra/config/config.go`); it is the natural home for tenant aliases or may be superseded by a routing-level map.
- Every model-bearing endpoint (chat/completions/embeddings/moderations/rerank/count_tokens) resolves the model through `RouteSelector.Select`.

## Dependencies

- Existing routing registry, selector, fallback handler, and admin routing UI.
- Provider registry (shipped) and wildcard routing (shipped): aliases resolve through the same route table, including wildcard routes.

## Requirements

- RQ-001 A tenant alias map MUST map a requested model name to a routed model name; an absent alias is the identity.
- RQ-002 Alias resolution MUST be tenant-scoped: the same requested name may map differently per tenant.
- RQ-003 The alias target MUST be resolved through the normal route table (exact and wildcard routes, existing precedence); a target with no route yields `NO_ROUTE`.
- RQ-004 A provider MUST accept a non-negative `weight` in configuration, persisted and editable like other provider attributes.
- RQ-005 Selection MUST restrict to the healthy providers of the minimum `priority` tier, then choose among them weighted-random by `weight`.
- RQ-006 When every candidate in the tier has no weight (or equal weight), selection MUST be deterministic and preserve today's ordered behavior (first declared).
- RQ-007 After the selected provider, the remaining candidates MUST form the fallback chain; a failed selected provider MUST advance to the next.
- RQ-008 Configuration and the admin API MUST reject a negative weight and an alias map with an empty target.
- RQ-009 Aliases and weights MUST be settable in YAML at startup and editable in the admin UI, surviving a restart.

## Non-Goals

- Key-level or request-level aliases.
- Latency-, cost-, or least-busy-based routing, adaptive routing, and routing plugins.
- A/B experiments, traffic mirroring, or canary splitting.
- Multi-hop alias chains or alias cycles.
- Changing fallback semantics beyond the selected provider order.

## Acceptance Criteria

### AC-001 Alias resolves the requested model

- Why this matters: clients use a stable name while the backend model changes.
- **Given** a tenant alias `gpt-4o → openai/gpt-4o-2024` and a route for `openai/gpt-4o-2024`
- **When** a client sends `POST /v1/chat/completions` with `model: "gpt-4o"`
- **Then** the request is routed to the provider of `openai/gpt-4o-2024`.
- Evidence: a recording provider shows the request; the response is not `NO_ROUTE`.

### AC-002 Aliases are tenant-scoped

- Why this matters: tenants may map the same name differently.
- **Given** tenant `a` maps `m → a-model` and tenant `b` has no alias for `m`
- **When** each tenant requests model `m`
- **Then** tenant `a` is routed via `a-model` and tenant `b` uses `m`.
- Evidence: per-tenant recording shows distinct upstream models.

### AC-003 Alias target with no route fails explicitly

- Why this matters: a broken alias must not silently pass the original name.
- **Given** an alias `m → missing-model` with no route for `missing-model`
- **When** a client requests `m`
- **Then** the gateway returns `NO_ROUTE` and makes no provider call.
- Evidence: response code/message is `NO_ROUTE`; zero provider calls.

### AC-004 Alias target can be covered by a wildcard route

- Why this matters: aliases and wildcard routing compose.
- **Given** an alias `m → groq/llama-3.3-70b` and a wildcard route `groq/* → groq`
- **When** a client requests `m`
- **Then** the request is routed to `groq`.
- Evidence: the `groq` provider receives the request.

### AC-005 Priority tier is respected

- Why this matters: a provider is a backup unless its tier is needed.
- **Given** healthy providers `primary` (priority 1) and `backup` (priority 5)
- **When** any model is selected
- **Then** `primary` is chosen and `backup` is not, while `primary` is healthy.
- Evidence: `backup` records no calls; when `primary` is marked unhealthy, `backup` is chosen.

### AC-006 Weighted selection follows weights

- Why this matters: operators control proportional load.
- **Given** two healthy same-priority providers with weights `A=3`, `B=1`
- **When** the selection is driven by a controlled random sequence (injected RNG)
- **Then** draws map to `A` for the first 75% of the range and `B` for the rest.
- Evidence: a unit test with a deterministic RNG asserts the exact provider per draw; equal weights split evenly.

### AC-007 No weights keeps the ordered behavior

- Why this matters: existing deployments must not change.
- **Given** same-priority providers with no weights
- **When** a model is selected
- **Then** the first declared provider is chosen deterministically.
- Evidence: repeated selections return the first provider.

### AC-008 Unhealthy providers are excluded

- Why this matters: balancing must not route to a down provider.
- **Given** a weighted provider marked unhealthy
- **When** a model is selected
- **Then** it receives no calls until it becomes healthy.
- Evidence: recording shows no traffic to the unhealthy provider; weight redistributes to the remaining healthy ones.

### AC-009 Aliases and weights are manageable and durable

- Why this matters: operators configure without redeploying.
- **Given** the admin UI/API
- **When** an operator sets an alias or a provider weight
- **Then** it is used immediately and still present after a restart.
- Evidence: config/UI round-trip test; a post-restart read returns the value.

### AC-010 Invalid values are rejected

- Why this matters: bad config must fail loudly.
- **Given** a negative weight or an alias with an empty target
- **When** the config is loaded or the admin API is called
- **Then** the operation fails with a clear error naming the offending entry.
- Evidence: startup fails (or API returns 400) with a message naming the provider/alias.

## Assumptions

- Aliases apply uniformly to every model-bearing endpoint.
- Aliases are single-hop; an alias target is not itself re-aliased.
- Key model-scope checks apply to the routed (post-alias) model; the requested alias name must not be explicitly blocked.
- `weight` is an integer; weight 0 or unset means "not weighted" for AC-006/AC-007.
- Provider health is already tracked; the same health signal gates selection.

## Success Criteria

- SC-001 Selection adds no measurable latency (<1 ms p95) versus the current first-healthy lookup.
- SC-002 Existing routing tests keep passing unchanged.

## Edge Cases

- Alias present for a model that also has an exact route: the alias applies first (RQ-001).
- Alias target equals the requested name: identity, no loop.
- All providers in the minimum tier unhealthy: fall through to the next tier.
- Single provider in the tier: selected deterministically regardless of weight.
- A tenant alias and a global wildcard both exist: alias resolution then normal precedence applies.

## Open Questions

- Alias storage: reuse `shield.tenant_model_mapping` or introduce a routing-level `routing.aliases` map? (Plan to decide; either is tenant-scoped.)
- Should key model scopes be evaluated against the requested alias name, the routed name, or both? (Default assumed: routed name, plus reject if the requested name is explicitly blocked.)
- `weight` as integer (assumed) or float.
