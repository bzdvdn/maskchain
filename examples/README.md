# MaskChain Examples

Prod-like local dev stack for testing mask/unmask flows.

## Quick Start

```bash
# 1. Start infrastructure
docker compose up -d --build

# 2. Seed tenant dictionaries (500 users, 50 depts, 300 projects)
./seed-tenant.sh

# 3. Open test-prompt.md for Postman test prompts
```

## Services

| Service  | URL                          | Auth                          |
|----------|------------------------------|-------------------------------|
| Gateway  | http://localhost:8080        | Bearer sk-test-default        |
| Admin    | http://localhost:8082        | Bearer sk-test-default        |
| Postgres | postgres://test:test@localhost:5432/maskchain | — |
| Valkey   | localhost:6379               | —                             |
| Grafana  | http://localhost:3000        | admin/admin (anonymous enabled) |
| Prometheus | http://localhost:9090      | —                             |

## Tenant: `default`

Pre-configured in `config.yaml` and synced to DB on startup.
Dictionaries are seeded by `seed-tenant.sh` into the tenant. Contains 3 dictionaries (exact match):

- **users** — 500 names (e.g. `James LastName42`, `Mary LastName99`)
- **departments** — 50 entries (e.g. `Engineering #1`, `Marketing #2`)
- **projects** — 300 entries (e.g. `Project-42`, `Project-15`)

## Test Flows

See `test-prompt.md` for detailed Postman requests.

### Flow A: Mask via regex detectors
1. `POST /api/v1/shield/mask` with raw CSV body → PII regex catches emails/phones/SSNs
2. `POST /api/v1/shield/unmask?mask_ids=<X-Mask-ID>` to restore

### Flow B: Shield scan with dictionary matching
1. `POST /v1/chat/completions` with `Authorization: Bearer sk-test-default`
2. Tenant `default` is identified by API key, its dictionaries are used for matching
3. Response shows `X-Shield-Status: suspicious`

## Tenants

### Via YAML

Tenants are defined in `config.yaml` (mounted into containers). On startup they are synced to DB:

```yaml
tenants:
  default:
    name: "Default Tenant"
    auth_header: "Authorization"
    api_keys:
      - "sk-test-default"
```

| Field        | Type     | Description |
|--------------|----------|-------------|
| `auth_header` | string  | HTTP header for API key (default: `X-Mask-Authorization`) |
| `api_keys`    | []string | Valid API keys for this tenant |

### Via API

Create a tenant at runtime through the admin API:

```bash
curl -X POST http://localhost:8082/api/v1/tenants \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-test-default" \
  -d '{
    "slug": "acme-corp",
    "name": "Acme Corp",
    "auth_header": "X-Acme-Key",
    "api_keys": ["sk-acme-001"],
    "dictionaries": [
      {"name": "employees", "entries": ["Alice","Bob"], "match_mode": "exact"}
    ]
  }'
```

Update dictionaries separately:

```bash
curl -X PUT http://localhost:8082/api/v1/tenants/acme-corp/dictionaries \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-test-default" \
  -d '{
    "dictionaries": [
      {"name": "employees", "entries": ["Alice","Bob","Charlie"], "match_mode": "exact"}
    ]
  }'
```
