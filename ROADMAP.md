# MaskChain Roadmap — Self-hosted AI Data Gateway

**A platform for reversible data masking in AI traffic — a gateway that sees your data first and releases it last.**

```
Status:  v1.0 — Content Shield DLP + Routing + Multi-Tenancy ✅
         v2.0 — Sessions + Analytics + Platform Maturity ✅
         v3.0 — Virtual Keys + Budgets + Routing CRUD ✅
Strategy: v4.0 — Depth-first (differentiation in DLP) → Parity (enterprise) → Agent wave
```

---

## Positioning (strategy anchor)

MaskChain is a **self-hosted AI data gateway**: a single control point for LLM traffic where data protection is the core mission, not a by-product. The foundation is **reversible masking** (reversible mask + streaming unmask + per-tenant dictionaries), on top of which routing, limits, budgets, and analytics are built.

Differentiation is built not on provider breadth (where we deliberately trail), but on **depth of data control** — something neither router-gateways, nor enterprise DLP, nor guardrail libraries offer.

### Principles (from the constitution, reaffirmed)

1. **Content Shield — core domain**: nothing new may weaken DLP.
2. **Tenant-driven policies**: every extension is tenant-scoped.
3. **Extensibility over hardcoding**: plugins, interfaces, adapters.
4. **Native-only data plane**: Go, no external runtime dependencies.
5. **AI traffic is network traffic**: passthrough, not translation.
6. **Infrastructure, not chatbot**: no agent framework, no prompt playground.
7. **Zero-retention by default**: the "log or don't log" balance is configured per tenant.

---

## Market: four competitor categories (2026)

| Category | Representatives | Strength | Weakness (our niche) |
|---|---|---|---|
| **Router gateways** | LiteLLM (100+ providers), Portkey (acquired by Palo Alto), Kong AI Gateway, Higress, TrueFoundry | Provider breadth, integrations, enterprise features | Shallow DLP; masking = NER-redact; no reversible; no streaming unmask; enterprise features paid/closed |
| **Enterprise DLP** | Nightfall, Strac, Google Cloud DLP | Detection depth, many channels (SaaS, storage) | AI is one channel among many; **no mask-and-restore** (redact only); per-scan pricing; no SSE awareness; heavy integration |
| **Guardrail libraries** | LLM Guard, Guardrails AI, Lakera Guard | Field-level scanning, code-first | Not a gateway, doesn't watch every path; reasons about "the prompt", not "the stream" |
| **AI security proxy / privacy-tools** | Bifrost, PasteGuard, CloakPipe, Privacy-filter | Aware of AI traffic | One feature (PII or routing), no full loop: DLP + gateway + BI |

### Key 2026 trends shaping the roadmap

1. **Sovereignty is a first-order axis.** DPDP (India), PDPL (Saudi Arabia), GDPR + self-host/air-gapped are becoming baseline requirements. A self-hosted Go binary with no Python dependencies is a strong position.
2. **Consolidation into security vendors.** Portkey → Palo Alto. The category is moving upmarket under enterprise sales. Our bet: an independent self-hosted project priced on performance, predictability, and an open roadmap targets the same demand — but from below.
3. **Mask-and-restore is an unclaimed niche.** Neither DLP (redact-only) nor gateways (NER-only) offer **reversible** masking. This is the lock-in mechanism: once integrated, switching off means losing data.
4. **Gateways become the gateway to agent infrastructure.** MCP (TrueFoundry native, Kong plugin, Higress) and A2A are growing. Masking data exchanged between agents is a natural extension of our theme.

---

## Already done (verified in code)

