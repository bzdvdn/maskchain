# Quickstart

One command to a running MaskChain gateway with a masked, reversible LLM request.

## Option A — Local Ollama (no external API keys)

```bash
cd examples/quickstart
./quickstart.sh            # or: make quickstart
```

This builds the combined binary, starts Postgres + Valkey + Ollama, pulls
`llama3.2`, and bootstraps the `default` tenant (key `sk-test-default`).

## Option B — OpenAI

```bash
cd examples/quickstart
cp .env.example .env
# edit .env: set OPENAI_KEY and MASKCHAIN_KEYS_KEY (openssl rand -base64 32)
docker compose -f docker-compose.openai.yml up -d --build
```

## Verify

```bash
curl -s http://localhost:8080/health

# Masked chat: PII is replaced before it reaches the model, restored after.
curl -s http://localhost:8080/api/v1/chat/completions \
  -H "Authorization: Bearer sk-test-default" \
  -H "Content-Type: application/json" \
  -d '{"model":"llama3.2","messages":[{"role":"user","content":"My email is alice@example.com, call me at +1-555-0100"}]}'
```

Response headers include `X-Shield-Status`; the body contains the original
values because MaskChain unmasks the model's answer on the way back.

## Services

| Service | URL | Credentials |
|---|---|---|
| Gateway (data plane) | http://localhost:8080 | `Authorization: Bearer sk-test-default` |
| Admin + UI | http://localhost:9090 | `admin` / `test` |
| Swagger | http://localhost:9090/api/v1/docs | — |
| Postgres | `postgres://test:test@localhost:5433/maskchain` (only if exposed) | — |

## Files

| File | Purpose |
|---|---|
| `docker-compose.ollama.yml` | Self-contained stack with a local Ollama provider |
| `docker-compose.openai.yml` | Stack pointed at OpenAI |
| `config-base.yaml` | Infrastructure sections (server, DB, Valkey, admin) |
| `config-runtime.ollama.yaml` | Ollama provider + `llama3.2` route + tenant |
| `config-runtime.openai.yaml` | OpenAI provider + `gpt-4o-mini` route + tenant |
| `.env.example` | Required secrets template |
| `quickstart.sh` | Generates `.env`, starts the stack, waits for health |

## Troubleshooting

- **`MASKCHAIN_KEYS_KEY` must be set** — MaskChain fails closed without it when
  DB-backed routing/tenancy is enabled (this is the at-rest encryption key).
  `quickstart.sh` generates one; for manual runs set it in `.env`.
- **Ollama model pull is slow on first run** — the gateway waits until the pull
  finishes. Override the model with `OLLAMA_MODEL=<name> ./quickstart.sh`.
- **Port already in use** — stop local Postgres/Valkey/Ollama, or edit the port
  mappings in the compose file.

## Next

- [Cookbook](../cookbook/) — grouped recipes (masking, routing, budgets, compliance, cache).
- [Clients](../clients/) — Python / Node / Go callers.
- [Postman](../postman/) — importable collection.
