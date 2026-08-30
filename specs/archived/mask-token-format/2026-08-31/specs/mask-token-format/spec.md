# Mask Clean Tokens (no doc id in output)

## Scope Snapshot

- In scope: change standalone `/api/v1/shield/mask` token format so masked text no longer contains the document id, add a `format` parameter with three modes (`clean` default, `id` legacy, `redact` non-reversible), keep `/unmask` contract with backward-compatible restore, and align proxy/conversation dictionary masking to use clean per-request counter tokens (no mapping-id leak).
- Out of scope: PII tokens in the proxy path (`[[pii.<label>.<N>]]`) stay untouched; no `format` parameter is added to the proxy path.

## Goal

Operators and consumers of `/mask` currently see `[MASK_<docId>.<N>]` inside output — the document id (the same value returned in `X-Mask-ID`/`data_mask_id` headers) leaks into masked text. This feature makes **clean tokens** the default: `[MASK.1]`, `[MASK.2]`, … (per-document counter only), so masked output carries no correlation identifier, while full reversibility and legacy compatibility are preserved. A `format` parameter selects three modes: `clean` (default, reversible), `id` (legacy id-visible tokens), `redact` (non-reversible `[REDACTED]`). Multi-id `/unmask` remains supported but is processed sequentially per document instead of merged.

## Primary User Flow

1. Starting point: a consumer posts text to `POST /api/v1/shield/mask` and receives masked text + `mask-id`/`data_mask_id` headers (unchanged).
2. Main interaction: the masked body shows clean `[MASK.1]` tokens with no document id; to restore, the caller passes the id(s) from the headers to `POST /api/v1/shield/unmask?mask_ids=…`.
3. Outcome: no sensitive/correlatable identifier ever appears in masked payloads; restore works exactly as before for clean and legacy documents.
4. Failure/fallback path: an unknown or already-consumed id returns 404; a `redact` document cannot be restored (unmask returns a clear error); an invalid `format` value returns 400; a caller that relied on id-visible tokens can pass `format=id` for the previous behavior.

## User Stories

- P1 Story: "As a data consumer I want masked output to look clean and leak nothing about internal document ids, while still being restorable via the header id."
- P2 Story: "As an integrator that already parses `[MASK_<id>.<N>]` tokens I want `format=id` so nothing breaks until I migrate."

## MVP Slice

- Clean-default tokens `[MASK.<N>]` in `MaskFromResults`, `format` param wiring in the handler, serial per-doc unmask, and legacy-token restore. MVP must satisfy `AC-001`, `AC-002`, `AC-004`, `AC-005`.

## First Deployable Outcome

After the first implementation pass, `POST /api/v1/shield/mask` returns counter-only tokens by default, `?format=id` reproduces the legacy format, and `POST /api/v1/shield/unmask` restores both new and pre-existing entries. Deployable independently because only the standalone mask domain/handler surface changes.

## Scope

- `src/internal/domain/shield/mask/usecase.go` — token builder; clean format `[MASK.<N>]` (counter per document), legacy format retained; `UnmaskText` processes `maskIDs` sequentially per entry instead of merging into one map.
- `src/internal/api/mask_handler.go` — `format` query param (`clean` default, `id` legacy, anything else → 400); headers `mask-id` / `data_mask_id` unchanged.
- `src/internal/api/middleware/shield.go` — proxy conversation dictionary tokens are clean per-request counters (`[MASK.<N>]`, no id); the `X-Shield-Dict-Mask-ID` header and mapping semantics are unchanged.
- `src/internal/api/mask_handler_test.go`, `src/internal/domain/shield/mask/usecase_test.go` — new coverage for both formats, invalid param, legacy restore, sequential unmask.
- `examples/test-prompt.md` / Postman flow docs — reflect the default clean format (docs-level).
- Untouched: proxy/conversation shield tokens (`[[pii…]]` in `app/usecase/shield`), mask storage schema, `MaskEntry` shape, other endpoints.

## Context

