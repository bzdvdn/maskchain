# Mask Clean Tokens — Tasks

## Phase Contract

Inputs: `plan.md` (approved), `spec.md`, `inspect.md` (pass), constitution summary.
Outputs: ordered tasks with `Touches:` per task, Surface Map, Implementation Context, AC coverage.
Stop if: any AC cannot be mapped to executable work — none; all 7 covered.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/adapters/repository/postgres/migrations/021_mask_entries_reversible.{up,down}.sql (new) | T1.1 |
| src/internal/domain/shield/mask/entity.go | T1.2 |
| src/internal/domain/shield/mask/errors.go | T1.2 |
| src/internal/domain/shield/mask/usecase.go | T1.3 |
| src/internal/adapters/repository/mask/postgres.go | T1.4 |
| src/internal/domain/shield/mask/usecase_test.go | T1.5 |
| src/internal/api/mask_handler.go | T2.1 |
| src/internal/api/mask_handler_test.go | T2.2 |
| src/internal/api/middleware/shield.go | T4.1 |
| src/internal/api/middleware/shield_test.go | T4.2 |
| examples/test-prompt.md | T3.1 |
| repo-wide (src/ build+tests) | T3.2 |

## Implementation Context

- MVP Goal: clean default tokens `[MASK.<N>]` (no id), `format=clean|id|redact` on `/mask`, redact non-reversible `[REDACTED]`, sequential per-doc unmask, legacy restore intact (AC-001..007).
- Key Rules: enum `mask.Format{Clean,ID,Redact}` resolved at handler (absent→clean, invalid→400); `MaskEntry.Reversible` persisted as `mask_entries.reversible DEFAULT TRUE` (legacy rows stay `true`); `UnmaskText` applies ids sequentially per document (no merged map); redact unmask → 400 `ErrNotReversible`, unknown id → 404.
- Contracts: `POST /api/v1/shield/mask?format=clean|id|redact` (additive, header `mask-id`/`data_mask_id` unchanged); `POST /api/v1/shield/unmask?mask_ids=…` unchanged.
- Invariants: legacy `[MASK_<docID>.<N>]` entries and texts restore unchanged; conversation/proxy tokens (`[[pii…]]`) untouched; no storage rewrite.
- Proof Signals: `go test ./internal/domain/shield/mask/... ./internal/api/...` green; migration up/down files exist; round-trip tests per format.
- Out of Scope: conversation shield tokens, `/unmask` contract changes, storage backfill, new endpoints.

## Phase 1: Domain & Persistence

Goal: enum + reversible flag + token builder + sequential unmask, with the additive migration.

- [x] T1.1 Add reversible migration — outcome: `mask_entries.reversible BOOLEAN NOT NULL DEFAULT TRUE` applied by up, dropped by down, next-numbered files exist. Touches: src/internal/adapters/repository/postgres/migrations/021_mask_entries_reversible.up.sql, src/internal/adapters/repository/postgres/migrations/021_mask_entries_reversible.down.sql
      Proof: code src/internal/adapters/repository/postgres/migrations/021_mask_entries_reversible.up.sql
- [x] T1.2 Add enum, reversible flag, error — outcome: `mask.Format` (`Clean|ID|Redact`) and `MaskEntry.Reversible bool` in the mask domain; `ErrNotReversible` sentinel; existing behavior unchanged for reversible entries. Touches: src/internal/domain/shield/mask/entity.go, src/internal/domain/shield/mask/errors.go
      Proof: code src/internal/domain/shield/mask/entity.go
- [x] T1.3 Token builder + sequential unmask — outcome: `MaskFromResults` accepts a format; clean tokens `[MASK.<N>]` (no id), id tokens `[MASK_<docID>.<N>]`, redact emits `[REDACTED]` and sets `Reversible=false`; `UnmaskText` applies entries per document sequentially and returns `ErrNotReversible` for redact entries. Touches: src/internal/domain/shield/mask/usecase.go
      Proof: code src/internal/domain/shield/mask/usecase.go
- [x] T1.4 Persist reversible — outcome: `PostgresMaskRepo.Save/Get` write and read the `reversible` column; legacy rows (absent) load as `true`. Touches: src/internal/adapters/repository/mask/postgres.go
      Proof: code src/internal/adapters/repository/mask/postgres.go
- [x] T1.5 Domain tests — outcome: use-case tests green for clean tokens, id tokens, redact tokens + `ErrNotReversible`, legacy `[MASK_<id>.<N>]` restore, and sequential two-document no-overwrite. Touches: src/internal/domain/shield/mask/usecase_test.go
      Proof: test src/internal/domain/shield/mask/usecase_test.go

## Phase 2: API

Goal: wire the `format` param and redact rejection in the handler.

- [x] T2.1 Handler format param — outcome: `/mask?format=clean|id|redact` parsed (absent → clean; anything else → 400 before any save); `/unmask` maps `ErrNotReversible` → 400 and keeps 404 for unknown. Touches: src/internal/api/mask_handler.go
      Proof: code src/internal/api/mask_handler.go
- [x] T2.2 Handler tests — outcome: default-clean, explicit id, redact, invalid → 400 with headers asserted, redact-unmask → 400: all green. Touches: src/internal/api/mask_handler_test.go
      Proof: test src/internal/api/mask_handler_test.go

## Phase 3: Docs & Validation

Goal: document the default and prove the slice via targeted gates.

- [x] T3.1 Update mask docs — outcome: `examples/test-prompt.md` states default clean output and shows `format=id` / `format=redact` usage. Touches: examples/test-prompt.md
      Proof: docs examples/test-prompt.md
- [x] T3.2 Target gates — outcome: `go build ./...`, `go test ./internal/domain/shield/mask/... ./internal/api/... ./internal/adapters/...` pass; no regressions reported in touched packages. Touches: repo-wide go build/tests over src/, specs/active/mask-token-format/tasks.md
      Proof: chore specs/active/mask-token-format/tasks.md

## Phase 4: Proxy Parity

Goal: conversations stop exposing the mapping id — dictionary tokens get a decoupled per-request nonce.

- [x] T4.1 Proxy dict clean counters — outcome: `shield.go` dictionary masking emits clean per-request tokens `[MASK.<N>]` (no id, no nonce); `X-Shield-Dict-Mask-ID` and mapping semantics unchanged. Touches: src/internal/api/middleware/shield.go
      Proof: code src/internal/api/middleware/shield.go
- [x] T4.2 Middleware clean-counter test — outcome: new test asserts clean token regexp and header presence; existing `TestDictUnmask`/`TestStreamingDictUnmask` stay green. Touches: src/internal/api/middleware/shield_test.go
      Proof: test src/internal/api/middleware/shield_test.go

## Acceptance Coverage

- AC-001 -> T1.2, T1.3, T2.1, T2.2
- AC-002 -> T1.3, T2.1, T2.2
- AC-003 -> T2.1, T2.2
- AC-004 -> T1.3, T1.5, T2.2
- AC-005 -> T1.3, T1.5, T3.2
- AC-006 -> T1.3, T1.5
- AC-007 -> T1.2, T1.3, T1.5, T2.1, T2.2
- AC-008 -> T4.1, T4.2

## Notes

- Task IDs follow `T<phase>.<index>`; no phase intentionally omitted.
- Each completed task must carry `Proof: <kind> <repo-root-relative-path> [<anchor>]` on the following line; without it the task is not done.