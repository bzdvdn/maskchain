# 03 — Dictionary masking (internal business terms)

**Goal:** mask things regex cannot know — employee names, project codenames,
departments, patient IDs — using per-tenant dictionaries.

Prereq: [quickstart](../../quickstart/) is running.

## Seed example dictionaries

The repo ships 500 users / 50 departments / 300 projects:

```bash
cd examples
./seed-tenant.sh http://localhost:9090 sk-test-default
```

This calls `PUT /api/v1/tenants/default` (PII rules) and
`PUT /api/v1/tenants/default/dictionaries` (entries).

## Manage dictionaries directly

```bash
curl -s -X PUT "http://localhost:9090/api/v1/tenants/default/dictionaries" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-test-default" \
  -d '{
    "dictionaries": [
      {"name": "projects", "match_mode": "exact",
       "entries": ["Project-42", "Project-15", "Project-300"]}
    ]
  }'
```

## Match modes

| `match_mode` | Matches |
|---|---|
| `exact` | the whole entry only |
| `contains` | entry as a substring |
| `regex` | entry as a regular expression |
| `fuzzy` | approximate match (typos / spacing) |

## Observe

```bash
curl -s -D - "http://localhost:8080/api/v1/chat/completions" \
  -H "Authorization: Bearer sk-test-default" \
  -H "Content-Type: application/json" \
  -d '{"model":"llama3.2","messages":[{"role":"user","content":"Summarize Project-42 owned by Engineering #1"}]}'
```

Expect `X-Shield-Status: suspicious`; the provider receives `[MASK.1]`-style
placeholders, and the answer is unmasked before it reaches the client. The
response header `X-Shield-Dict-Mask-ID` carries the mapping id for correlation.

## Notes

- Changing dictionaries invalidates the Valkey dictionary cache automatically.
- Prefer the dedicated `/dictionaries` endpoint: unlike `PUT /tenants/:slug` it
  does not overwrite the rest of the tenant config.
