# Provider & Model Registry

## Scope Snapshot

- In scope: declare providers and their models in YAML at startup, then add/edit providers and models in the admin UI, with wildcard routing so a provider's models are reachable without enumerating each one.
- Out of scope: provider-format translation, media endpoints, community provider-definition sync, automatic cost-rate updates, MCP/A2A.

## Goal

Operators bootstrap the gateway with a `routing.providers[]` list (including each provider's models) in YAML. After startup, admins manage the same providers, their models, and their routes entirely in the admin UI, without editing YAML or redeploying. A wildcard route (`<provider>/*`) exposes every model a provider serves so operators do not have to enumerate them, and `GET /v1/models` returns the resulting catalog. Success is visible when a provider/model added in the UI survives a restart, a request for an unlisted model resolves through its wildcard route, and `/v1/models` lists the union of routed and provider-declared models.

## Primary User Flow

1. Starting point: an operator lists providers and their models in `config.yaml`; the gateway seeds them at startup.
2. Main interaction: an admin opens the Providers/Models pages and adds a provider or a model (a name mapped to a provider), or removes one.
3. Outcome: the change is persisted, appears in `/v1/models`, and survives a restart without being overwritten by the YAML seed.
4. Failure/fallback path: an unknown model is routed through its provider wildcard route when configured, otherwise the existing `NO_ROUTE` error is returned.

## User Stories

- P1 Story: as an operator, I declare providers and models in YAML and never touch YAML again — everything else is done in the UI.
- P2 Story: as an admin, I add a provider's model in one place and it immediately becomes routable and discoverable.
- P3 Story: as an integrator, `GET /v1/models` reflects the catalog so my client can discover models.

## MVP Slice

- Provider model catalog in YAML seeded to the store + UI add/remove model; `GET /v1/models` includes the catalog. Must satisfy `AC-001`, `AC-002`, `AC-005`.

## First Deployable Outcome

- Adding `models: [llama3.2]` to a provider in YAML makes `llama3.2` appear in `/v1/models` after startup.
- Adding a model in the UI for a provider makes it appear in `/v1/models` and it survives a restart.
- Demonstrable with the admin UI and a `curl GET /v1/models`.

## Scope

- `routing.providers[].models[]` in YAML, seeded idempotently into the routing store.
- Admin API + UI to add/remove a provider's models and to add a model route.
- Wildcard routing: a route whose model is `"<provider>/*"` matches any model not matched by a more specific route.
- Aggregated catalog in `GET /v1/models` and `GET /api/v1/models`.
- Precedence and validation rules for exact vs wildcard routes.

## Context

- `ProviderConfig` (`src/internal/infra/config/config.go`, mirrored in `src/internal/domain/routing/config.go`) has no `models` field today; routes are per-model `RouteConfig{Model, Providers}`.
- The routing store already separates `Source: "yaml" | "ui"` and seeds from YAML only when empty (`routing_registry.go`); this feature must extend seeding to be idempotent per provider/model rather than all-or-nothing.
- The selector resolves models by exact match only (`selector.go`); wildcard routing is new.
- The admin UI already has a Providers page (CRUD, "load models from provider") and a Models page (cost rate + global route); this feature makes the model catalog first-class and adds wildcard routes.
- `api_type: proxy` is already the generic OpenAI-compatible provider; no new provider type is required for breadth.

## Dependencies

- Existing routing registry, admin routing handlers, and Providers/Models UI pages.
- Path-fidelity feature (shipped) for correct upstream paths.
- Tenant/key model scopes for catalog filtering.

## Requirements

- RQ-001 `routing.providers[].models[]` MAY be declared in YAML; each declared model is associated with its provider.
- RQ-002 The model catalog MUST be persisted and MUST be readable/writable through the admin API and UI (add and remove a model for a provider).
- RQ-003 A route whose model is the pattern `"<provider>/*"` MUST match any model that is not matched by a more specific route and MUST route to that provider.
- RQ-004 Model resolution precedence MUST be: exact tenant route, wildcard tenant route, exact global route, wildcard global route.
- RQ-005 `GET /v1/models` and `GET /api/v1/models` MUST return the de-duplicated union of routed models and provider-declared models, filtered by the presenting key's model scopes.
- RQ-006 YAML seeding MUST be idempotent and MUST NOT overwrite or remove entries whose `Source` is `ui`.
- RQ-007 Invalid catalog/route entries (unknown provider in a wildcard route, malformed pattern) MUST be rejected with a clear error at config load and at the admin API.
- RQ-008 Provider and model changes made in the UI MUST persist across a restart.

## Non-Goals

- Adding new provider formats or translation beyond the existing `api_type` set.
- Automatic discovery/importer of community provider definitions.
- Media endpoints (images/audio), responses, batches, MCP/A2A.
- Automatic cost-rate synchronization (cost rates remain operator-managed).
- Changing the existing `/api/v1/models` array shape (only its contents may grow).

## Acceptance Criteria

### AC-001 YAML model catalog is seeded

- Why this matters: operators bootstrap the catalog declaratively.
- **Given** a provider `groq` with `models: [llama-3.3-70b]` in YAML and an empty store
- **When** the gateway starts
- **Then** `llama-3.3-70b` is associated with `groq` in the store and returned by `GET /v1/models`.
- Evidence: a post-start `GET /v1/models` contains `llama-3.3-70b`; a registry test reads the association back.

### AC-002 UI model add persists

- Why this matters: admins manage models without YAML.
- **Given** a running gateway and provider `groq`
- **When** an admin adds model `mixtral-8x7b` for `groq` in the UI
- **Then** `POST` succeeds, `GET /v1/models` includes `mixtral-8x7b`, and after a restart it is still present.
- Evidence: the admin API response and a post-restart `GET /v1/models` both contain the model.

### AC-003 Wildcard route resolves unknown models

- Why this matters: a provider's models are usable without enumerating each one.
- **Given** a global wildcard route `model: "groq/*"` → provider `groq` and no exact route for `some-new-model`
- **When** a client sends `POST /v1/chat/completions` with `model: "some-new-model"`
- **Then** the request is routed to `groq` (provider `groq` receives it) instead of `NO_ROUTE`.
- Evidence: the recording provider shows the request; the response is not `NO_ROUTE`.

### AC-004 Exact route beats wildcard

- Why this matters: specific overrides must win over the catch-all.
- **Given** a wildcard route `"groq/*"` → `groq` and an exact route `"some-new-model"` → `openai`
- **When** a client requests `some-new-model`
- **Then** the request is routed to `openai`, not `groq`.
- Evidence: the recording provider for `openai` receives the request; the `groq` provider receives none.

### AC-005 Catalog is aggregated and scoped

- Why this matters: clients discover exactly what they may use.
- **Given** models from routes (`m1`, `m2`), a provider catalog (`m3`), and a key scoped to `m1` and `m3`
- **When** the key calls `GET /v1/models`
- **Then** the response contains `m1` and `m3` once each and not `m2`.
- Evidence: response `data[].id` set equals `{m1, m3}` with no duplicates; an unscoped key returns `{m1, m2, m3}`.

### AC-006 YAML seed does not overwrite UI edits

- Why this matters: UI changes must be durable.
- **Given** a provider/model added in the UI (`Source: ui`) and a YAML seed with different content
- **When** the gateway restarts
- **Then** the UI entry is preserved and the seed only fills missing entries.
- Evidence: after restart the UI-managed model is still present and unchanged; the seed added only the absent one.

### AC-007 Invalid entries are rejected

- Why this matters: a bad wildcard must not silently break routing.
- **Given** a wildcard route referencing an unknown provider, or a malformed pattern
- **When** the config is loaded (or the entry is submitted through the admin API)
- **Then** the operation fails with a clear error naming the offending entry.
- Evidence: startup fails (or the API returns 400) with a message naming the route/provider; no partial state is written.

### AC-008 UI model management round-trips

- Why this matters: providers and models are fully manageable from the console.
- **Given** the Providers/Models pages
- **When** an admin adds and then removes a model for a provider
- **Then** both operations are reflected in the stored catalog and in `/v1/models` after each step.
- Evidence: `GET /v1/models` includes the model after add and excludes it after remove.

## Assumptions

- `api_type: proxy` remains the generic OpenAI-compatible provider; breadth comes from configuration, not new code.
- The admin UI is the operator console (not an end-user surface), per the constitution.
- Wildcard routes apply to the request model name via the same routing table; no new request field is introduced.
- Cost rates remain separately managed; the catalog does not imply pricing.

## Success Criteria

- SC-001 Adding a model in the UI is reflected in `/v1/models` within 1 second (no restart required).
- SC-002 Startup seeding adds no measurable boot delay for catalogs up to 1000 models.

## Edge Cases

- A wildcard route and an exact route coexist for the same provider: exact wins (AC-004).
- A model declared in the catalog but not routed and not covered by a wildcard: appears in `/v1/models` but requests return `NO_ROUTE` (documented).
- Duplicate models across providers: listed once in `/v1/models`; routing uses the configured route precedence.
- Removing a model that an existing route references: the route remains; the model leaves the catalog.
- Empty `models: []`: allowed, adds nothing.

## Open Questions

- Wildcard syntax: `"<provider>/*"` (assumed) versus an explicit `model_pattern` field on the route.
- Is the catalog global with a provider association (assumed) or per-tenant?
- Should a model declared in the catalog but unrouted be auto-routed to its provider (implicit route), or require an explicit/wildcard route? (Default assumed: require a route/wildcard.)
