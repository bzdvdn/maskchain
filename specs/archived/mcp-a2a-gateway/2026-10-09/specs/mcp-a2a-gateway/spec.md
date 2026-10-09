# Agent Protocol Gateway (MCP + A2A)

## Scope Snapshot

- In scope: proxy MCP (Model Context Protocol) and A2A (Agent-to-Agent) traffic through the gateway with tenant auth, budgets, and reversible Content Shield masking of tool/message text.
- Out of scope: building an agent runtime or tool executor, provider-format translation of agent payloads, and non-HTTP agent transports.

## Goal

Operators route agent traffic (MCP tool calls and A2A messages) through MaskChain like any other LLM traffic. A configured upstream MCP server and A2A agent are exposed behind tenant-authenticated endpoints; JSON-RPC requests are forwarded, the shield masks text in `tools/call` arguments and A2A message parts before the upstream sees them, and tool results / agent replies are unmasked back to the caller. Tool access is tenant-scoped via an allow/block list. Success is visible when an MCP client calls a tool and the upstream never sees the original values, while the client gets them restored.

## Primary User Flow

1. Starting point: an MCP client calls `POST /mcp` with a JSON-RPC `tools/call` request; an A2A client calls `POST /a2a` with a `message/send` request.
2. Main interaction: the gateway authenticates the key, checks model/tool access and budget, masks the tool arguments / message text per tenant policy, and forwards the JSON-RPC request to the configured upstream.
3. Outcome: the client receives the upstream JSON-RPC response with tool-result / message text restored (matching request/response ids preserved).
4. Failure/fallback path: a disallowed tool returns an explicit error before the upstream call; an upstream that streams (MCP SSE) is relayed with the same unmasking.

## User Stories

- P1 Story: as an integrator, my MCP client works through MaskChain with DLP on tool arguments and results.
- P2 Story: as an integrator, my A2A client works through MaskChain with DLP on message text.
- P3 Story: as an operator, I allow/block specific tools per tenant and track spend for agent traffic.

## MVP Slice

- `POST /mcp` JSON-RPC passthrough with `tools/call` argument masking and tool-result unmasking, plus a tenant tool allow/block check. Must satisfy `AC-001`, `AC-002`, `AC-003`, `AC-004`.

## First Deployable Outcome

- A configured MCP upstream is reachable at `POST /mcp`; a `tools/call` with PII reaches the upstream masked and the client gets originals back.
- Demonstrable with a recording mock MCP server.

## Scope

- `POST /mcp` (and `/mcp/`) exposing a configured upstream MCP server.
- `POST /a2a` and `GET /.well-known/agent.json` exposing a configured A2A agent.
- JSON-RPC 2.0 request/response passthrough preserving ids and non-text fields.
- Reversible masking of `tools/call` arguments and tool-result text; A2A message text.
- Per-tenant MCP tool allow/block list.
- Auth, model/tool access, budget; streaming (SSE) relay with unmasking where the upstream streams.

## Context

- The gateway already proxies OpenAI-shaped HTTP with auth, model access, budgets, shield (reversible), and SSE streaming; MCP/A2A are JSON-RPC over HTTP, so they need their own request/response shape handling but reuse auth/budget/shield/path infrastructure.
- The shield's reversible mapping (`maskTexts` → placeholder→original) and the unmask writer/stream carry buffer already exist (chat + Responses).
- MCP "streamable HTTP" returns JSON or an SSE stream for a single `POST`; A2A uses JSON-RPC with an agent card for discovery.
- The constitution is explicit: protocol proxying only, no agent runtime (principle 6).

## Dependencies

- Virtual-key auth, budget, the shield engine and mask mapping, the streaming egress client, and the responses streaming carry-buffer unmasker (all shipped).

## Requirements

- RQ-001 `POST /mcp` MUST forward the JSON-RPC request to the configured upstream MCP server and return its response unchanged except for unmasking.
- RQ-002 The shield MUST mask text in `tools/call` `params.arguments` (string values, recursively) before the upstream call.
- RQ-003 Tool-result text (MCP `result.content[].text`, and JSON-stringified structured content) MUST be unmasked to the original values.
- RQ-004 A `tools/call` for a tool not allowed for the tenant MUST be rejected with an explicit JSON-RPC error and MUST NOT reach the upstream.
- RQ-005 MCP SSE responses MUST be relayed with the same per-event unmasking and carry-buffer behavior as Responses.
- RQ-006 `POST /a2a` MUST forward the JSON-RPC request to the configured upstream agent; A2A message part text MUST be masked on the request and unmasked on the reply.
- RQ-007 `GET /.well-known/agent.json` MUST return the configured agent card (passthrough).
- RQ-008 The gateway MUST enforce virtual-key auth and the pre-request budget check on all agent endpoints.
- RQ-009 JSON-RPC `id`, `method`, `jsonrpc`, and non-text fields MUST be preserved exactly.