- Standalone mask use case builds `[MASK_<docID>.<N>]` (`usecase.go:56`), where `docID` equals the header id (`data_mask_id`). Unmask replaces placeholders by exact keys from the stored `Replacements` map (`usecase.go:74-101`) — the mechanism is token-agnostic, which is what makes this backward compatible.
- Callers already receive the id out-of-band via headers, so embedding it in the body is redundant for reversibility.
- The conversation/proxy pipeline uses a separate scheme (`[[pii.<label>.<N>]]`) and is not part of this change.
- Pre-existing entries keep their legacy tokens in storage; `UnmaskText` restoring by exact stored token keys means legacy text continues to restore without migration.

## Dependencies

- `MaskStorage` interface and entries — unchanged (same keying by mask/document id).
- Detector registry / preprocessors — unchanged.
- None external.

## Requirements

- RQ-001 The `/mask` endpoint MUST accept a `format` query parameter with values `clean` (default), `id`, and `redact`; any other value MUST return 400 with a clear message.
- RQ-002 In `clean` mode, generated tokens MUST be `[MASK.<N>]` where `N` is a per-document counter — the token MUST NOT contain the document id or any nonce; headers `mask-id`/`data_mask_id` MUST keep returning the ids.
- RQ-003 In `id` mode, generated tokens MUST keep the legacy `[MASK_<docID>.<N>]` shape so existing integrators parsing tokens are unaffected.
- RQ-004 In `redact` mode, every detected fragment MUST be replaced with the fixed token `[REDACTED]` (no counter, no id); the entry MUST be stored as non-reversible and `/unmask` MUST reject restoring it.
- RQ-005 `/unmask` MUST keep its contract (`mask_ids` comma list) and MUST restore clean tokens; when multiple ids are given, entries MUST be applied per document sequentially (not merged into a shared map).
- RQ-006 /unmask MUST continue to restore legacy documents created before this feature (their stored tokens are `[MASK_<docID>.<N>]`).
- RQ-007 Masked output in clean and redact modes MUST NOT leak the document id into the response body — only the token and the (unchanged) response headers reference the id.
- RQ-008 In the proxy/conversation path, dictionary tokens MUST be clean per-request counters (`[MASK.<N>]`) with no id or nonce prefix; uniqueness within the request comes from the single per-request counter, the `X-Shield-Dict-Mask-ID` header keeps carrying the mapping id, and reversibility comes from the per-request mapping.

## Non-Goals

- No security/format parameter on the proxy path; PII tokens (`[[pii.<label>.<N>]]`) and the conversation payload shape stay untouched.
- No storage/migration rewrite of legacy entries (they keep their tokens and are restorable as-is).
- No new endpoint; `/mask` and `/unmask` paths and methods stay.

## Acceptance Criteria

### AC-001 Clean tokens by default, no document id in body

- Why this matters: masked payloads leak no internal correlation identifier.
- **Given** a text containing PII detected by the mask flow
- **When** a consumer posts it to `/mask` without a `format` param
- **Then** the body contains only `[MASK.<N>]` counter tokens (no `_<id>` fragment), the `mask-id`/`data_mask_id` headers are present, and the masked text is served as before.
- Evidence: handler/use-case unit test asserting the generated tokens match `^\[MASK\.[0-9]+\]$` and headers are set.

### AC-002 Legacy format via `format=id`

- Why this matters: integrators parsing `[MASK_<id>.<N>]` can migrate at their own pace.
- **Given** the same input
- **When** `/mask?format=id` is called
- **Then** tokens keep the legacy `[MASK_<docID>.<N>]` shape identical to the pre-feature output.
- Evidence: unit test asserting the token contains the document id and the same counter sequence as before.

### AC-003 Invalid format returns 400

- Why this matters: typos must not silently fall back to a different behavior.
- **Given** a `format` param not in `clean|id|redact` (e.g. `?format=plain`)
- **When** `/mask` is called
- **Then** the endpoint returns 400 with a message naming the allowed values, and no mask entry is saved.
- Evidence: handler test asserting 400 and empty storage.

### AC-004 Clean tokens restore via /unmask

