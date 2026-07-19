# MaskChain Content Shield -- Architecture Deep Dive

## 1. Overview

The Content Shield intercepts LLM chat requests and responses at the HTTP middleware layer, scans for sensitive content across multiple detection dimensions (PII, PHI, financial data, secrets, per-tenant dictionary terms), and applies configurable reactions: masking (reversible), redaction (irreversible), alerting, or blocking. What makes it distinct from generic redaction tools is its **dual-path architecture**: dictionary terms are masked with request-scoped in-memory placeholders, while PII/PHI/financial/secret patterns flow through a registry-backed detector engine with a formal reaction pipeline. The shield integrates with OTel tracing, Prometheus metrics, session counters, and system-prompt injection to preserve LLM output fidelity.

## 2. Pipeline Architecture

```
Request
  |
  v
[Preprocessors] -- (JSON/CSV field-level masking, optional)
  |
  v
[Dictionary Scan] -- per-tenant word lists, 4 match modes
  |                   placeholder map stored in-memory on context
  v
[PII Engine Scan] -- regex detectors via ScanPipelineFactory
  |                   placeholder map merged into dict mapping
  v
[Reaction Pipeline] -- Block / Mask / Redact / Alert
  |
  v
[Response]
  |
  v
[Unmask Writers] -- streamDictUnmaskWriter (SSE) or dictUnmaskWriter (buffered)
                     replaces [MASK_*] / [[pii.*]] placeholders with originals
```

### Stages

- **Preprocessors** (optional, per-tenant config): `JSONProcessor` masks fields by JSONPath; `CSVProcessor` masks columns by name. Run before detection to reduce noise.
- **Dictionary Scan**: `DictionaryDetector` per dictionary, runs first. Prioritizes known terms so PII regexes don't false-positive on dictionary entries. Produces `[MASK_<ID>.<N>]` placeholders (dictionary name omitted — system prompt already tells the LLM all `[MASK_...]` are data tokens).
- **PII Engine Scan**: `ShieldEngine` wraps `ScanUseCase`, builds a `Pipeline` of `DetectorBinding`s from tenant `PIARule`s. Produces `[[pii.<label>.<N>]]` placeholders.
- **Reaction Pipeline**: `ApplyPolicyUseCase` evaluates `ScanResult` severity via `PolicyEvaluator`, selects a `Reaction` (allow/block/review/log). Executed by `DefaultReactionPipeline`.
- **Unmask**: On the response path, the middleware restores original text from the merged placeholder map. SSE streams are unmasked chunk-by-chunk; non-streaming responses are buffered then unmasked before flush.

## 3. Detection Layers

### 3.1 Regex Detectors

All implement the `detector.Detector` interface:

```go
type Detector interface {
    Scan(ctx context.Context, text string) ([]DetectorResult, error)
}

type DetectorResult struct {
    DetectorType string
    Fragment     string
    StartPos     int
    EndPos       int
    Confidence   float64
}
```

| Detector | Patterns | Validation |
|---|---|---|
| `PIIDetector` | email, phone, SSN (`\d{3}-\d{2}-\d{4}`), passport | Regex match only |
| `PHIDetector` | ICD-10 codes (`[A-TV-Z][0-9][0-9AB]\.?[0-9A-Z]{0,2}`) | Regex match only |
| `FinancialDetector` | credit card (13-19 digits), IBAN, SWIFT/BIC | Luhn check on card numbers |
| `SecretsDetector` | API key patterns (`sk-\|pk-\|bearer `), JWT (`eyJ...`), PEM private keys | Regex match only |

Detectors are registered in a `DetectorRegistry` keyed by `entity.DetectorType` (`regex`, `keyword`, `presidio`, `dictionary`). The registry is populated at startup and queried by `ScanPipelineFactory.BuildFromRules` using the tenant's PIARule type strings.

### 3.2 Dictionary Detector

`DictionaryDetector` wraps a per-tenant `dictionary.Dictionary` and supports four `MatchMode` values:

| Mode | Mechanism | Use case |
|---|---|---|
| `exact` | `strings.Index` loop, O(n*m) | Small, precise word lists |
| `contains` | Aho-Corasick trie automaton (`WordlistMatcher`) | Medium/large lists, substring matching |
| `regex` | `regexp.Compile` per entry | Pattern-based dictionary entries |
| `fuzzy` | Levenshtein distance threshold >= 0.8 | Typos, approximate matches |