| Block | Status | Evidence |
|---|---|---|
| **Content Shield** — PII/PHI/finance/secrets/dictionary, reversible mask, streaming unmask, Aho-Corasick, block/redact/mask/alert, CSV/JSON preprocessors | ✅ | `src/internal/domain/shield/`, `detector/` |
| **Prompt injection detector** | ✅ (already present!) | `src/internal/domain/shield/detector/promptinjectiondetector.go` |
| **Routing & Egress** — provider registry, fallback, health-aware, CB, retry, per-provider proxy, SSE, CRUD API + UI | ✅ | `/api/v1/routing/*`, `provider_health.go` |
| **Multi-Tenancy** — API key→tenant, tenant policies, rate limiting (Valkey) | ✅ | `src/internal/domain/tenant/`, `middleware/auth.go` |
| **Virtual Keys** — entity, auth middleware, model access, Admin API, migration backfill | ✅ | `src/internal/domain/virtualkey/`, `middleware/virtualkey_auth.go` |
| **Budgets + Spend** — entity, Valkey counters, middleware enforcement, webhook alerts, aggregation worker, Admin API + UI (Budgets page) | ✅ | `src/internal/domain/budget/`, `app/budget/agg_worker.go`, `handler/admin/budget_handler.go` |
| **Cost Rates** — store + Admin API | ✅ | `cost_rate_store.go`, `cost_rate_handler.go` |
| **Observability** — OTel, Prometheus, structured logs, health probes | ✅ | `61-observability` archived |
| **Admin UI** — React SPA: tenants, policies, dictionaries, incidents, Routing CRUD, Keys, Budgets | ✅ | `ui/src/pages/` |
| **Platform** — single Go binary (~18MB), 3 Dockerfiles, Helm, CI/CD, 14 linters, audit trail | ✅ | `deployments/`, `src/internal/api/handler/admin/audit_handler.go` |

**Conclusion:** phases 1–2 of the old v3.0 roadmap (Virtual Keys, Budgets, Cost Rates) are effectively shipped in code. The next line forms wave 4.0.

---

## Gap analysis: what's missing

Next to each row — where competitors have it (source: 2026 market monitoring).

| Gap | Where competitors have it | Priority |
|---|---|---|
| **Semantic cache over masked data** | Portkey, Higress, TrueFoundry, LiteLLM (kπcache) | 🔴 diff+ops |
| **Compliance packs (HIPAA/PCI/GDPR presets)** | Nightfall (20+ frameworks), Strac | 🔴 diff+sales |
| **Zero-retention mode** | Grepture/Prompt Security, TrueFoundry air-gap | 🔴 diff (regulated) |
| **Moderation detector (hate/sexual/violence)** | LiteLLM guardrails, OpenAI Moderation API | 🟠 diff |
| **Context-aware NER** (full prompt + history, not fragments) | NER engines (spaCy/Presidio in PasteGuard) | 🟠 diff |
| **Hierarchical RBAC (admin/ops/viewer), teams** | TrueFoundry, Kong Konnect, LiteLLM enterprise | 🟠 adoption |
| **SSO / OIDC (Keycloak, Azure AD, Okta)** | LiteLLM enterprise, Kong Konnect, TrueFoundry | 🟠 adoption |
| **Log export (Langfuse, S3/GCS, Datadog, webhook)** | LiteLLM (15+), Portkey, Higress | 🟠 adoption |
| **Provider registry (YAML definitions, community)** | LiteLLM (100+), Higress | 🟠 ecosystem |
| **Embeddings / vision / audio endpoints** | LiteLLM, Portkey, Kong | 🟠 parity |
| **Weighted routing, A/B, traffic mirroring** | LiteLLM, Higress | 🟢 advanced |
| **Config hot-reload / dynamic routing (zero-downtime)** | Higress (Envoy, ms-level), Kong | 🟢 ops |
| **MCP / A2A gateway (agent data protection)** | TrueFoundry (native), Kong (plugin), Higress | 🟢 agent wave |
| **Cost-rates auto-update** | LiteLLM community defs | 🟢 ecosystem |
| **Multi-region / HA** | TrueFoundry, Kong | 🟢 scale |

---

## Roadmap v4.0 — Depth-first

Order: **differentiation (4.0) → parity (4.1) → agent wave (4.2)**. The parity block runs in parallel in the background as capacity allows.

### Wave 4.0 — Depth (diff-first). Priority: 🔴🟠

