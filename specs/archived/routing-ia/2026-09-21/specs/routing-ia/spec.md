# Providers, Models & Routing IA

## Scope Snapshot

- In scope: split the single Routing page into Providers / Models / Routing pages, introduce a global default model routing (a reserved tenant marker with selector fallback), add the aggregate and atomic admin APIs the pages need, and fix provider validation/proxy exposure.
- Out of scope: provider breadth, weighted/latency-based routing strategies, per-tenant cost rates, and auth/role changes.

## Goal

Operators currently manage three different concepts on one page and cannot express "this model is served by these providers by default, and tenant X overrides it". This feature gives each concept its own surface: Providers (credentials, proxy, health), Models (cost and default providers), and Routing (per-tenant overrides), backed by a global default that applies to every tenant until overridden. Adding a provider can attach its models in one action, and provider validation becomes api_type-aware so editing a provider with masked secrets no longer fails.

## Primary User Flow

1. Starting point: an operator adds a provider (for example OpenRouter) with its base URL, API key, and optional egress proxy.
2. Main interaction: they attach model ids to that provider, then open Models to set each model's cost and default provider order.
3. Outcome: tenants with no explicit override use the model's default providers; the operator opens Routing to override a specific tenant's provider order.
4. Failure/fallback path: a model with no route returns the existing no-route error; a tenant override always wins over the default; deleting a model removes its cost rate and default route.

## User Stories

- P1 Story: as an operator, I can add a provider with its models in one step.
- P2 Story: as an operator, I can see and edit a model's cost and its default providers in one place.
- P3 Story: as an operator, I can bind a model's provider order to a specific tenant without affecting others.
- P4 Story: as an operator, I can edit a provider that has masked secrets without re-entering them, and I can set an egress proxy per provider.

## MVP Slice

- Global default routing (marker + selector fallback), provider CRUD fix (validation + proxy), and the Providers page.
- AC covered: AC-001, AC-002, AC-005.

## First Deployable Outcome

- After the first pass, a provider can be created with proxy and models, a model's default providers route traffic for tenants without overrides, and a tenant override still wins.

## Scope

- Backend: global tenant marker and selector fallback; aggregate models endpoint; atomic provider+models endpoint.
- Frontend: three pages (`/providers`, `/models`, `/routing`) with their navigation entries and modals.
- Fixes: api_type-aware provider validation (masked-secret aware) and a `proxy_url` field in the provider form.
- Existing routing data and behavior for tenants `""`/`default` are preserved.

## Out of Scope

- New routing strategies (weighted, latency, cost-based) and traffic mirroring.
- Per-tenant cost rates or per-tenant pricing.
- Provider registry YAML/community definitions.
- Reworking the tenant, budget, or virtual-key pages.

## Context

- Storage today: `routing_providers`, `routing_model_routes(model, tenant, providers, source, UNIQUE(model,tenant))`, and `cost_rates(model PK, prices, currency, source)`.
- There is no first-class model entity; a model exists implicitly in routes and cost rates.
- The route selector normalizes an empty tenant to `default` and matches rules exactly, so there is no cross-tenant default today.
- The admin routing handler requires a non-empty `api_keys` list for every provider, while the config validator exempts `ollama` and uses AWS credentials for `bedrock`.
- The provider repository already preserves masked secrets on upsert; the UI validation is what blocks editing a provider whose key is masked.
- Per-provider egress proxy is already supported end to end (`proxy_url` in the DTO, repository, and `egress.NewTransport`) but is not exposed in the UI form.

## Dependencies

- Existing routing registry repository, route selector, and admin routing/cost-rate handlers.
- Existing provider egress transport and secret-preservation logic.
- Existing UI primitives (Button, Card, EmptyState, StatusPill, ConfirmModal, Toast) and the admin API client.
- No new external dependency.

## Requirements

- RQ-001 A reserved global tenant marker MUST exist such that a route stored under it applies to every tenant that has no explicit route for that model.
- RQ-002 The route selector MUST prefer a tenant-specific route and fall back to the global route only when no tenant-specific route matches; when neither exists the existing no-route behavior applies.
- RQ-003 The admin API MUST expose a read-only aggregate of models with their cost, default providers, and the number of tenant overrides.
- RQ-004 The admin API MUST create or update a provider together with a set of attached models in one call, writing the provider, its global routes, and placeholder cost rates atomically.
- RQ-005 The Providers page MUST support full provider CRUD, including egress proxy, auth fields, and AWS fields, with api_type-aware validation that accepts unchanged masked secrets.
- RQ-006 The Models page MUST let an operator set a model's cost and its ordered default providers, and deleting a model MUST remove its cost rate and global route.
- RQ-007 The Routing page MUST manage per-tenant ordered-provider overrides for a model and MUST indicate when a model inherits the global default instead of having an override.
- RQ-008 Navigation MUST expose Providers, Models, and Routing as separate destinations.
- RQ-009 Existing routing behavior MUST be preserved: routes stored for `""`/`default` keep selecting as before, and existing YAML/UI routes are not rewritten.
- RQ-010 Editing or removing the global default MUST NOT silently change tenants that have an explicit override.
- RQ-011 The admin API MUST expose an endpoint that lists a provider's available models by querying the provider's own models API, honoring the provider's auth and egress proxy; provider types without a models API MUST return a clear unsupported error.
- RQ-012 The Providers form MUST let an operator load the provider's models and select them (searchable picker), and MUST keep manual model entry as a fallback when loading fails or is unsupported.