## Non-Goals

- Executing tools or agents inside MaskChain.
- Translating MCP↔A2A↔OpenAI formats.
- Managing MCP server registries / A2A agent directories beyond a small configured set.
- Non-HTTP transports (stdio, WebSocket) and MCP resources/prompts beyond passthrough.
- An agent UI beyond configuration.

## Acceptance Criteria

### AC-001 MCP tools/call is proxied

- Why this matters: MCP clients must work unchanged.
- **Given** a configured upstream MCP server and a key
- **When** the client sends `POST /mcp` with a `tools/call` JSON-RPC request
- **Then** the upstream receives the request and the client receives the upstream response with the same `id`.
- Evidence: a recording mock shows the method and id; the client response echoes the id.

### AC-002 Tool arguments are masked

- Why this matters: tool arguments can carry PII/secrets.
- **Given** a tenant whose policy masks emails and a `tools/call` whose `arguments` contain an email
- **When** the request is forwarded
- **Then** the upstream sees a placeholder instead of the email.
- Evidence: the mock's received `arguments` contain a placeholder, not the email.

### AC-003 Tool results are unmasked

- Why this matters: the client must see original values.
- **Given** the masked call and an upstream result echoing the placeholder in `content[].text`
- **When** the client receives the result
- **Then** the text contains the original email.
- Evidence: the client response contains the email; no placeholder remains.

### AC-004 Disallowed tools are rejected

- Why this matters: operators restrict tool access per tenant.
- **Given** a tenant whose allow/block list excludes tool `x`
- **When** the client calls `tools/call` for `x`
- **Then** the gateway returns a JSON-RPC error (e.g. `-32601`/custom code) and makes no upstream call.
- Evidence: error response with the code; zero upstream calls.

### AC-005 MCP SSE is relayed and unmasked

- Why this matters: MCP can stream results.
- **Given** an upstream returning an SSE stream with a placeholder in a result event
- **When** the client reads the stream
- **Then** it receives the original values with valid event framing.
- Evidence: collected events contain the original and no partial placeholder.

### AC-006 A2A messages are masked and unmasked

- Why this matters: A2A carries user data.
- **Given** an upstream A2A agent and a `message/send` with text parts
- **When** the client sends it
- **Then** the upstream receives masked part text and the client receives the reply with originals.
- Evidence: the mock shows masked input; the client reply shows the original.

### AC-007 Agent card passthrough

- Why this matters: A2A discovery must work.
- **Given** a configured agent
- **When** the client GETs `/.well-known/agent.json`
- **Then** the configured agent card is returned.
- Evidence: the response body equals the configured card.

### AC-008 Auth and budget apply

- Why this matters: agent endpoints must not bypass guardrails.
- **Given** a missing/invalid key or an over-budget tenant
- **When** any agent endpoint is called
- **Then** the gateway returns 401/429 and makes no upstream call.
- Evidence: statuses; the mock records zero calls.

### AC-009 JSON-RPC envelope is preserved

- Why this matters: clients correlate by `id` and dispatch by `method`.
- **Given** a request with `jsonrpc`, `id`, `method`, and non-text params
- **When** it is forwarded
- **Then** those fields reach the upstream unchanged.
- Evidence: the mock's received body has identical `jsonrpc`/`id`/`method`/non-text params.

## Assumptions

- Upstream MCP/A2A endpoints are configured (URL + auth) and OpenAI-style auth/budget uses the virtual key.
- MCP uses streamable HTTP JSON-RPC (single POST; JSON or SSE response); A2A uses JSON-RPC over HTTP with an agent card.
- Tool arguments and message parts are text-bearing JSON; binary parts are passed through.
- One upstream MCP server and one A2A agent per configuration (multi-upstream routing is a follow-up).

## Success Criteria

- SC-001 Proxying adds no measurable latency beyond the shield scan for JSON responses.
- SC-002 Existing suites pass; LLM endpoints unchanged.

## Edge Cases

- `tools/call` with no arguments: forwarded unchanged, no masking.
- Nested/structured arguments: string values masked recursively; non-string values untouched.
- Upstream error result: status/body surfaced without unmasking.
- Client disconnect on an SSE stream: upstream stream cancelled.
- Unknown JSON-RPC method: proxied (passthrough), not rejected locally.

## Open Questions

- Configuration surface: `mcp:`/`a2a:` YAML only, or admin UI too?
- Routing: a single upstream per protocol, or multiple with path/tenant selection?
- Should A2A `tasks/get` polling responses also be unmasked, or only `message/send`?
- Which JSON-RPC error code for a disallowed tool (custom vs `-32601`)?
- Do we unmask tool-result `structuredContent` recursively as well as `content[].text`?
