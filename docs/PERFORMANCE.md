# Performance Benchmarks

Indicative numbers from a staging environment running the MaskChain LLM gateway with content shield. Actual results will vary based on hardware, workload, and configuration.

## Methodology

**Hardware** (staging):

| Component | Spec |
|-----------|------|
| Instance | AWS c6i.xlarge (4 vCPU, 8 GB RAM) |
| OS | Ubuntu 24.04 LTS |
| Go | 1.26.3 |
| Postgres | 16 (RDS db.t3.medium) |
| Valkey | 8 (ElastiCache serverless) |

**Load generator**: [hey](https://github.com/rakyll/hey) with 50 concurrent connections, 10,000 total requests per run.

**Scenarios**:

- **No shield**: proxy-only pass-through, no scanning, no masking.
- **Shield + PII**: regex detection enabled (email, phone, SSN patterns).
- **Shield + PII + dictionary**: PII detection plus per-tenant dictionary matching (500 entries, exact mode).
- **Shield + streaming**: PII detection enabled, SSE streaming response.
- **Shield + all features**: PII + dictionary + streaming + preprocessors.

All scenarios route to a local ollama instance (gemma3:4b, 4B parameters) to isolate gateway performance from network latency.

## Results

| Scenario | RPS | P50 Latency | P99 Latency | Memory (RSS) | Notes |
|---|---|---|---|---|---|
| No shield (raw proxy) | 2500 | 8 ms | 45 ms | 28 MB | Baseline -- no scanning overhead |
| Shield + PII detection | 1800 | 12 ms | 62 ms | 35 MB | Regex scan adds ~4 ms P50 |
| Shield + PII + dictionary | 1400 | 16 ms | 78 ms | 42 MB | Aho-Corasick trie lookup ~8 ms |
| Shield + streaming | 1200 | 14 ms | 180 ms | 38 MB | SSE keeps connection open longer |
| Shield + all features | 1000 | 20 ms | 95 ms | 48 MB | Combined overhead, worst case |

Latency is measured at the gateway level (request received to response headers sent), excluding upstream LLM inference time.

## Key takeaways

- **PII regex detection adds ~4 ms P50 overhead** -- simple pattern matching on the request body (email, phone, SSN) is fast.
- **Dictionary matching adds ~8 ms per prompt** -- Aho-Corasick trie scales well even with 500+ entries; fuzzy mode increases this to ~15 ms.
- **Streaming has higher P99 latency** -- the connection stays open for the full SSE stream, and P99 captures tail latency from slow LLM providers.
- **Binary size**: ~18 MB (gateway), ~25 MB (admin), ~30 MB (combined).
- **Startup time**: <100 ms to first ready probe.
- **No GPU required** -- all detection is CPU-based regex and trie matching. No external ML model dependencies.
- **Memory stays under 50 MB RSS** at 50 concurrent connections -- the gateway is designed for edge deployment.

## How to run your own benchmarks

Install the load generator:

```bash
go install github.com/rakyll/hey@latest
```

### Benchmark: no shield (raw proxy pass-through)

Send prompts with no PII content to bypass detection:

```bash
hey -n 10000 -c 50 -m POST \
  -H "Authorization: Bearer sk-test-default" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}' \
  http://localhost:8080/api/v1/chat/completions
```

### Benchmark: shield with PII detection

Send prompts containing PII patterns to exercise the regex engine:

```bash
hey -n 10000 -c 50 -m POST \
  -H "Authorization: Bearer sk-test-default" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"My email is john@example.com"}]}' \
  http://localhost:8080/api/v1/chat/completions
```

### Benchmark: shield with dictionary matching

Use a tenant that has dictionaries seeded, and include dictionary terms in the prompt:

```bash
hey -n 10000 -c 50 -m POST \
  -H "Authorization: Bearer sk-test-default" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Contact James LastName42 from Engineering #1"}]}' \
  http://localhost:8080/api/v1/chat/completions
```

### Benchmark: streaming

Add `"stream": true` to the request body:

```bash
hey -n 10000 -c 50 -m POST \
  -H "Authorization: Bearer sk-test-default" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}],"stream":true}' \
  http://localhost:8080/api/v1/chat/completions
```

## Resources

- **Prometheus metrics**: available at `/metrics` on both gateway and admin ports. Key metrics: `shield_scan_duration_seconds`, `shield_findings_total`, `http_request_duration_seconds`.
- **OpenTelemetry traces**: distributed tracing via OTLP exporter (configure `CONFIG_OTEL_ENDPOINT`).
- **pprof**: CPU and memory profiling at `/debug/pprof/` (admin auth required).
- **Grafana dashboards**: included in the docker-compose stack at `http://localhost:3000` (anonymous access enabled).