The `Dictionary` entity accepts heterogeneous entries (strings, maps, arrays), flattened by `AllValues()` into unique string values. Dictionaries are loaded from the database per-tenant and attached to the `Tenant` entity.

### 3.3 Entropy Detection

**Not implemented.** No Shannon-entropy fallback exists in the current codebase. Detection is limited to regex patterns and dictionary matching.

## 4. Reaction Pipeline

After detection, `ApplyPolicyUseCase.Evaluate` maps `ScanResult` status to an `entity.Reaction`:

```go
type Reaction string

const (
    ReactionAllow  Reaction = "allow"
    ReactionBlock  Reaction = "block"
    ReactionReview Reaction = "review"
    ReactionLog    Reaction = "log"
)
```

The `DefaultReactionPipeline.Execute` dispatches to one of four executors:

```go
type ReactionPipeline interface {
    Execute(ctx context.Context, reaction entity.Reaction,
        result *entity.ScanResult, text string) (string, error)
}
```

| Reaction | Executor | Behavior |
|---|---|---|
| `block` | `BlockReaction` | Returns `ErrBlockedByPolicy` -- middleware returns 403 |
| `log` | `RedactReaction` | Logs via zap, returns text unchanged (redact placeholder replacement is TBD in middleware path) |
| `review` | `AlertReaction` | Logs structured event with severity |
| `allow` | (passthrough) | Returns text unchanged |

**Note:** The `MaskReaction` executor exists and logs the event but the actual masking/replacement logic for PII happens in `ScanUseCase.Scan` (inline replacement with `[[pii.<label>.<N>]]` placeholders) and in the middleware for dictionary terms. The reaction pipeline currently operates as a policy-decision point rather than the transformation engine.

## 5. Streaming Unmask

The middleware maintains a merged placeholder-to-original map (`dictMaskMapping`). On the response path, two writer wrappers restore original content:

### Non-streaming (`dictUnmaskWriter`)
- Buffers all `Write()` calls into a `bytes.Buffer`.
- On `flush()`, replaces all placeholders with originals via `strings.ReplaceAll`, then writes to the real `ResponseWriter`.
- Used when `chatReq.Stream == false`.

### SSE streaming (`streamDictUnmaskWriter`)
- Overrides `Write()` to replace placeholders chunk-by-chunk on every call.
- No buffering -- each SSE data frame is unmasked in real-time before reaching the client.
- Used when `chatReq.Stream == true`.

### Unmask scope
Both writers handle two placeholder families:
- `[MASK_<ID>.<N>]` -- from dictionary scans
- `[[pii.<label>.<N>]]` -- from PII engine scans

These are merged into a single `dictMaskMapping` map in the middleware before response processing.

## 6. System Prompt Injection

When placeholders are present in the modified request body, the middleware injects a system message instructing the LLM to:
1. Never modify or replace `[MASK_...]` tokens
2. Treat `[[pii...]]` tokens as redacted (do not guess)
3. Respond normally without mentioning masking

This prevents the LLM from corrupting or expanding on masked content.

## 7. Finding Model

```go
type Finding struct {
    DetectorType DetectorType
    Label        string
    Fragment     string
    StartPos     int
    EndPos       int
    Severity     value.Severity
}

type ScanResult struct {
    status    value.ScanStatus  // clean | suspicious | blocked | error
    scannedAt time.Time
    findings  []Finding
}
```

`ScanStatus` values control the middleware's response behavior:

| Status | Middleware action |
|---|---|
| `clean` | Pass through, unmask placeholders, increment session counters |
| `suspicious` | Pass through (or block if `ActionOnSuspicious == "block"`), unmask, increment |
| `blocked` | Return 403 with `X-Shield-Status: blocked`, abort |
| `error` | Return 502, abort |

Findings flow from detectors through `ScanUseCase` into `ScanResult`, then into OTel span attributes (`shield.findings_count`, `shield.status`, `shield.tenant`). Each finding also maps to a Prometheus histogram observation (`ShieldScanDuration`) and a profile counter increment (`ShieldProfilesEvaluated`).

## 8. Key Interfaces

