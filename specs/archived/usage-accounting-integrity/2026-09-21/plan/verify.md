---
report_type: verify
slug: usage-accounting-integrity
status: pass
docs_language: en
generated_at: 2026-09-21
---

# Verify Report: usage-accounting-integrity

## Scope

- snapshot: streaming (SSE) and legacy `/completions` traffic is now accounted for spend/usage, unrated models use a configurable fallback rate, and accounting gaps are observable.
- verification_mode: default
- artifacts:
  - CONSTITUTION.md
  - specs/active/usage-accounting-integrity/spec.md
  - specs/active/usage-accounting-integrity/plan.md
  - specs/active/usage-accounting-integrity/tasks.md
- inspected_surfaces:
  - src/internal/api/middleware/usage_capture.go
  - src/internal/api/middleware/budget.go
  - src/internal/api/middleware/usage.go
  - src/internal/api/server.go
  - src/internal/api/provider_handler.go
  - src/internal/domain/analytics/cost_rate.go
  - src/internal/infra/config/{config,defaults}.go
  - src/internal/infra/metrics/metrics.go
  - src/cmd/gateway/run.go, src/cmd/all/gateway.go

## Verdict

- status: pass
- archive_readiness: safe
- summary: all 13 tasks carry `Proof:` lines, 17 targeted tests pass, and every AC maps to passing tests plus inspected wiring; no AC-critical gap found.

## Checks

- task_state: completed=13, open=0 (verify-task-state.sh: `OK: all tasks are marked complete`; `PROOFS_MISSING=0`).
- acceptance_evidence:
  - AC-001 -> `TestBudgetMiddlewareStreamingRecordsSpend` (streamed spend 40) + `TestSSEUsageParserOpenAI`/`TestSSEUsageParserAnthropic` + wiring `budget.go:94-140`.
  - AC-002 -> `TestBudgetMiddlewareHardLimitRejects` (pre-flight 429) + streamed increment above; `TestCompletionsBudgetStageReturns429`.
  - AC-003 -> `TestUsageMiddlewareStreamingRecords` (streamed tokens 100/50 into metrics).
  - AC-004 -> `TestBudgetMiddlewareStreamingMissingUsage` + `TestUsageMiddlewareNoUsage` + `TestSSEUsageParserAbsent`; `metrics.UsageMissingTotal` used in budget.go:105 and usage.go:60.
  - AC-005 -> `TestCompletionsUsesFullChain` (403 disallowed, 200 allowed, usage+budget stages ran) + `TestCompletionsBudgetStageReturns429`; `server.go:144` mounts `/completions` on the chat chain.
  - AC-006 -> `TestCostRateRegistryResolve`/`TestCostRateRegistryFallbackCost`; `newCostRateRegistry` called at gateway/run.go:397,477 and all/gateway.go:282,357.
  - AC-007 -> `TestCostRateRegistryResolveMissing` (zero rate, zero cost); `metrics.CostRateMissingTotal` used in budget.go:116 and usage.go:71.
- implementation_alignment:
  - `wrapUsageCapture` sets the streaming flag and tees JSON or SSE without altering the client response.
  - `budget`/`usage` middlewares account after the response via `capture.Usage()`; absent usage increments the metric and records nothing.
  - `RoutingProxyHandler.WithStreamUsage` wired in both binaries; injection is limited to OpenAI-shaped paths in `withStreamUsage`.

## Errors

- none

## Warnings

- none

## Questions

- none

## Not Verified

- Live provider behavior: no network call was made, so real OpenAI/Anthropic streaming usage payloads and acceptance of `stream_options.include_usage` were not observed end-to-end (covered by unit fixtures).
- `newCostRateRegistry` helper has no dedicated unit test; verified by code inspection and the registry-level tests.
- The spend-history entry for a streamed request is exercised (RecordSpend is called in the same loop) but the streaming test asserts the counter, not the stored history entry.

## Next Step

- safe to archive
