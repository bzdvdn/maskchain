# 09 — Observability: metrics, tracing, health, admin status

**Goal:** see what the gateway is doing and wire it into your monitoring stack.

Prereq: [quickstart](../../quickstart/) is running.

## Prometheus metrics

```bash
curl -s http://localhost:8080/metrics | grep '^maskchain_' | head
```

Key series: HTTP request rate/latency, shield scan stats, egress pool stats,
cache hit rate. The admin port exposes its own `/metrics`:

```bash
curl -s http://localhost:9090/metrics | head
```

## Health probes

| Endpoint | Meaning |
|---|---|
| `GET /health` | liveness — process is up |
| `GET /ready` | readiness — critical deps (DB) reachable, 503 otherwise |
| `GET /live` | startup probe |

```bash
curl -s http://localhost:8080/ready | jq .
```

## Admin status (single control-plane view)

```bash
ADMIN=http://localhost:9090
TOKEN=$(curl -s -X POST "$ADMIN/api/v1/admin/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"test"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')

curl -s "$ADMIN/api/v1/admin/status" -H "Authorization: Bearer $TOKEN" | jq .
```

Returns version, uptime, at-rest key state, aggregated dependency health, and a
live-vs-file config diff. The UI renders this on **System → Settings**.

## OpenTelemetry

```yaml
otel:
  endpoint: "otel-collector:4317"   # OTLP gRPC
  service_name: maskchain-gateway
  environment: production
  sampling_ratio: 0.1
```

Logs are structured (`slog`) and enriched with the OTel `trace_id`/`span_id`.
The quickstart compose ships Prometheus + Grafana in the older
[`examples/docker-compose.all.yml`](../../docker-compose.all.yml) if you want a
dashboard stack.

## Provider health

```bash
curl -s "$ADMIN/api/v1/routing/providers" -H "Authorization: Bearer $TOKEN" \
  | jq '.[] | {name, status, latency_ms}'
```