```go
// Detector: scan text for sensitive patterns
type Detector interface {
    Scan(ctx context.Context, text string) ([]DetectorResult, error)
}

// Processor: preprocess structured data before scanning
type Processor interface {
    Name() string
    Process(data string, namespace string) *ProcessResult
}

// ReactionExecutor: apply a reaction to a scan result
type ReactionExecutor interface {
    Execute(ctx context.Context, result *entity.ScanResult, text string) (string, error)
}

// ReactionPipeline: route reaction to executor
type ReactionPipeline interface {
    Execute(ctx context.Context, reaction entity.Reaction,
        result *entity.ScanResult, text string) (string, error)
}

// MaskStorage: persist/retrieve mask entry mappings
type MaskStorage interface {
    Save(ctx context.Context, entry *MaskEntry) error
    Get(ctx context.Context, maskID string) (*MaskEntry, error)
    Delete(ctx context.Context, maskID string) error
}

// ScanPipelineFactory: build detection pipeline from tenant rules
type ScanPipelineFactory struct {
    registry *DetectorRegistry
}

func (f *ScanPipelineFactory) BuildFromRules(ctx context.Context,
    rules []entity.PIARule) (*Pipeline, error)

type Pipeline struct {
    Preprocessors []preprocessor.Processor
    Detectors     []DetectorBinding
}

type DetectorBinding struct {
    Interface detector.Detector
    Type      entity.DetectorType
    Label     string
    Severity  value.Severity
}

// Scanner: public API consumed by middleware
type Scanner interface {
    Scan(ctx context.Context, req ScanRequest) (*ScanResponse, error)
}
```

The `Scanner` interface is implemented by `ShieldEngine`, which delegates to `ScanUseCase`. `ScanUseCase` calls `ScanPipelineFactory.BuildFromRules` to construct the detector pipeline, then iterates `DetectorBinding`s, collecting `DetectorResult`s and building `Finding` + replacement maps.

## 9. Configuration

### Global (`ShieldConfig`)

```go
type ShieldConfig struct {
    ActionOnSuspicious string                       `mapstructure:"action_on_suspicious"`
    TenantModelMapping map[string]map[string]string `mapstructure:"tenant_model_mapping"`
}
```

- `ActionOnSuspicious`: either `"block"` (return 403) or anything else (pass through). Default: defined in defaults.go.
- `TenantModelMapping`: optional model override per tenant (not shield-specific but lives in ShieldConfig).

### Per-tenant (`PIIConfig` on `Tenant`)

```go
type PIIConfig struct {
    Enabled       bool      `json:"enabled"`
    DefaultAction string    `json:"default_action"`   // "block" on engine error
    Rules         []PIARule `json:"rules"`
}

type PIARule struct {
    Label   string `json:"label"`   // e.g. "email", "phone", "credit_card"
    Type    string `json:"type"`    // matches DetectorType in registry
    Pattern string `json:"pattern"`
    Action  string `json:"action"`  // "mask", "redact", "alert", "block"
}
```

### Per-tenant Dictionaries

Attached to `Tenant.Dictionaries()` as `[]*dictionary.Dictionary`. Each dictionary has:
- `Name`: logical name, used for diagnostics/logging only (not included in placeholder format)
- `MatchMode`: exact / contains / regex / fuzzy
- `Entries`: heterogeneous slice (strings, maps, arrays), flattened at scan time

Dictionaries are scanned first, before PII rules, so that known business terms are pre-masked and do not trigger false-positive PII hits.

## 10. Unique Characteristics vs Generic PII Redaction

1. **Dual-path scan**: dictionary detection runs before regex PII detection, with priority masking to avoid false positives on legitimate business terms.
2. **In-band unmask**: placeholders carry enough context (`[MASK_<ID>.<N>]`) for lossless restoration on the response path, even across SSE chunk boundaries.
3. **System prompt injection**: tells the LLM how to handle masked tokens, preventing it from hallucinating replacements or disclosing the masking mechanism.
4. **Aho-Corasick dictionary engine**: custom trie-based matcher for tenant-specific term lists at scale, with fuzzy fallback via Levenshtein.
5. **Middleware-level integration**: shield operates as a transparent HTTP middleware, not an SDK -- it intercepts JSON chat bodies, mutates them in-flight, and restores them on the way out.
6. **Reaction pipeline as policy point**: the domain model separates detection from decision from action, allowing per-tenant reaction configuration without changing detector code.
7. **Luhn validation on credit cards**: the `FinancialDetector` validates checksums, not just regex format, reducing noise from numeric false positives.
