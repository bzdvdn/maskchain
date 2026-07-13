# MaskChain Examples

Prod-like local dev stack for testing mask/unmask flows.

## Quick Start

```bash
# 1. Start infrastructure
docker compose up -d --build

# 2. Seed profile with dictionaries (500 users, 50 depts, 300 projects)
./seed-profile.sh

# 3. Open test-prompt.md for Postman test prompts
```

## Services

| Service  | URL                          | Auth                          |
|----------|------------------------------|-------------------------------|
| Gateway  | http://localhost:8080        | Bearer sk-test-default        |
| Admin    | http://localhost:8081        | Bearer sk-test-default        |
| Postgres | postgres://test:test@localhost:5432/maskchain | — |
| Valkey   | localhost:6379               | —                             |
| Grafana  | http://localhost:3000        | admin/admin (anonymous enabled) |
| Prometheus | http://localhost:9090      | —                             |

## Profile: `pii-protect`

Created by `seed-profile.sh`. Contains 3 dictionaries (exact match):

- **users** — 500 names (e.g. `James LastName42`, `Mary LastName99`)
- **departments** — 50 entries (e.g. `Engineering #1`, `Marketing #2`)
- **projects** — 300 entries (e.g. `Project-42`, `Project-15`)

## Test Flows

See `test-prompt.md` for detailed Postman requests.

### Flow A: Mask via regex detectors
1. `POST /api/v1/shield/mask` with raw CSV body → PII regex catches emails/phones/SSNs
2. `POST /api/v1/shield/unmask?mask_ids=<X-Mask-ID>` to restore

### Flow B: Shield scan with dictionary matching
1. `POST /v1/chat/completions` with `X-Shield-Profile-Slug: pii-protect`
2. Dictionary entries (names, departments, projects) are detected
3. Response shows `X-Shield-Status: suspicious`

## Configuration

Tenant `default` with API key `sk-test-default` is pre-configured in docker-compose.
