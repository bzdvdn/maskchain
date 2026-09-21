# Shield Detector & Pack Catalog

## Scope Snapshot

- In scope: a read-only admin endpoint that reports the shield detectors, reactions and compliance packs the running process actually serves, and switching the operator UI to consume it instead of a hardcoded pack list.
- Out of scope: new detectors (NER/moderation), per-tenant policy status, gateway-side exposure, and any change to how presets are applied.

## Goal

Operators and the UI cannot currently discover what the shield actually supports: the pack list is hardcoded in the frontend and detector availability is only implicit in code. This feature adds a single catalog endpoint backed by the runtime registries so the UI and operators see the truth — which detector types are registered, which reactions are valid, and which compliance packs are loaded — and the UI stops drifting from the backend.

## Primary User Flow

1. Starting point: an operator opens the Compliance page (or any pack selector) in the admin UI.
2. Main interaction: the UI calls the catalog endpoint with its admin session.
3. Outcome: the pack selector is populated from the loaded presets and the page can show the detectors/reactions the shield serves, with no hardcoded list.
4. Failure/fallback path: when no presets are loaded, the catalog still returns detectors and reactions with an empty pack list (HTTP 200), so the UI shows an empty state instead of breaking.

## User Stories

- P1 Story: as an operator, I can see which detectors and reactions the shield actually serves.
- P2 Story: as an operator, I can see which compliance packs are available without guessing or reading source.
- P3 Story: as a UI maintainer, I fetch packs from the backend so the frontend cannot drift from the preset set.

## MVP Slice

- AC-001 (detectors + reactions), AC-002 (packs), AC-003 (auth). These make the endpoint usable and safe on their own.

## First Deployable Outcome

- After the first pass, `GET /api/v1/shield/catalog` with an admin session returns the registered detector types, the valid reactions and the loaded packs; without a session it returns 401.

## Scope

- Admin endpoint `GET /api/v1/shield/catalog` under the admin session middleware.
- Response content: registered detector types, allowed reactions, loaded compliance packs (key + name).
- Wiring the admin process to expose the same detector reference set and compliance registry it already uses for preset validation.
- UI Compliance page consumes the catalog for its pack options.

## Out of Scope

- Reporting per-tenant active/deviated rules (that stays on the existing compliance report endpoint).
- Exposing the catalog on the gateway/data plane.
- Implementing declared-but-unregistered detector types (for example a statistical/Presidio-style detector).
- Changing preset loading, application, or validation semantics.

## Context

- Detector types are registered at runtime in the gateway (`regex`, `dictionary`, `prompt_injection`); the admin process already builds a reference detector registry for preset validation that mirrors the gateway set.
- Compliance packs are loaded from `compliance.preset_dir` into `compliance.Registry`; the admin process already holds that registry for apply/report.
- The admin UI currently hardcodes the pack list in `ui/src/api/tenants.ts`, which can drift from the loaded presets.
- The admin API uses a unified envelope (`{data, error, pagination}`) and admin-session auth.

## Dependencies

- Existing detector registry (`domain/shield/detector`) and reaction constants (`domain/shield/entity`).
- Existing `compliance.Registry` and its `Packs()` accessor.
- Admin session middleware and the admin router.
- No new external dependency.

## Requirements

- RQ-001 The admin process MUST expose `GET /api/v1/shield/catalog` behind the admin session middleware.
- RQ-002 The response MUST list the detector types that are actually registered in the runtime registry, the allowed reactions, and the loaded compliance packs.
- RQ-003 The response MUST reflect runtime state: a detector type that is declared but not registered MUST NOT be advertised as available.
- RQ-004 When no packs are loaded, the endpoint MUST return HTTP 200 with an empty pack list plus the detectors and reactions.
- RQ-005 Detector types, reactions and packs MUST be returned in a deterministic order so diffs and UI rendering are stable.
- RQ-006 The admin UI Compliance page MUST populate its pack selector from the catalog rather than a hardcoded list.

## Non-Goals

- Changing the shape of existing compliance apply/report endpoints.
- Adding caching, pagination or filtering to the catalog (it is small and static per process).
- Exposing rule details beyond pack key and name.

## Acceptance Criteria

### AC-001 Catalog reports registered detectors and reactions

- Why this matters: operators must know what the shield can detect and which reactions are valid.
- **Given** an admin session and a runtime detector registry with registered types
- **When** the client requests the catalog
- **Then** the response contains exactly those registered detector types and the set of allowed reactions.
- Evidence: an admin-handler test that builds a registry with a known type set and asserts the returned detector types and reactions match.

### AC-002 Catalog reports loaded compliance packs

- Why this matters: the UI and operators must see the packs that are actually available.
- **Given** a compliance registry loaded with known packs
- **When** the client requests the catalog
- **Then** each loaded pack appears with its key and name.
- Evidence: an admin-handler test with two known packs asserting both keys/names are present.

### AC-003 Catalog requires an admin session

- Why this matters: catalog data is control-plane information and must not be public.
- **Given** no valid admin session
- **When** the client requests the catalog
- **Then** the request is rejected with HTTP 401.
- Evidence: a route test hitting the endpoint without credentials.

### AC-004 Declared-but-unregistered types are not advertised

- Why this matters: the catalog must be runtime truth, not a static declaration.
- **Given** a detector type constant that exists but has no registered detector
- **When** the client requests the catalog
- **Then** that type is absent from the reported detector list.
- Evidence: an admin-handler test using a registry that omits a declared type and asserting it is not returned.

### AC-005 Empty pack set still returns detectors and reactions

- Why this matters: a deployment without presets must not break the UI.
- **Given** compliance is disabled or no packs are loaded
- **When** the client requests the catalog
- **Then** the response is HTTP 200 with an empty pack list and a non-empty detector/reaction set.
- Evidence: an admin-handler test with a nil/empty compliance registry.

### AC-006 UI pack selector is catalog-driven

- Why this matters: the frontend must not drift from the backend preset set.
- **Given** the Compliance page loads
- **When** it renders the pack selector
- **Then** the options come from the catalog response and no hardcoded pack list remains in the UI source.
- Evidence: the UI fetches the catalog client function and the hardcoded pack constant is removed; a UI test asserts options render from the mocked catalog.

## Assumptions

- The admin process can build the same reference detector registry it already uses for preset validation, so the reported detector types match the gateway's served set.
- Packs loaded at admin startup are the authoritative available set; the catalog does not need to re-read the preset directory per request.
- Reaction identifiers are the existing four values (`allow`, `block`, `review`, `log`).

## Success Criteria

- SC-001 The catalog responds in under 50ms (in-memory registries, no I/O).
- SC-002 Removing or adding a preset file changes the catalog pack list without a code change.

## Edge Cases

- Compliance registry is nil because `preset_dir` is missing: return detectors/reactions with `packs: []`.
- Duplicate or malformed packs are already rejected at load time; the catalog only reports loaded packs.
- A registry with zero registered detectors: return an empty detector list rather than an error.
- The endpoint is requested with a tenant virtual key instead of an admin session: treated as unauthenticated (401).

## Open Questions

- none