## Non-Goals

- Changing how the gateway selects a provider among a healthy set beyond adding the global fallback.
- Introducing model metadata beyond id, cost, and default providers.
- Changing budget or analytics semantics; cost remains global per model.

## Acceptance Criteria

### AC-001 Global default routes tenants without an override

- Why this matters: operators need one place to define a model's default providers.
- **Given** a model with a global route and a tenant with no route for that model
- **When** that tenant requests the model
- **Then** the global route's providers are selected.
- Evidence: a selector test asserting the global provider is chosen for an unconfigured tenant.

### AC-002 Tenant override wins over the global default

- Why this matters: per-tenant control must remain authoritative.
- **Given** a model with both a global route and a tenant-specific route
- **When** the tenant requests the model
- **Then** the tenant-specific providers are selected, and a model with no route at all still returns the no-route error.
- Evidence: selector tests for the override and the no-route cases.

### AC-003 Model aggregate is available

- Why this matters: the Models page needs one source for cost, default providers and override count.
- **Given** cost rates, a global route, and a tenant override for a model
- **When** the aggregate endpoint is called
- **Then** it returns the model with its cost, its default providers, and an override count of one.
- Evidence: a handler test asserting the aggregate shape.

### AC-004 Provider and models are saved atomically

- Why this matters: a partial save would leave a provider without its models or vice versa.
- **Given** a provider create request that includes model ids
- **When** the request succeeds
- **Then** the provider, its global routes, and placeholder cost rates all exist; and when the provider write fails, no route or cost rate is created.
- Evidence: handler/repository tests for the success and failure paths.

### AC-005 Provider CRUD is api_type-aware and exposes the proxy

- Why this matters: operators must be able to edit a provider with masked secrets and route it through a proxy.
- **Given** an existing openai-type provider whose key is masked, and a submitted `proxy_url`
- **When** the provider is saved without re-entering the key
- **Then** the save succeeds, the stored secret is unchanged, and the proxy is persisted; a bedrock provider accepts AWS credentials and an `ollama` provider needs no key.
- Evidence: UI validation tests plus a handler/repository test for masked-secret preservation and proxy persistence.

### AC-006 Models page manages cost and default providers

- Why this matters: cost and default routing belong to the model.
- **Given** the Models page with a model
- **When** the operator sets its cost and default providers
- **Then** the cost rate and global route are updated; deleting the model removes both.
- Evidence: UI tests plus handler tests for the delete path.

### AC-007 Routing page manages tenant overrides and shows inheritance

- Why this matters: tenant overrides are the routing page's job.
- **Given** a model with a global default and one tenant override
- **When** the Routing page renders
- **Then** the override is listed and models without an override are shown as inheriting the default.
- Evidence: UI test asserting the override row and the inherited indicator.

### AC-008 Navigation exposes the three pages

- Why this matters: the split must be discoverable.
- **Given** the operator console
- **When** the sidebar renders
- **Then** Providers, Models, and Routing each appear as separate destinations.
- Evidence: a layout/navigation test.

### AC-009 Existing routing behavior is preserved

- Why this matters: the change must not break current deployments.
- **Given** existing routes stored for `""`/`default`
- **When** the gateway selects a provider
- **Then** selection is unchanged and existing routing tests pass.
- Evidence: existing selector/registry tests stay green plus a regression test.

### AC-010 Global changes do not mutate tenant overrides

- Why this matters: changing a default must not silently rewrite tenant intent.
- **Given** a tenant override for a model
- **When** the global default for that model is edited or removed
- **Then** the tenant's override is unchanged.
- Evidence: a test asserting the override persists after a global edit.

### AC-011 Provider model discovery

- Why this matters: typing model ids by hand is error-prone and slow.
- **Given** a provider with valid credentials whose API exposes a models endpoint
- **When** the operator requests its models
- **Then** the response lists the provider's model ids, and a provider type or endpoint that cannot list models returns a clear unsupported/unreachable error instead of an empty list.
- Evidence: a handler/service test with a fake models endpoint for a supported type and an unsupported type.

### AC-012 Provider model picker with manual fallback

- Why this matters: attaching models must be fast and still work when discovery fails.
- **Given** the Providers form
- **When** the operator loads models from the provider
- **Then** they can search and select ids that populate the provider's models; and when loading fails, manual entry still adds models.
- Evidence: a UI test covering selection from a loaded list and the manual fallback after a load error.

## Assumptions

- The global marker is a reserved tenant value (`*`) that cannot be a real tenant slug; tenant slug validation already forbids it.
- Cost rates remain global per model; per-tenant pricing is out of scope.
- The atomic provider+models call is the only place that writes global routes on provider creation; the Routing page writes tenant overrides.
- The Models page's override count is derived from routes with a non-global tenant.

## Success Criteria

- SC-001 An operator can add a provider with models and set model cost/defaults without leaving the UI.
- SC-002 A tenant override always wins and is never altered by global edits.
- SC-003 Existing routing tests remain green after the selector change.

## Edge Cases

- A model has a cost rate but no route: it appears on Models with no default providers and routes nowhere until configured.
- A provider referenced by a route is deleted: selection skips the missing provider as it does today.
- An override exists for a model with no global default: the override is used; other tenants get no route.
- A tenant literally named `*` is rejected by slug validation.
- Editing a bedrock provider with masked AWS credentials without re-entering them succeeds.

## Open Questions

- none
