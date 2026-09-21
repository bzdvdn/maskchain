# 07 — Semantic cache over masked data

**Goal:** serve repeated/near-duplicate prompts from cache instead of paying the
provider again — while the cache only ever sees **masked** text.

Prereq: [quickstart](../../quickstart/) is running.

## Enable

Add to `config-base.yaml` (or a config overlay) and restart the `maskchain`
service:

```yaml
data:
  cache:
    enabled: true
    ttl: 3600                 # seconds
    similarity_threshold: 0.90
    budget_guard_percent: 5   # stop writing near the hard limit
    max_entry_bytes: 1048576  # 1 MiB
    embedding:
      source: self-contained  # offline, deterministic (external = OpenAI-compatible)
      # external_url: https://api.openai.com/v1/embeddings
      # timeout_sec: 3
```

Restart:

```bash
docker compose -f ../quickstart/docker-compose.ollama.yml restart maskchain
```

## Observe

Send the same request twice (non-streaming):

```bash
BODY='{"model":"llama3.2","stream":false,"messages":[{"role":"user","content":"What is the capital of France?"}]}'
for i in 1 2; do
  curl -s -o /dev/null -w "request $i: %{http_code} (%{time_total}s)\n" \
    http://localhost:8080/api/v1/chat/completions \
    -H "Authorization: Bearer sk-test-default" \
    -H "Content-Type: application/json" -d "$BODY"
done
```

The second request is typically much faster and does not hit the provider.

## What is cached

- Key = `cache:<tenant>:<sha256(masked-embedding)>` — tenant-scoped.
- Only **masked** prompts are embedded; raw PII never enters the cache.
- Cache hits are saved to budget accounting; writes are suppressed when spend
  crosses `(100 - budget_guard_percent)%` of a hard limit.

## Notes

- Streaming requests bypass the cache.
- Entries expire after `ttl`; Valkey is the store (see the quickstart stack).
