# 02 — Per-tenant PII policies (mask vs block)

**Goal:** decide what happens when PII is detected — mask it (reversible) or block
the request (403).

Prereq: [quickstart](../../quickstart/) is running.

## Where policies live

`pii_config` on the tenant. Each rule maps a detector `pattern` (a named regex:
`EMAIL`, `PHONE`, `SSN`, …) to an `action`.

```yaml
tenants:
  default:
    pii_config:
      enabled: true
      default_action: mask      # fallback when a rule has no action
      rules:
        - {label: email, type: regex, pattern: EMAIL, action: mask}
        - {label: phone, type: regex, pattern: PHONE, action: mask}
        - {label: ssn,   type: regex, pattern: SSN,   action: block}
```

## Change a policy at runtime

Tenant endpoints accept either an admin session or a tenant key.

```bash
# Block emails instead of masking them.
curl -s -X PUT "http://localhost:9090/api/v1/tenants/default" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-test-default" \
  -d '{
    "name": "Default Tenant",
    "auth_header": "Authorization",
    "pii_config": {
      "enabled": true,
      "default_action": "mask",
      "rules": [
        {"label": "email", "type": "regex", "pattern": "EMAIL", "action": "block"},
        {"label": "phone", "type": "regex", "pattern": "PHONE", "action": "mask"},
        {"label": "ssn",   "type": "regex", "pattern": "SSN",   "action": "mask"}
      ]
    }
  }'
```

## Observe

```bash
# action=mask  -> 200, the model never sees the address
# action=block -> 403 and X-Shield-Status: blocked
curl -s -D - "http://localhost:8080/api/v1/chat/completions" \
  -H "Authorization: Bearer sk-test-default" \
  -H "Content-Type: application/json" \
  -d '{"model":"llama3.2","messages":[{"role":"user","content":"email bob@example.com"}]}'
```

## Notes

- Rules are applied before the provider call; `block` short-circuits the request.
- `default_action` is used if the engine errors or a detector has no explicit rule.
- The header `X-Shield-Status` reports `clean | suspicious | blocked`.
