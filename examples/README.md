# MaskChain Examples

Prod-like local dev stack for testing mask/unmask flows.

## Quick Start

```bash
# 1. Start infrastructure + gateway + admin
docker compose up -d --build

# 2. Or start combined binary (gateway + admin in one container)
docker compose --profile combined up -d --build postgres valkey all

# 3. Seed tenant dictionaries (500 users, 50 depts, 300 projects)
./seed-tenant.sh

# 4. Open test-prompt.md for Postman test prompts
```

## Services

| Service    | URL                          | Auth                          |
|------------|------------------------------|-------------------------------|
| Gateway    | http://localhost:8080        | Bearer sk-test-default        |
| Admin      | http://localhost:8082        | Bearer sk-test-default        |
| All        | http://localhost:8080        | Bearer sk-test-default        |
| All (admin)| http://localhost:9091        | Session cookie                |
| Postgres   | postgres://test:test@localhost:5432/maskchain | — |
| Valkey     | localhost:6379               | —                             |
| Grafana    | http://localhost:3000        | admin/admin (anonymous enabled) |
| Prometheus | http://localhost:9090        | —                             |

## Tenant: `default`

Pre-configured in `config.yaml` and synced to DB on startup.
Dictionaries are seeded by `seed-tenant.sh` into the tenant.
PII rules per-tenant via PIIConfig (email, phone, SSN — block by default).

Contains 3 dictionaries (exact match):

- **users** — 500 names (e.g. `James LastName42`, `Mary LastName99`)
- **departments** — 50 entries (e.g. `Engineering #1`, `Marketing #2`)
- **projects** — 300 entries (e.g. `Project-42`, `Project-15`)

PIIConfig rules:

| Rule   | Type | Action |
|--------|------|--------|
| email  | pii  | block  |
| phone  | pii  | block  |
| ssn    | pii  | block  |

Default action on engine error: `block`.

## Test Flows

See `test-prompt.md` for detailed Postman requests.

### Flow A: Mask/Unmask — PII Regex + Dictionary
1. `POST /api/v1/shield/mask` with raw CSV body → PII regex catches emails/phones/SSNs + dictionary values masked
2. `POST /api/v1/shield/unmask?mask_ids=<X-Mask-ID>` to restore
3. PII rules are per-tenant via PIIConfig (email/phone/SSN block)

### Flow B: Shield scan with dictionary matching
1. `POST /v1/chat/completions` with `Authorization: Bearer sk-test-default`
2. Tenant `default` is identified by API key, its PIIConfig rules + dictionaries are used
3. Response shows `X-Shield-Status: suspicious` (dictionary match) or `X-Shield-Status: blocked` (PII block)

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
    pii_config:
      enabled: true
      default_action: block
      rules:
        - label: email
          type: pii
          pattern: EMAIL
          action: block
        - label: phone
          type: pii
          pattern: PHONE
          action: block
        - label: ssn
          type: pii
          pattern: SSN
          action: block
```

| Field        | Type     | Description |
|--------------|----------|-------------|
| `auth_header` | string  | HTTP header for API key (default: `X-Mask-Authorization`) |
| `api_keys`    | []string | Valid API keys for this tenant |
| `pii_config`  | object  | PII rules per-tenant (enabled, default_action, rules) |

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
    ],
    "pii_config": {
      "enabled": true,
      "default_action": "block",
      "rules": [
        {"label": "ssn", "type": "pii", "pattern": "SSN", "action": "block"}
      ]
    }
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

Update tenant PIIConfig (requires admin API support):

```bash
curl -X PUT http://localhost:8082/api/v1/tenants/acme-corp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-test-default" \
  -d '{
    "name": "Acme Corp",
    "auth_header": "X-Acme-Key",
    "api_keys": ["sk-acme-001"],
    "pii_config": {
      "enabled": true,
      "default_action": "allow",
      "rules": [
        {"label": "email", "type": "pii", "pattern": "EMAIL", "action": "block"},
        {"label": "phone", "type": "pii", "pattern": "PHONE", "action": "allow"}
      ]
    }
  }'
```
