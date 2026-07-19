# Deployment Guide for MaskChain

## Overview

MaskChain supports two deployment modes (Docker Compose for single-node, Helm for Kubernetes) and two binary profiles (`gateway`, `admin`). Three Docker images are available: `maskchain/gateway`, `maskchain/admin`, `maskchain/combined`.

## Prerequisites

| Dependency | Minimum Version | Notes |
|-----------|----------------|-------|
| Docker | 24+ | For Compose mode |
| Kubernetes | 1.28+ | For Helm mode |
| PostgreSQL | 15+ | Or use bundled Bitnami subchart in Helm |
| Valkey | 7.2+ | Or use bundled Bitnami subchart in Helm |

## Option A: Docker Compose (Quick Start)

```bash
# Start the full production stack
docker compose -f deployments/docker-compose/docker-compose.yml --profile production up -d

# Verify health
curl http://localhost:8080/health
```

Environment variables with the `CONFIG_*` prefix override any YAML config value at runtime.

**Key environment variables:**

| Variable | Description |
|----------|-------------|
| `CONFIG_DATABASE_DSN` | PostgreSQL connection string |
| `CONFIG_VALKEY_ADDR` | Valkey address (`host:port`) |
| `CONFIG_ROUTING_PROVIDERS_0_API_KEYS_0` | OpenAI API key |
| `CONFIG_ROUTING_PROVIDERS_1_PROXY_URL` | Per-provider egress proxy (HTTP/HTTPS/SOCKS5) |
| `CONFIG_TENANTS_DEFAULT_API_KEYS_0` | Default tenant API key |

The stack includes three services — `gateway` (port 8080, 2 replicas), `admin` (port 8081, 1 replica), and `combined` (single binary with both profiles, port 8080 + 9090). Config is loaded from `config-base.yaml` and `config-runtime.yaml` mounted into `/etc/maskchain/conf.d/`.

## Option B: Kubernetes with Helm

```bash
# Add Bitnami Legacy repo for PostgreSQL and Valkey subcharts
helm repo add bitnamilegacy https://raw.githubusercontent.com/bitnamilegacy/charts/archive/refs/heads/main/bitnami
helm dependency update deployments/helm/maskchain/

# Install
helm install maskchain deployments/helm/maskchain/ \
  --set gateway.image.tag=latest \
  --set postgresql.auth.password=mysecret \
  --set valkey.auth.password=mysecret \
  --set apiKeys.OPENAI_API_KEY=sk-... \
  --set apiKeys.ADMIN_PASSWORD=admin123 \
  --set apiKeys.DEFAULT_API_KEY=dk-...
```

**Key values:**

| Value | Description |
|-------|-------------|
| `gateway.enabled` / `admin.enabled` / `all.enabled` | Component selection (mutually exclusive with `all.enabled`) |
| `<component>.image.tag` | Docker image tag per component |
| `<component>.replicaCount` | Replicas per component (e.g. `gateway.replicaCount: 2`) |
| `<component>.resources` | CPU/memory requests and limits per component |
| `<component>.ingress.enabled` | Expose component via Ingress |
| `<component>.gatewayAPI.enabled` | Use Gateway API (HTTPRoute) per component |
| `apiKeys` | All secrets injected as env vars (`OPENAI_API_KEY`, `ADMIN_PASSWORD`, `DEFAULT_API_KEY`, `POSTGRES_DSN`, `VALKEY_ADDR`, `VALKEY_PASSWORD`) |
| `postgresql.enabled` | Deploy bundled PostgreSQL (set `postgresql.external.enabled=true` to use external) |
| `valkey.enabled` | Deploy bundled Valkey (set `valkey.external.enabled=true` to use external) |
| `servicemonitor.enabled` | Prometheus ServiceMonitor integration |
| `pdb.enabled` | PodDisruptionBudget for HA |
| `networkPolicy.enabled` | Network policy isolation |

Config is split into `configBase` (infrastructure — db, valkey, egress, otel) and `configRuntime` (business logic — routing, shield, tenants). Both use `${VAR}` placeholders resolved from the `apiKeys` secret.

## Option C: Bare Binary

```bash
# Build gateway binary (CGO disabled for distroless compatibility)
CGO_ENABLED=0 go build -tags gateway -ldflags="-s -w" -o gateway ./src/cmd/gateway/

# Build admin binary
CGO_ENABLED=0 go build -tags admin -ldflags="-s -w" -o admin ./src/cmd/admin/

# Run
./gateway --config config.yaml
./admin --config config.yaml
```

## Production Best Practices

1. **Database**: Use managed PostgreSQL (RDS, CloudSQL) or run with streaming replication. Set `postgresql.external.enabled=true` in Helm.
2. **Valkey**: Use Valkey Sentinel or Cluster for HA. Set `valkey.architecture: replication` in the subchart.
3. **Secrets**: Use `apiKeys` in Helm or vault/env vars. Never commit secrets to git.
4. **Monitoring**: Prometheus + Grafana dashboards (`/metrics` endpoint). OTel tracing configurable via `configBase.otel`.
5. **Resource limits**: CPU 500m–2, memory 256Mi–1Gi depending on traffic profile.
6. **Readiness probes**: Gateway exposes `/health` (liveness), `/ready` (readiness), `/live` (startup).
7. **TLS**: Terminate at Ingress/Load Balancer, or configure within the Gateway server.
8. **Backup**: Regular PostgreSQL backups (`pg_dump` or WAL archiving). Export analytics data periodically.

## Security Checklist

- [ ] Change default admin password
- [ ] Enable TLS for gateway and admin
- [ ] Configure network policies (Helm: `networkPolicy.enabled=true`)
- [ ] Rotate API keys periodically
- [ ] Enable audit logging (`log.level: debug` if needed)
- [ ] Run `make security-check` before deployment
- [ ] Use non-root user (Docker images use distroless nonroot by default)

## Scaling

- **Horizontal**: Stateless gateway — scale replicas behind a load balancer. In Helm set `<component>.replicaCount` (e.g. `gateway.replicaCount: 3`).
- **Database connection pool**: Tune `max_conns` and `min_conns` in `configBase.database`.
- **Rate limiting**: Adjust `default_rate_per_window` and `default_window_sec` in `configBase.ratelimit`.
- **Dictionary cache**: Tune `lru_size` and `valkey_ttl_sec` in `configBase.dictionary_cache`.
- **Egress**: Configure `max_idle_conns`, `max_idle_conns_per_host`, `max_retries`, and circuit breaker settings under `configBase.egress`. Per-provider `proxy_url` (HTTP/HTTPS/SOCKS5) set in provider config for corporate proxy environments.

## Troubleshooting

See `deployments/runbook.md` for detailed procedures.

| Issue | Quick Pointer |
|-------|---------------|
| Connection pool exhaustion | Check `maskchain_pgx_pool_*` metrics; increase `max_conns` |
| TLS handshake failures | Verify certs, proxy env vars, CA bundle |
| Provider timeouts | Adjust `egress.max_retries`, `base_backoff`, provider timeout |
| Startup crashes | Check logs for `FATAL`/`panic`, verify DB/Valkey connectivity |