#### 400-semantic-cache-masked
**Problem:** repeated requests cost tokens and budget; competitors already cache.
**Insight:** we cache **masked data** — masking and caching are compatible (placeholders are deterministic), which competitors can't do.
| Artifact | Description |
|---|---|
| `SemanticCache` entity | Index by masked embedding (vector) or exact masked-query hash |
| Embedding source | Self-contained (statistical/online) or OpenAI-compatible — configurable |
| `SemanticCacheMiddleware` | Check on ingress; serve from cache only when masks match and policy allows caching |
| TTL, per-tenant enable, budget-safe (no caching when limit is close) | — |
| Metrics | `maskchain_cache_hit_rate`, token savings |

#### 401-compliance-packs
**Problem:** regulated customers (HIPAA/PCI/GDPR) expect ready-made rule sets; today everything is configured by hand.
| Artifact | Description |
|---|---|
| `CompliancePack` YAML | "HIPAA", "PCI DSS", "GDPR", "Legal" — presets: which detectors, reactions, resolutions, masking |
| UI | Apply a pack to a tenant in one click; customize on top |
| Reports | Compliance check per pack: which rules are active/deviated |

#### 402-zero-retention-mode
**Problem:** regulated customers require "don't store prompts at all".
| Artifact | Description |
|---|---|
| Tenant mode `retention: none` | Prompts/responses are not written to session/log; only aggregated counters (tokens, masks, redirects) |
| Fallback logging | Blocking anomalies logged with anonymized facts only (detector, category, nothing more) |
| Config | `data.retention: {mode: full|meta|none}` per tenant |

#### 403-moderation-detector
| Artifact | Description |
|---|---|
| `ModerationDetector` | OpenAI Moderation API (or self-hosted) as a Detector in the pipeline |
| Policy action per category | `action_on_hate: block`, `action_on_sexual: mask` |
| Enables | Content safety without building our own NER pipeline |

#### 404-context-aware-nlp
| Artifact | Description |
|---|---|
| `ContextAwareDetector` | Analysis of the full prompt + previous turns (session history) |
| Sliding window N tokens | Data leakage through context |
| NLP-based PII | spaGO / external NER (optional, off by default) — not in the hot data-plane path |

### Wave 4.1 — Parity (adoption). Priority: 🟠🟢

#### 410-sso-oidc
| Artifact | Description |
|---|---|
| `JWTValidator` + `OIDCProvider` | RS256/ES256, JWKS cache, `iss/aud/exp` |
| Admin UI login via IdP | Keycloak, Azure AD, Okta |
| Design | MaskChain is NOT an IdP; SAML via oauth2-proxy optionally (principle 6) |

#### 411-rbac-teams
| Artifact | Description |
|---|---|
| `Role` entity | admin / ops / viewer (per tenant) |
| Rights model | Restrict CRUD endpoints by role; audit of role changes |
| Teams | Group tenants; grant permissions on the group |

#### 412-log-exporters
| Artifact | Description |
|---|---|
| `LogExporter` port | webhook, S3/GCS (batch, JSONL/Parquet), Datadog, Langfuse-compatible |
| Sampling | per-exporter `sample_rate`; sanctioned events always 100% |
| Enrichment | X-Request-ID, session, virtual key label, route decision, shield verdict |

#### 413-provider-registry-plugin
| Artifact | Description |
|---|---|
| `ProviderDefinition` YAML | api_type, base_url, auth_scheme, supported endpoints, models |
| `DynamicProviderClient` | Passthrough client driven by metadata (no translation — principle 5) |
| Community defs | `providers/` + `routing.providers.definitions_path` |
| CLI | `maskchain provider add <name> --api-type openai --base-url ...` |

#### 414-more-api-endpoints
| Artifact | Description |
|---|---|
| `/v1/embeddings`, `/v1/images/generations`, `/v1/audio/*` | Passthrough + optional shield scan of text fields |
| `/v1/completions` | ✅ (already present) — extend with a model aggregator |
| `/v1/models` | Proxy aggregating models of active providers |

#### 415-routing-advanced
| Artifact | Description |
|---|---|
| Weighted routing | `providers[].weight`, reservoir sampling |
| Traffic mirroring | `mirror: {provider, sample_rate}` → fire-and-forget + mirror_log |
| Router plugins | `RoutingPlugin` pipeline: cost-optimizer, latency-prioritizer, WASM-host |

