# Agent Protocol Gateway (MCP + A2A) Tasks

## Phase Contract

Inputs: `specs/active/mcp-a2a-gateway/plan.md`, `spec.md`.
Outputs: ordered executable tasks with coverage mapping.
Stop if: tasks would be vague or acceptance coverage cannot be mapped.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/infra/config/config.go | T1.1 |
| src/internal/infra/config/defaults.go | T1.1 |
| src/internal/infra/config/validator.go | T1.1 |
| src/internal/infra/config/config_test.go | T1.3 |
| src/internal/api/responses_handler.go | T1.2 |
| src/internal/api/responses_handler_test.go | T1.3 |
| src/internal/api/middleware/agent_shield.go | T2.1 |
| src/internal/api/middleware/agent_shield_test.go | T2.3 |
| src/internal/api/agent_handler.go | T2.2 |
| src/internal/api/agent_handler_test.go | T2.3 |
| src/internal/api/server.go | T3.1 |
| src/cmd/gateway/run.go | T3.2 |
| src/cmd/all/gateway.go | T3.2 |
| src/internal/api/server_test.go | T3.3 |
| Makefile | T4.1 |

## Implementation Context

- MVP goal: `POST /mcp` JSON-RPC passthrough with `tools/call` argument masking, tool-result unmasking, and a tenant tool check (AC-001–AC-004, AC-009).
- Invariants/semantics: mask only MCP `params.arguments` (recursive strings) and A2A `params.message.parts[].text`; unmask the reply with a recursive string replace of placeholders (buffered for JSON, carry-buffer for SSE); JSON-RPC `jsonrpc`/`id`/`method`/non-text params are preserved; a disallowed tool is rejected before any upstream call; agent endpoints use auth → budget → agentShield → handler (no model access).
- Errors/codes: disallowed tool → JSON-RPC error (`-32601` "tool not allowed") with HTTP 200 JSON-RPC body; missing/invalid key → 401; over budget → 429; upstream non-2xx surfaced unchanged.
- Contracts/protocols: routes `POST /mcp`(+`/mcp/`), `POST /a2a`, `GET /.well-known/agent.json`; config `mcp:`/`a2a:` (upstream url + auth header/prefix/key) and per-tenant MCP tool policy.
- Scope boundaries: do not execute tools/agents; do not translate MCP↔A2A↔OpenAI; no DB-backed tool policy or agent UI; no stdio/WebSocket.
- Proof signals: extractor tests (MCP args, A2A parts, envelope untouched); recursive unmask test; tool-policy reject with zero calls; JSON + SSE relay tests; agent-card test; auth/budget gate tests.
- References: DEC-001..DEC-007, RQ-001..RQ-009 (no mandatory re-read).

## Phase 1: Foundation

Goal: configure upstreams and share the recursive unmask helper.

- [x] T1.1 Add `MCPConfig`/`A2AConfig` (upstream URL + auth) and per-tenant MCP tool policy to config with defaults and validation (DEC-005, DEC-006, AC-004, AC-006, AC-007). Touches: src/internal/infra/config/config.go, src/internal/infra/config/defaults.go, src/internal/infra/config/validator.go
      Proof: code src/internal/infra/config/config.go MCPConfig
- [x] T1.2 Generalize the streaming unmask into a reusable recursive helper (JSON in-place + carry buffer) usable by both Responses and the agent relay (DEC-003, DEC-004, AC-003). Touches: src/internal/api/responses_handler.go
      Proof: code src/internal/api/responses_handler.go unmaskJSONRecursive
- [x] T1.3 Prove the config sections parse and the recursive unmask restores nested text (AC-003, AC-006, AC-007). Touches: src/internal/infra/config/config_test.go, src/internal/api/responses_handler_test.go
      Proof: test src/internal/infra/config/config_test.go TestConfigAgentSections

## Phase 2: Agent shield and handler (MVP)

Goal: mask tool/message text and proxy MCP/A2A JSON-RPC.

- [x] T2.1 Implement the agent shield: extract and mask MCP `params.arguments` and A2A `params.message.parts[].text`, enforce the tenant tool policy, store the mapping in context, and unmask responses (buffered + carry) (DEC-002, DEC-003, DEC-005, AC-002, AC-003, AC-004, AC-006, AC-009). Touches: src/internal/api/middleware/agent_shield.go
      Proof: code src/internal/api/middleware/agent_shield.go AgentShieldMiddleware
- [x] T2.2 Implement `AgentHandler`: forward JSON-RPC to the configured upstream via the egress client, relay JSON and SSE (with unmask), and serve the A2A agent card (DEC-001, DEC-004, DEC-006, AC-001, AC-005, AC-006, AC-007, AC-009). Touches: src/internal/api/agent_handler.go
      Proof: code src/internal/api/agent_handler.go Handle
- [x] T2.3 Add tests for the shield extractors/policy and the handler JSON/SSE/card/envelope behavior (AC-001, AC-002, AC-003, AC-004, AC-005, AC-006, AC-007, AC-009). Touches: src/internal/api/middleware/agent_shield_test.go, src/internal/api/agent_handler_test.go
      Proof: test src/internal/api/middleware/agent_shield_test.go TestAgentShieldMasksMCPArguments

## Phase 3: Routes, wiring, gates

Goal: mount the endpoints and enforce auth/budget.

- [x] T3.1 Register the agent shield and handler and mount `/mcp`, `/mcp/`, `/a2a`, `/.well-known/agent.json` on both prefixes with the auth → budget → agentShield → handler chain (DEC-007, AC-001, AC-008). Touches: src/internal/api/server.go
      Proof: code src/internal/api/server.go RegisterAgentRoutes
- [x] T3.2 Construct/register the agent handler + shield in both binaries (AC-001). Touches: src/cmd/gateway/run.go, src/cmd/all/gateway.go
      Proof: code src/cmd/gateway/run.go wireAgentGateway
- [x] T3.3 Add route/gate tests: routes mount when configured, auth 401 / budget 429 with zero upstream calls (AC-008). Touches: src/internal/api/server_test.go
      Proof: test src/internal/api/server_test.go TestAgentRoutesMountAndGate

## Phase 4: Validation

Goal: prove the feature end to end and leave the package reviewable.

- [x] T4.1 Run the affected suites and `make test`; fix regressions (SC-001, SC-002). Touches: Makefile
      Proof: chore Makefile test

## Acceptance Coverage

- AC-001 -> T2.2, T2.3, T3.1, T3.2
- AC-002 -> T2.1, T2.3
- AC-003 -> T1.2, T1.3, T2.1, T2.3
- AC-004 -> T1.1, T2.1, T2.3
- AC-005 -> T2.2, T2.3
- AC-006 -> T1.1, T2.1, T2.2, T2.3
- AC-007 -> T1.1, T2.2, T2.3
- AC-008 -> T3.1, T3.3
- AC-009 -> T2.1, T2.2, T2.3

## Notes

- MVP is Phase 2 for `POST /mcp`; MCP SSE and A2A land in the same phases (they reuse the same shield/handler).
- Tenant tool policy is config-driven for the MVP; DB/UI parity is a documented follow-up.
- Every `[x]` task needs a `Proof:` line on the next line; no proof means not done.
