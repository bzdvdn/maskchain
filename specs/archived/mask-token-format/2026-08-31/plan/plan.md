# Mask Clean Tokens — Plan

## Phase Contract

Inputs: `spec.md` (approved), `inspect.md` (pass), constitution summary, minimal repo context.
Outputs: `plan.md`; `data-model.md` not created (single additive column documented inline).
Stop if: any AC would require behavior not in spec — none found.

## Goal

Give standalone `/v1/shield/mask` clean output tokens by default (`[MASK.<N>]`, no document id), a `format=clean|id|redact` parameter, redact mode as non-reversible `[REDACTED]`, and sequential per-document unmask that stays backward compatible with legacy `[MASK_<id>.<N>]` entries. Scope is the standalone mask domain + handler + storage entity; conversation/proxy shield tokens (`[[pii…]]`) are untouched.

## MVP Slice

- Domain enum `mask.Format` (`Clean | ID | Redact`), `MaskEntry.Reversible` flag persisted via a new column, `MaskFromResults` threaded with format, handler `format` param, sequential `UnmaskText`, and tests for clean/id/redact + legacy restore.
- Must satisfy `AC-001`..`AC-008` (single coherent slice; proxy nonce included as the final piece).

## First Validation Path

1. `go test ./internal/domain/shield/mask/... ./internal/api/` — use-case round-trips (clean, id, redact-reject, legacy restore, sequential) and handler param tests green.
2. Manual: `curl -X POST localhost:8080/api/v1/shield/mask` with a PII line → body shows `[MASK.1]` (no `_<id>`); `?format=id` reproduces `[MASK_<id>.1]`; `?format=redact` shows only `[REDACTED]`; unmask of redact doc returns 400.
3. Legacy check: mask with the *old* binary/entry (`[MASK_<id>.<N>]`) then unmask with the new code → restores.

## Scope

- `src/internal/domain/shield/mask/` — `Format` enum, `MaskEntry.Reversible`, `MaskFromResults` signature + token builder, `UnmaskText` sequential semantics.
- `src/internal/adapters/repository/mask/postgres.go` — persist/read `reversible`.
- `deployments/migrations/` — additive `mask_entries.reversible boolean NOT NULL DEFAULT TRUE`.
- `src/internal/api/mask_handler.go` — `format` query param resolution (absent → clean, invalid → 400), redact unmask rejection passthrough.
- `src/internal/api/middleware/shield.go` — proxy dictionary tokens are clean per-request counters with no id (AC-008).
- Handler + domain tests: `mask_handler_test.go`, `usecase_test.go`; middleware test: `shield_test.go`.
- Docs: `examples/test-prompt.md` notes the default format.
- Untouched: shield engine proxy PII tokens (`[[pii…]]`), conversation payload shape, `MaskStorage` interface shape, `/unmask` contract.

## Performance Budget

- None meaningful — same per-doc O(fragments) work; `UnmaskText` iterates ids sequentially; token build unchanged complexity. State `none` for latency/memory targets.

## Implementation Surfaces

- `src/internal/domain/shield/mask/entity.go` — `Reversible bool` on `MaskEntry` (json tag for storage if needed).
- `src/internal/domain/shield/mask/errors.go` — `ErrNotReversible` sentinel.
- `src/internal/domain/shield/mask/usecase.go` — `MaskFormat` enum + `MaskFromResults` format param + token builder; `UnmaskText` per-doc sequential replace; `ResolveOverlaps` unchanged.
- `src/internal/adapters/repository/mask/postgres.go` — SELECT/INSERT/scan of `reversible`.
- `deployments/migrations/` — new migration file (adds column).
- `src/internal/api/mask_handler.go` — parse `format`, default clean, 400 invalid, 400 on redact unmask.
- `src/internal/api/middleware/shield.go` — emit clean per-request counters for dictionary tokens; keep `dictMaskID`/header/mapping semantics.
- Tests: `usecase_test.go`, `mask_handler_test.go`, `shield_test.go`.

## Bootstrapping Surfaces

- `mask.Format` enum + `MaskEntry.Reversible` + migration MUST exist before handler logic; handler depends on the enum for param mapping.
- Existing `MaskStorage` interface is sufficient — no port changes.

## Architecture Impact

- Domain: token style becomes an explicit enum; reversibility becomes a stored property instead of being implied by token shape.
- Persistence: one additive column (`mask_entries.reversible`, default TRUE) — legacy rows read as reversible without backfill.
- API: `/mask` gains an additive query param; `/unmask` semantics changed from merged-map to sequential (compatible for all realistic inputs); error surface gains 400 "not reversible".

## Acceptance Approach

- AC-001 (clean default, no id in body) → default enum `Clean` in handler; token `[MASK.<N>]` → proof: use-case + handler tests assert `^\[MASK\.[0-9]+\]$` and headers present.
- AC-002 (legacy via `format=id`) → param maps to enum `ID`; token builder legacy branch → proof: test asserts `[MASK_<docID>.<N>]`.
- AC-003 (invalid format 400) → handler validation before usecase; no entry saved → proof: handler test 400 + storage untouched.
- AC-004 (clean restore) → `UnmaskText` replaces by entry mapping keys → proof: round-trip test text→mask→unmask equality.
- AC-005 (legacy entries restore) → same mapping-key mechanism, entries with old tokens → proof: test that seeds legacy tokens and asserts restoration.
- AC-006 (sequential multi-id) → per-doc loop replacing into running result, not merged map → proof: two-entry test where doc B must not overwrite doc A's resolved tokens.
- AC-007 (redact non-reversible, no id) → `Redact` emits `[REDACTED]`, saves `Reversible=false`; unmask on it → 400 `ErrNotReversible` → proof: use-case + handler tests assert body tokens and 400.
- AC-008 (proxy dict tokens are clean) → `shield.go` emits per-request counters; unmask mapping and header unchanged → proof: middleware test asserting the clean-token regexp; existing `TestDictUnmask`/`TestStreamingDictUnmask` stay green.
- Persistence proof: migration file + repo test round-trips `reversible` default true / explicit false.

