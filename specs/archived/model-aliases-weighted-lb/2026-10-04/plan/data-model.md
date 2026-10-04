# Model Aliases & Weighted Load Balancing Data Model

## Scope

- Related acceptance IDs: `AC-006`, `AC-007`, `AC-009`
- Related decision IDs: `DEC-002`, `DEC-004`, `DEC-006`
- Status: `changed`

## Entities

### DM-001 Provider weight (extends `routing_providers`)

- Purpose: give each provider a relative share when it is in the selected priority tier.
- Source of truth: `routing_providers.weight` (Postgres); mirrored by `ProviderConfig.Weight`.
- Invariants: non-negative integer; `0` means "unweighted" and is treated as effective weight `1` at selection time (DEC-004).
- Related acceptance IDs: `AC-006`, `AC-007`.
- Related decision IDs: `DEC-004`.
- Fields:
  - `weight` - `INTEGER NOT NULL DEFAULT 0`, optional (defaults to 0 = unweighted)
- Lifecycle:
  - created with the provider row (UI upsert, YAML seed, or migration default)
  - updated on provider upsert via the admin API/UI
  - removed with the provider row
- Failure or consistency notes:
  - a negative value must be rejected at config load and admin API before write

### DM-002 Routing alias (`routing_aliases`)

- Purpose: map a tenant's requested model name to a routed model name before selection.
- Source of truth: `routing_aliases` (Postgres); mirrored by `RoutingConfig.Aliases`.
- Invariants: unique per `(tenant, alias)`; `target` non-empty; resolution is single-hop (no chains, no cycles).
- Related acceptance IDs: `AC-001`, `AC-002`, `AC-003`, `AC-004`, `AC-009`.
- Related decision IDs: `DEC-001`, `DEC-005`, `DEC-006`.
- Fields:
  - `tenant` - `TEXT NOT NULL`; a tenant slug or the reserved global tenant `*`
  - `alias` - `TEXT NOT NULL`; the requested model name
  - `target` - `TEXT NOT NULL`; the model name to route
  - `source` - `TEXT NOT NULL`; `yaml` or `ui` provenance
  - `created_at` / `updated_at` - `TIMESTAMPTZ NOT NULL DEFAULT now()`
- Lifecycle:
  - created by YAML seed (insert-if-absent) or admin UI/API upsert
  - updated by an admin upsert (sets `source = ui`)
  - removed by admin delete
- Failure or consistency notes:
  - a target with no route is allowed to persist but yields `NO_ROUTE` at request time (AC-003); it must never silently pass the alias name through
  - YAML seed must not overwrite an existing `(tenant, alias)` (DEC-006)

## Relationships

- `DM-001` belongs to a provider row (`routing_providers`), 1:1.
- `DM-002` is independent of providers; it references a model name that is resolved through `routing_model_routes` (exact or wildcard).
- `DM-002.tenant` parallels the tenant key used by `routing_model_routes`.

## Derived Rules

- Effective weight = `weight` when `> 0`, otherwise `1` (DEC-004). If all candidates in the tier have equal effective weight, selection returns the first declared (deterministic, AC-007).
- Alias resolution: `model' = aliasFor(tenant, model) ?? model`; then normal route lookup on `model'` (single hop, DEC-005).
- Candidate set = healthy providers of the minimum `priority` tier across the resolved chain (AC-005, AC-008).
- Fallback chain = weighted primary first, then the remaining chain in declared order (DEC-007).

## State Transitions

- Alias upsert: absent -> present (create) or present -> updated (`source` becomes `ui`).
- Alias delete: present -> removed.
- Provider weight: any non-negative value <-> any other non-negative value on upsert.
- Guards: negative weight rejected; empty alias target rejected.

## Out of Scope

- Key-level or request-level aliases.
- Multi-hop alias chains and cycle detection beyond the single-hop rule.
- Weighted fallback ordering (only the primary is weighted).
- Per-request weight overrides.

## Migration

- `022_provider_weight_and_aliases.up.sql`:
  - `ALTER TABLE routing_providers ADD COLUMN IF NOT EXISTS weight INTEGER NOT NULL DEFAULT 0;`
  - create `routing_aliases` with `PRIMARY KEY (tenant, alias)`.
- `022_provider_weight_and_aliases.down.sql`: drop `routing_aliases`; drop `routing_providers.weight`.
