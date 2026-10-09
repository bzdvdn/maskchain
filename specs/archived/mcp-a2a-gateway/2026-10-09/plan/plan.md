# Agent Protocol Gateway (MCP + A2A) Plan

## Phase Contract

Inputs: `specs/active/mcp-a2a-gateway/spec.md` and targeted repo context.
Outputs: `plan.md` (data model noted inline).
Stop if: the spec is too vague to plan safely.

## Goal

Expose a configured upstream MCP server at `POST /mcp` and an A2A agent at `POST /a2a` (+ `/.well-known/agent.json`), behind virtual-key auth and budget. A shield masks MCP `tools/call` arguments and A2A message text on the request and restores placeholders in the reply, reusing the existing mask mapping and streaming unmask carry buffer. A tenant tool policy rejects disallowed MCP tools before the upstream call.

## MVP Slice

- `POST /mcp` JSON-RPC passthrough with `tools/call` argument masking, tool-result unmasking, and the tenant tool check. Must satisfy `AC-001`–`AC-004`, `AC-009`.

## First Validation Path

1. Unit-test the agent extractor/masker (MCP arguments, A2A parts) and the recursive unmask.
2. Recording-mock test: `tools/call` reaches the upstream masked; the client gets originals.
3. `curl POST /mcp` against a local mock.

## Scope

- `POST /mcp` (and `/mcp/`) → configured MCP upstream; JSON and SSE responses.
- `POST /a2a` and `GET /.well-known/agent.json` → configured A2A agent.
- Reversible masking of MCP `tools/call` arguments and A2A message parts; recursive unmask of results.
- Tenant MCP tool allow/block check.
- Auth + budget on all agent endpoints.

## Performance Budget

- One body read/rewrite for the shield; SSE unmask keeps only a bounded carry buffer.
- No new per-request allocation beyond the shield path.

## Implementation Surfaces

- `src/internal/infra/config/config.go` — `MCPConfig`/`A2AConfig` (upstream url, auth header/prefix/key), tenant tool policy fields.
- `src/internal/infra/config/defaults.go`, `validator.go` — defaults and validation for the new sections.
- `src/internal/api/middleware/agent_shield.go` (new) — JSON-RPC extraction/masking, tenant tool check, mapping in context, JSON buffered unmask + SSE carry unmask.
- `src/internal/api/agent_handler.go` (new) — `AgentHandler` forwarding to the upstream via the egress client; JSON + SSE relay; agent-card passthrough.
- `src/internal/api/responses_handler.go` — generalize the stream unmask helper (recursive string replacement) for reuse (or export a shared helper).
- `src/internal/api/server.go` — `RegisterAgent*` + mount routes.
- `src/cmd/gateway/run.go`, `src/cmd/all/gateway.go` — construct/register the agent handler + shield.
- Tests: `agent_shield_test.go` (new), `agent_handler_test.go` (new), `server_test.go`, `config_test.go`.

## Bootstrapping Surfaces

- A configured upstream MCP/A2A endpoint must exist for the routes to mount.

## Architecture Impact

- Local: one shield + one handler + config sections.
- Cross-boundary: reuses the egress client, mask mapping, and streaming unmask.
- Compatibility: additive routes; no change to LLM endpoints or DB schema.

## Acceptance Approach

- AC-001 → handler forwards `tools/call`; surfaces `agent_handler.go`; proof: mock shows method/id; client echoes id.
- AC-002 → extractor masks `params.arguments`; surfaces `agent_shield.go`; proof: mock sees placeholder.
- AC-003 → recursive unmask of `result.content[].text`; surfaces `agent_shield.go`; proof: client sees original.
- AC-004 → tenant tool policy; surfaces `agent_shield.go`; proof: JSON-RPC error, zero upstream calls.
- AC-005 → SSE relay with carry unmask; surfaces `agent_handler.go`; proof: stream test, no partial placeholder.
- AC-006 → A2A message parts mask/unmask; surfaces `agent_shield.go`, `agent_handler.go`; proof: mock masked input, client original.
- AC-007 → agent card passthrough; surfaces `agent_handler.go`; proof: body equals configured card.
- AC-008 → auth/budget before upstream; surfaces `server.go`; proof: 401/429 with zero calls.
- AC-009 → envelope preserved; surfaces `agent_shield.go`; proof: mock body has identical `jsonrpc`/`id`/`method`/non-text params.

## Data and Contracts