## Data and Contracts

- `Data model: change` — additive: `mask_entries.reversible BOOLEAN NOT NULL DEFAULT TRUE` (migration). No backfill; legacy rows default reversible.
- `MaskEntry.Reversible` bool (defaults `true` when loading legacy rows) — informs unmask reject for redact docs.
- API contract: `GET/POST /api/v1/shield/mask?format=clean|id|redact` — additive, absent = clean; `/unmask?mask_ids=…` unchanged. No headers/fields removed. `contracts/api.md` not created — the only API delta is one additive query param documented here.

## Implementation Strategy

- DEC-001 `mask.Format` enum in the domain, resolved at the handler
  Why: token style and reversibility become testable domain behavior; handler stays a thin mapper.
  Not: passing raw format strings into the use case.
  Affects: `mask/entity.go`, `mask/usecase.go`, `mask_handler.go`.
  Validation: domain tests per format; handler test for invalid value → 400.
- DEC-002 Persist `Reversible` as a `mask_entries.reversible` column (DEFAULT TRUE)
  Why: distinguishes redact (400 not reversible) from unknown (404); legacy rows restore without migration.
  Not: packing reversibility into the `replacements` JSON (breaks reading legacy plain-map rows).
  Affects: `postgres.go`, `deployments/migrations/`.
  Validation: repo round-trip test; migration file present.
- DEC-003 `UnmaskText` applies ids sequentially per document
  Why: removes cross-doc token collisions that made ids-in-tokens necessary; matches low multi-id demand.
  Tradeoff: restoring one body mixing tokens from two different documents is unsupported (documented limitation).
  Affects: `usecase.go`, `usecase_test.go`.
  Validation: sequential two-entry test (AC-006).
- DEC-004 `clean` is the default; `id` is legacy opt-in
  Why: product goal (no id in output) is default; integrators parse old tokens via `format=id`.
  Tradeoff: consumers omitting `format` see the new shape (accepted decision, documented).
  Affects: `mask_handler.go`.
  Validation: AC-001/AC-002 handler tests; docs note.
- DEC-005 Proxy dict tokens are clean per-request counters
  Why: hiding the mapping id entirely; history is re-scanned per turn with one per-request counter, so counter tokens are unambiguous within a request and unmask is per-request (nonce prefix is unnecessary complexity).
  Tradeoff: tokens look similar across turns; acceptable because each turn re-masks and unmaps with its own mapping.
  Affects: `middleware/shield.go`, `shield_test.go`.
  Validation: AC-008 middleware test; existing dict-unmask tests green.

## Incremental Delivery

### MVP (First Value)

- Migration (reversible column) → enum + entity → usecase token builder/unmask → handler param → tests.
- MVP readiness: AC-001..AC-007 green via unit tests + manual curl path above.

### Iterative Expansion

- none planned — the feature is a single coherent slice.

## Sequencing Notes

- Migration lands first (additive, safe on existing DBs).
- Domain (enum + token builder + sequential unmask) before handler wiring.
- Handler param parsing last; all behind the same feature branch, no flag needed (additive param + additive column).

## Risks

- Existing consumers omitting `format` see new token shape
  Mitigation: `format=id` kept; behavior documented; spec Assumption records it as accepted.
- Security: `[MASK.<N>]` counters alone could still correlate two texts from the same doc
  Mitigation: accepted — clean mode hides only the doc id; callers needing zero correlation use `redact`.
- Redact rejection error surface (400 vs historical 404 for unknown)
  Mitigation: unknown ids remain 404; known redact entries get a distinct 400 `not reversible` — covered by tests.
- DB column addition on existing clusters
  Mitigation: NOT NULL DEFAULT TRUE, additive; no application reads depend on absence.

## Rollout and Compatibility

- One additive migration; no backfill.
- New masks default to clean tokens; legacy entries and texts remain restorable (mapping-key mechanism).
- `/unmask` behavior change (sequential) is compatible for all realistic inputs; the only incompatible case (mixed two-doc single body) is documented.
- Docs (`examples/test-prompt.md`) update to show default clean output and `format=id`/`redact`.

## Validation

- Unit (Go): use-case — clean format tokens; id format tokens; redact tokens + `ErrNotReversible`; legacy-token restore; sequential multi-doc; empty-results no-op. Handler — default clean, `format=id`, `format=redact`, invalid → 400, redact unmask → 400, header presence. Middleware — dict tokens use a nonce distinct from the header id; dict unmask round-trips. Repo — reversible round-trip (default true, explicit false).
- Commands: `go test ./internal/domain/shield/mask/... ./internal/api/... ./internal/adapters/...`
- Manual: validation path in "First Validation Path".
- Proves: AC-001..AC-007, DEC-001..DEC-004.

## Constitution Compliance

- no conflicts — change is confined to the standalone mask domain/API; no impact on Content Shield placement, tenants, UI scope, or language policy.