#### 416-config-hot-reload
| Artifact | Description |
|---|---|
| Dynamic reload | routing/config without restart (SIGHUP + FS watch) |
| Validate before apply | atomic config swap |

#### 417-cost-rates-auto
| Artifact | Description |
|---|---|
| Community price tables | Update cost-rates from external defs |
| Fallback | Estimate from `input_price/output_price` in provider config |

### Wave 4.2 — Agent & Scale. Priority: 🟢

#### 420-mcp-gateway
| Artifact | Description |
|---|---|
| MCP detection | Content-Type `application/vnd.mcp+json`, path `/mcp/` |
| Proxy + shield | Tool-call content scanned; per-tenant tool blacklist/whitelist |
| Unmask on egress | Tool responses carry `@sk-masked` — auto-restore for the client |

#### 421-a2a-gateway
| Artifact | Description |
|---|---|
| A2A registration | `agents: [{agent_name, url, capabilities}]` |
| Body DLP | Scan data exchanged between agents at the boundary |

#### 422-ha-multiregion
| Artifact | Description |
|---|---|
| Stateless layer | Gateway replicas, shared Valkey/PG |
| Leader election for workers | Aggregation/cleanup workers run on one replica |
| Load test | Public benchmark (gateway path latency overhead) |

---

## Development order

```
v4.0 Depth-first (now):
  400-semantic-cache-masked ─── key diff+ops win, fast payoff
  401-compliance-packs      ─── diff+sales, quick demos
  402-zero-retention-mode   ─── required for regulated (sales)
  403-moderation-detector   ─── optional, cheap
  404-context-aware-nlp     ─── after 403 (shared pipeline)

v4.1 Parity (in background, as ready):
  410-sso-oidc  →  411-rbac-teams  →  412-log-exporters
  413-provider-registry → 414-more-api-endpoints
  415-routing-advanced  →  416-config-hot-reload  →  417-cost-rates-auto

v4.2 Agent & Scale:
  420-mcp-gateway → 421-a2a-gateway
  422-ha-multiregion (on demand)
```

Priority: **400 → 401 → 402** (sales + diff). In parallel a minimal parity block (410, 413) so we don't lose adoption in LiteLLM comparisons.

---

## Differentiation strategy

| Area | MaskChain advantage | Action |
|---|---|---|
| **Reversible mask + streaming unmask** | Unique in the category | Deepen (400, 401, 402) |
| **Per-tenant dictionary (4 match modes)** | No one replicates it | Marketing |
| **Self-hosted Go, ~18MB, <100ms** | No Python dependencies, air-gap ready | Preserve |
| **Prompt injection detector (native)** | Already present, rare in gateways | Surface in marketing |
| **Virtual keys + budgets + cost-rates** | Enterprise features free in OSS | Preserve |
| **Ecosystem (providers)** | Lagging 6 vs 100+ | Parity 413, 417 |
| **Zero-retention / compliance** | Regulated niche | 401, 402 |

### Deliberate no-gos

- Translation between provider formats (except gemini/bedrock) — passthrough only.
- Agent framework / agent runtime — protocol proxying only.
- Prompt playground / chatbot UI — infrastructure product.
- SSO/SAML in the core — SAML via external proxy.
- Low-code / visual workflow.
- Custom NER in the hot data-plane path without justification.

---

## v4.0 success metrics

| Metric | Target | Wave |
|---|---|---|
| Semantic cache hit rate | ≥40% on repeat requests | 4.0 |
| Spend savings via cache | ≥25% of repeat traffic | 4.0 |
| Tenants with compliance pack | 100% new, ≥50% existing | 4.0 |
| Zero-retention usage | ≥1 regulated case (demo) | 4.0 |
| Provider coverage | 50+ definitions | 4.1 |
| SSO enabled | ≥1 IdP (Keycloak) in CI/demo | 4.1 |
| MCP e2e | Working MCP proxy with unmask | 4.2 |
| Gateway latency | Overhead <15% at p95 | 4.2 |