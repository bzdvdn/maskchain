# Production Runbook

## Startup Sequence

1. Ensure PostgreSQL and Valkey are reachable (check `CONFIG_DATABASE_DSN` and `CONFIG_VALKEY_ADDR`)
2. Run database migrations: `make build && ./bin/gateway` (migrations run automatically on startup)
3. Verify gateway starts without errors: `docker compose --profile production up -d`
4. Check logs: `docker compose --profile production logs -f gateway`

## Health Check Endpoints

| Endpoint | Expected | Purpose |
|----------|----------|---------|
| `GET /health` | `{"status":"ok"}` | Liveness probe |
| `GET /ready` | `{"status":"ok"}` | Readiness probe |
| `GET /live` | `{"status":"alive"}` | Startup probe |
| `GET /metrics` | Prometheus text | Scrape target |
| `GET /debug/pprof/` | HTML page (auth req.) | Profiling (requires `debug.enabled: true`) |

## Debug Procedure

### Connection Pool Exhaustion

**Symptoms:** `pool exhausted`, `timeout waiting for connection`, increased latency.

**Checks:**
- `/metrics` → `maskchain_pgx_pool_acquire_count`, `maskchain_pgx_pool_idle_conns`, `maskchain_pgx_pool_in_use_conns`
- Logs at startup show current pool configuration
- Check active connections: `SELECT count(*) FROM pg_stat_activity WHERE datname = 'maskchain';`

**Mitigation:**
- Increase `CONFIG_DATABASE_MAX_CONNS` / `CONFIG_EGRESS_MAX_IDLE_CONNS_PER_HOST`
- Reduce `CONFIG_DATABASE_MIN_CONNS` if idle connections are too high
- Restart gateway with updated config

### TLS Handshake Failure

**Symptoms:** `tls: handshake failure`, `certificate signed by unknown authority` in logs.

**Checks:**
- Verify TLS certificates are valid and not expired
- Check proxy environment variables (`HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`)
- Review `openssl s_client -connect <host>:443` for chain issues

**Mitigation:**
- Update CA certificate bundle or configure custom CA
- Add proxy exception in `NO_PROXY` if proxy interferes with TLS
- Restart gateway after certificate update

### Provider Timeout

**Symptoms:** `context deadline exceeded`, `upstream timeout`, `504 Gateway Timeout`.

**Checks:**
- Check provider health endpoint if available
- Review `CONFIG_EGRESS_MAX_RETRIES` and `CONFIG_EGRESS_BASE_BACKOFF`
- Monitor network latency between gateway and provider

**Mitigation:**
- Increase provider timeout in config
- Reduce `CONFIG_EGRESS_MAX_IDLE_CONNS` if connections are stale
- Enable retry with `CONFIG_EGRESS_RETRY_ON_5XX: "true"`

### Startup Crash

**Symptoms:** Gateway exits immediately after start, `FATAL` in logs.

**Checks:**
- `docker compose --profile production logs gateway` — look for `FATAL` or `panic`
- Verify config file syntax: `CONFIG_*` environment variables
- Check PostgreSQL and Valkey connectivity from gateway container
- Review migration logs — failed migrations block startup

**Mitigation:**
- Fix config syntax or connection strings
- Ensure dependent services are healthy
- Rollback to last known good config

## Recovery Steps

1. **Rollback:** Deploy previous known-good version
2. **Config rollback:** Restore previous config file or env vars
3. **Database:** Check migration status — roll back partial migrations if needed
4. **Logs:** Collect logs for postmortem: `docker compose --profile production logs --tail=1000 gateway > crash.log`
5. **Profile:** If crash is performance-related, enable debug mode and collect pprof:
   ```
   curl -H "X-Admin-Token: <token>" http://localhost:8080/debug/pprof/heap > heap.out
   curl -H "X-Admin-Token: <token>" http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.out
   ```
