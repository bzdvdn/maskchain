# MaskChain Examples

Grouped by what you want to do. Start with the quickstart, then jump to a recipe.

## Pick a path

| Path | Use it for | Time |
|---|---|---|
| [**quickstart/**](quickstart/) | One command to a working masked LLM request (local Ollama, or OpenAI) | ~5 min |
| [**cookbook/**](cookbook/) | Grouped recipes: masking, PII policies, dictionaries, routing, keys/budgets, compliance, cache, conversations, observability | per recipe |
| [**clients/**](clients/) | Python / Node / Go callers | 1 min |
| [**postman/**](postman/) | Importable collection (mask, chat, admin) | 1 min |
| [**compliance/presets/**](compliance/presets/) | HIPAA / PCI DSS / GDPR / Legal / SOC 2 pack YAMLs | — |

## Quickstart

```bash
cd examples/quickstart
./quickstart.sh              # local Ollama, no external API keys
./quickstart.sh openai       # OpenAI; needs OPENAI_KEY in .env
```

Then:

```bash
curl -s http://localhost:8080/api/v1/chat/completions \
  -H "Authorization: Bearer sk-test-default" \
  -H "Content-Type: application/json" \
  -d '{"model":"llama3.2","messages":[{"role":"user","content":"email alice@example.com"}]}'
```

## Cookbook

| # | Recipe | Topic |
|---|---|---|
| 01 | [mask-unmask](cookbook/01-mask-unmask/) | Reversible placeholders, `clean`/`id`/`redact` formats |
| 02 | [pii-policies](cookbook/02-pii-policies/) | Per-tenant mask vs block |
| 03 | [dictionary-masking](cookbook/03-dictionary-masking/) | Internal names, codenames, IDs |
| 04 | [routing-fallback](cookbook/04-routing-fallback/) | Providers, fallback order, circuit breaker |
| 05 | [keys-and-budgets](cookbook/05-keys-and-budgets/) | Scoped virtual keys + spend enforcement |
| 06 | [compliance-packs](cookbook/06-compliance-packs/) | Apply/audit HIPAA/PCI/GDPR presets |
| 07 | [semantic-cache](cookbook/07-semantic-cache/) | Cache over masked data |
| 08 | [conversations](cookbook/08-conversations/) | Encrypted request/response logging |
| 09 | [observability](cookbook/09-observability/) | Metrics, health, admin status, OTel |

## Reference

### Tenant config (YAML)

Tenants are defined in config and synced to the DB on startup:

```yaml
tenants:
  default:
    name: "Default Tenant"
    auth_header: "Authorization"
    api_keys: ["sk-test-default"]   # bootstrapped into hashed virtual keys
    pii_config:
      enabled: true
      default_action: mask
      rules:
        - {label: email, type: regex, pattern: EMAIL, action: mask}
        - {label: ssn,   type: regex, pattern: SSN,   action: block}
```

`api_keys` are idempotently bootstrapped into SHA-256-hashed virtual keys;
rotate/add keys at runtime via `/api/v1/keys` instead of editing YAML.

### Legacy stacks (kept for reference)

- [`docker-compose.split.yml`](docker-compose.split.yml) — separate gateway + admin + Prometheus + Grafana.
- [`docker-compose.all.yml`](docker-compose.all.yml) — combined binary + monitoring.
- [`config.yaml`](config.yaml) — full config reference (not mounted by the stacks).
- [`seed-tenant.sh`](seed-tenant.sh) — seeds 500 users / 50 departments / 300 projects.
- [`test-prompt.md`](test-prompt.md) — long-form Postman walkthrough.
- [`ollama/README.md`](ollama/README.md) — historical Ollama notes.

The scenario-based structure above is the supported path. The legacy files are
retained for the split/combined monitoring stack, but the quickstart is the
recommended entry point.