- Why this matters: reversibility is preserved in the new format.
- **Given** a clean-masked document (tokens `[MASK.<N>]`) whose id is known
- **When** a consumer posts the masked body to `/unmask?mask_ids=<id>`
- **Then** all `[MASK.<N>]` tokens are replaced by their original fragments and the restored text equals the input.
- Evidence: use-case test round-tripping mask→unmask on the clean format.

### AC-005 Legacy entries still restore

- Why this matters: historical data must not break.
- **Given** a stored entry whose tokens are `[MASK_<id>.<N>]` (created before this feature)
- **When** `/unmask?mask_ids=<id>` is called with the legacy-styled text
- **Then** restore succeeds and reproduces the originals.
- Evidence: use-case test using the legacy token in an entry and asserting full restoration.

### AC-006 Multi-id unmask applies per document, not merged

- Why this matters: identical clean counters from different documents must not overwrite each other.
- **Given** two documents each masked in clean mode with tokens `[MASK.1]`… (distinct originals)
- **When** `/unmask?mask_ids=<a>,<b>` is called on a text containing only one document's tokens
- **Then** each entry's tokens are applied in sequence and the correct document's originals win; cross-document tokens in the same body do not cause wrong replacements.
- Evidence: use-case test with two entries applied sequentially, asserting document B does not re-replace tokens already resolved for document A.

### AC-007 Redact mode is non-reversible and id-free

- Why this matters: strict no-leak cases want output stripped of counter and id entirely, with restore explicitly forbidden.
- **Given** a text with detected fragments masked via `/mask?format=redact`
- **When** the response is inspected and then `/unmask?mask_ids=<id>` is attempted
- **Then** the body contains only fixed `[REDACTED]` tokens (no `[MASK…]` counter, no id fragment) and unmask returns a clear error (400) instead of restoring, while headers still carry the id.
- Evidence: handler/use-case tests asserting the body matches `^[REDACTED]$`-style tokens and that unmask on the entry fails with the non-reversible error.

### AC-008 Proxy dictionary tokens are clean counters

- Why this matters: conversations must not expose the mapping id; clean per-request counters hide it entirely while staying reversible.
- **Given** a chat request with a dictionary hit routed through the proxy
- **When** the request is masked and the `X-Shield-Dict-Mask-ID` header is read
- **Then** the masked text contains only `[MASK.<N>]` counter tokens (no id, no `_` fragment), the header still carries the mapping id, and the response unmask restores the originals.
- Evidence: middleware test asserting the clean token regexp and header presence; existing dict-unmask tests stay green.

## Assumptions

- Multi-id restore is a rare manual/ops path; per-document sequential application is the intended semantics (a mixed single body with tokens from two different documents is out of scope and is the documented limitation).
- `clean` is the correct new default; integrators that need id-visible tokens migrate by passing `format=id` (recorded in docs).
- The standalone `/mask` domain and the conversation shield pipeline are independent schemes; no shared token contract exists between them.

## Success Criteria

- SC-001 The default `/mask` response body contains zero occurrences of the `^[A-Za-z0-9-]{1,64}$` document id (grep/test evidence) for a representative PII corpus.
- SC-002 `go test ./...` for the `shield/mask` domain and API packages passes with the new seq parameters; legacy restore and clean restore both covered.

## Edge Cases

- No detections: masked text equals input (unchanged behavior; an empty entry is still saved and unmask is a no-op).
- Counter reset: `N` restarts at 1 for every reversible document (id and clean).
- Redact restore: unmask on a `redact` entry returns 400 "not reversible"; unknown ids still return 404.
- Empty/missing `format`: treated as `clean`.
- Whitespace ids in `mask_ids`: trimmed and skipped, as today.
- Unknown id on unmask: 404 (unchanged).
- Same document unmasked twice sequentially after its tokens were already replaced: no-op (tokens absent → no change).

## Open Questions

- ~~`redact` mode~~ **Resolved**: included in this feature (`format=redact`, non-reversible `[REDACTED]`).
- ~~Parameter naming~~ **Resolved**: single `format` param with `clean|id|redact` values.
- Default: `clean` as default (accepted in design discussion) — deployed consumers that depended on id-visible tokens must pass `format=id`; flagging for awareness, no further change expected.