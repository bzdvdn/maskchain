# Data Model — semantic-cache-masked

## Status: change

The semantic cache introduces a new data structure in Valkey. The PostgreSQL schema is unchanged; no migrations.

## Entities

- `domain/cache`:
  - `CacheKey` — composite key: `cache:{tenant_slug}:{sha256(masked-embedding)}`. Tenant is part of the key → per-tenant isolation (AC-003). The embedding is computed from the masked prompt, never from raw data.
  - `CacheEntry` — value: JSON object of the masked response (wire form containing `[MASK_...]` placeholders), `created_at`, `expires_at`. Holds only masked forms (AC-005).
- Storage: Valkey (`src/internal/adapters/repository/cache/valkey.go`), TTL per tenant from `data.cache.*`.

## New config fields

`DataConfig` in `src/internal/infra/config/config.go`:

```yaml
data:
  cache:
    enabled: false            # default off, feature flag
    ttl: 5m                   # per-tenant default TTL
    similarity_threshold: 0.90  # default cosine threshold
    budget_guard_percent: 5     # write-blocked when spent >= 95% hard limit
    max_entry_bytes: 1048576    # skip write for responses > 1MB
    embedding:
      source: external          # external | self-contained
      external_url: ""           # OpenAI-compatible endpoint
      external_api_key: ""
      timeout: 2s
```

## System effect

- Unchanged structures: `storage.go` (mask), PG schema, existing use cases/ports (budget/virtual-key/analytics) are not modified.
- New records only in Valkey under the `cache:` prefix; other keys untouched.

## Considered and rejected

- Storage in PostgreSQL: semantic search and high TTL rotation do not fit the data profile (hot/expiring reads); Valkey is already used for mask/rate-limit — a consistent choice.
- Exact key from masked-text hash: placeholders are non-deterministic (`MASK_<id>.<n>`, UUID-based), string equality gives no stable key → the key is built from the embedding of the masked input.