- Data model: no change — no new tables or migrations. Agent endpoints and tenant tool policy come from config.
- Contracts: new routes `/mcp`, `/mcp/`, `/a2a`, `/.well-known/agent.json`; JSON-RPC 2.0 pass-through; config `mcp:`/`a2a:` sections.
- Reuse: `egress.Client`, `maskTexts` mapping, the Responses carry-buffer approach.

## Implementation Strategy

- DEC-001 Reuse `egress.Client` as the upstream HTTP client
  Why: it already handles retries, pooling, per-upstream proxy, and SSE streaming. Not: a bespoke HTTP client.
  Tradeoff: construct a transport per configured upstream.
  Affects: `agent_handler.go`.
  Validation: handler tests with a mock upstream.
- DEC-002 A dedicated agent shield extracts only designated text fields
  Why: masking must not touch `method`/`id`; MCP arguments and A2A parts are the text-bearing fields. Not: masking every string.
  Tradeoff: new field-specific extractors for two protocols.
  Affects: `agent_shield.go`.
  Validation: extractor unit tests.
- DEC-003 Reusable recursive unmask for the reply
  Why: placeholders can appear anywhere in the result (content, structuredContent, parts); a recursive string replace is precise because placeholders are unique. Not: enumerating every result path.
  Tradeoff: touches all string values of the reply (safe).
  Affects: `responses_handler.go` (shared helper), `agent_shield.go`, `agent_handler.go`.
  Validation: AC-003/AC-006 tests.
- DEC-004 SSE relay reuses the carry-buffer unmask
  Why: MCP streamable HTTP may stream; placeholders can split across events. Not: buffering the whole stream.
  Tradeoff: shared carry logic.
  Affects: `agent_handler.go`.
  Validation: AC-005 test.
- DEC-005 Tenant MCP tool policy from config
  Why: avoids a schema change and UI work for the MVP. Not: a DB-backed per-tenant tool table.
  Tradeoff: policy is YAML-managed; UI/DB is a follow-up.
  Affects: `config.go`, tenant entity/options, `agent_shield.go`.
  Validation: AC-004 test.
- DEC-006 Single configured upstream per protocol
  Why: satisfies the spec with minimal routing config; matches "one upstream per configuration". Not: multi-upstream routing.
  Tradeoff: multi-tenant/multi-upstream selection is deferred (spec Open Question).
  Affects: `config.go`, `agent_handler.go`.
  Validation: route tests.
- DEC-007 Agent endpoints require auth + budget but skip the model pipeline
  Why: JSON-RPC has no `model`; model access does not apply. Not: reusing model-access middleware.
  Tradeoff: a dedicated chain (auth → budget → agentShield → handler).
  Affects: `server.go`.
  Validation: AC-008 test.

## Incremental Delivery

### MVP (First Value)

- Config sections + agent shield (extract/mask/tool check/recursive unmask) + `AgentHandler` for `POST /mcp` JSON + routes.
- MVP readiness: `AC-001`–`AC-004`, `AC-009` pass.

### Iterative Expansion

- MCP SSE relay → `AC-005`.
- A2A (`/a2a` + agent card) → `AC-006`, `AC-007`.
- Auth/budget gate tests → `AC-008`.

## Sequencing Notes

- Config sections + validator first (handler depends on upstream config).
- Shield extractors + recursive unmask before the handler.
- Generalize the unmask helper in `responses_handler.go` before agent SSE reuse.
- Wire both binaries together.

## Risks

- Masking the wrong JSON-RPC field could corrupt the envelope.
  Mitigation: mask only designated fields; AC-009 test pins `method`/`id`.
- Recursive unmask over the whole reply could replace a coincidental placeholder-like string.
  Mitigation: placeholders are unique generated tokens.
- Tool policy stored only in config (no UI).
  Mitigation: documented; follow-up to add DB/UI parity.
- Upstream streaming differences.
  Mitigation: reuse the proven carry-buffer relay; AC-005 test.

## Rollout and Compatibility

- Additive routes; no migration/flag. Existing endpoints unchanged. Routes mount only when the corresponding upstream is configured.

## Validation

- Automated: `go test ./src/internal/api/... ./src/internal/infra/config/...` plus `make test`.
- New tests: extractors (MCP/A2A), recursive unmask, tool policy, JSON + SSE relay, agent card, auth/budget, envelope preservation.
- Proves: `AC-001`–`AC-009`; `DEC-001`–`DEC-007`.

## Constitution Compliance

- no conflicts. Manages only DLP/routing config (no tenant content); protocol proxying only, no agent runtime (principle 6); Content Shield applied with reversible masking; no new runtime dependencies.
