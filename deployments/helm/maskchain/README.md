# MaskChain Helm Chart

Deploy [MaskChain](https://github.com/bzdvdn/maskchain) — AI Gateway with Content Shield — to Kubernetes.

## Prerequisites

- Kubernetes 1.28+
- Helm 3+
- Bitnami Helm repository (for subchart dependencies)

```bash
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update
```

## Quick Start

```bash
cd deployments/helm/maskchain
helm dependency update
helm install maskchain .
```

This deploys:
- MaskChain gateway (default component)
- PostgreSQL (Bitnami subchart, `docker.io/bitnamilegacy` images)
- Valkey (Bitnami subchart, `docker.io/bitnamilegacy` images)

Check status:

```bash
kubectl get pods -l app.kubernetes.io/instance=maskchain
kubectl port-forward svc/maskchain-maskchain-gateway 8080:8080
```

---

## Architecture

The chart renders **separate** Kubernetes resources per component, organised into subdirectories:

```
templates/
├── configmap.yaml          # Shared config
├── secret.yaml             # Shared API keys + auto-generated DB secrets
├── deployments/            # Component Deployments
│   ├── gateway.yaml        (gateway.enabled)
│   ├── admin.yaml          (admin.enabled)
│   └── all.yaml            (all.enabled)
├── services/               # Component Services
│   ├── gateway.yaml
│   ├── admin.yaml
│   └── all.yaml
├── ingresses/              # Component Ingresses
│   ├── gateway.yaml
│   ├── admin.yaml
│   └── all.yaml
├── httproute.yaml          # Gateway API HTTPRoute
├── servicemonitor.yaml     # Prometheus ServiceMonitor
├── pdb.yaml                # PodDisruptionBudget
├── networkpolicy.yaml      # NetworkPolicy
└── tests/test-connection.yaml
```

Each component is self-contained: a Deployment with its own selector label (`app.kubernetes.io/component: <name>`), paired Service, and optional Ingress.

---

## Component Modes

Three mutually exclusive component flags control what runs:

| Flag | Default | Description |
|------|---------|-------------|
| `gateway.enabled` | `true` | Separate gateway Deployment (port `config.server.port`) |
| `admin.enabled` | `false` | Separate admin Deployment (port `config.server.admin_port`) |
| `all.enabled` | `false` | Combined Deployment with both ports (`all` conflicts with `gateway` and `admin`) |

### Examples

**Gateway only** (default):
```bash
helm install maskchain .
```

**Admin only**:
```bash
helm install maskchain . \
  --set gateway.enabled=false \
  --set admin.enabled=true
```

**Combined binary** (one pod, two ports):
```bash
helm install maskchain . \
  --set all.enabled=true \
  --set gateway.enabled=false \
  --set admin.enabled=false
```

**Gateway + Admin** (separate Deployments):
```bash
helm install maskchain . \
  --set gateway.enabled=true \
  --set admin.enabled=true
```

---

## Configuration Reference

All chart settings are documented in [values.yaml](values.yaml). Key sections:

### Global

```yaml
nameOverride: ""
fullnameOverride: ""
replicaCount: 1
```

### Component Images

```yaml
gateway:
  enabled: true
  image:
    repository: maskchain/gateway
    tag: latest
    pullPolicy: IfNotPresent

admin:
  enabled: false
  image:
    repository: maskchain/admin
    tag: latest
    pullPolicy: IfNotPresent

all:
  enabled: false
  image:
    repository: maskchain/all
    tag: latest
    pullPolicy: IfNotPresent
```

### Application Config

The `config` section maps directly to MaskChain's `config.yaml`. Every field from `src/internal/infra/config/config.go` is supported:

```yaml
config:
  log:
    level: info

  server:
    port: 8080
    admin_port: 9090
    shutdown_timeout: 30
    tenant_reload_interval: 15s
    health_check:
      critical_deps:
        - database

  database:
    dsn: ${POSTGRES_DSN}
    max_conns: 25
    min_conns: 5
    max_conn_lifetime: 30m

  valkey:
    addr: ${VALKEY_ADDR}
    password: ${VALKEY_PASSWORD}
    ttl_sec: 3600

  mask:
    cache_ttl_sec: 3600

  shield:
    action_on_suspicious: mask
    tenant_model_mapping:
      default:
        "gpt-4o": strict

  routing:
    providers:
      - name: openai
        api_type: openai
        base_url: https://api.openai.com
        api_keys:
          - ${OPENAI_API_KEY}
        timeout: 120s
        priority: 1
    rules:
      - tenant: default
        routes:
          - model: gpt-4o
            providers: [openai]
          - model: gpt-4o-mini
            providers: [openai]

  egress:
    max_idle_conns: 25
    max_idle_conns_per_host: 4
    idle_timeout: 60s
    max_retries: 3
    base_backoff: 200ms
    retry_on_5xx: true
    disable_keep_alives: false
    circuit_breaker:
      max_failures: 5
      cooldown: 30s

  session:
    default_ttl: 1h
    max_ttl: 24h
    cache_ttl: 5m
    cleanup_interval: 15m
    cleanup_enabled: true

  admin:
    username: admin
    password: ${ADMIN_PASSWORD}
    session_ttl: 30m
    dashboard_poll_interval: 5s

  otel:
    endpoint: ""
    service_name: maskchain
    environment: production
    sampling_ratio: 0.1

  ratelimit:
    default_rate_per_window: 100
    default_window_sec: 60

  debug:
    enabled: false
    admin_token: ""

  analytics:
    batch_interval: 5s
    retention_days: 90

  dictionary_cache:
    valkey_ttl_sec: 300
    lru_size: 10000
    warm_on_startup: true
    warm_concurrency: 5

  tenants:
    default:
      name: Default Tenant
      auth_header: Authorization
      api_keys:
        - ${DEFAULT_API_KEY}
```

Sensitive values (`${POSTGRES_DSN}`, `${VALKEY_PASSWORD}`, `${OPENAI_API_KEY}`, `${ADMIN_PASSWORD}`, `${DEFAULT_API_KEY}`) are resolved at runtime from environment variables injected via the `apiKeys` Secret. See [API Keys & Secrets](#api-keys--secrets).

### Resources

```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 256Mi

service:
  type: ClusterIP
```

---

## Database Modes

PostgreSQL and Valkey can run as internal subcharts or connect to external instances.

### Internal (default)

Bitnami subcharts with `docker.io/bitnamilegacy` images:

```yaml
postgresql:
  enabled: true
  image:
    registry: docker.io/bitnamilegacy
  auth:
    username: maskchain
    password: ""            # auto-generated if empty
    database: maskchain
  primary:
    persistence:
      size: 10Gi

valkey:
  enabled: true
  image:
    registry: docker.io/bitnamilegacy
  auth:
    password: ""            # auto-generated if empty
  architecture: standalone
  master:
    persistence:
      size: 5Gi
```

### External

Connect to existing PostgreSQL/Valkey:

```yaml
postgresql:
  enabled: false            # disable subchart
  external:
    enabled: true
    dsn: "postgres://user:pass@host:5432/maskchain?sslmode=disable"

valkey:
  enabled: false
  external:
    enabled: true
    addr: "host:6379"
```

When using external mode, you must provide connection details via `apiKeys` (see below).

### Auto-generated Secrets

When **internal** subcharts are used, the chart automatically populates these secret keys:

| Secret Key | Source |
|------------|--------|
| `POSTGRES_DSN` | `postgres://<username>:<password>@<release>-postgresql:5432/<database>?sslmode=disable` |
| `VALKEY_ADDR` | `<release>-valkey:6379` |
| `VALKEY_PASSWORD` | `valkey.auth.password` |

When **external** mode is used:

| Secret Key | Source |
|------------|--------|
| `POSTGRES_DSN` | `postgresql.external.dsn` |
| `VALKEY_ADDR` | `valkey.external.addr` |
| `VALKEY_PASSWORD` | `valkey.auth.password` |

User-supplied values in `apiKeys` always **override** auto-generated values.

---

## API Keys & Secrets

Sensitive values use `${VAR}` placeholders in `config.*` and are resolved at runtime from environment variables. The `apiKeys` section creates a Kubernetes Secret whose keys become environment variables.

```yaml
apiKeys:
  OPENAI_API_KEY: "sk-..."
  ADMIN_PASSWORD: "admin-secret"
  DEFAULT_API_KEY: "dk-..."
  # Only needed for external mode (auto-generated for internal):
  POSTGRES_DSN: "postgres://user:pass@host:5432/maskchain?sslmode=disable"
  VALKEY_ADDR: "host:6379"
  VALKEY_PASSWORD: ""
```

**Important:** Keys are base64-encoded in the Secret. Helm renders them as-is from `values.yaml` — for production, use `--set` or a separate encrypted secrets file.

**Security note:** The ConfigMap contains `${VAR}` placeholders, **not** plaintext secrets. Actual values are injected via `envFrom.secretRef` in the Deployment.

---

## Optional Resources

### Ingress

```yaml
ingress:
  enabled: false
  className: ""
  annotations: {}
  tls: []
  gateway:
    host: api.maskchain.local
    path: /
  admin:
    host: admin.maskchain.local
    path: /
```

- **gateway mode** (`gateway.enabled=true`, `all.enabled=false`): Ingress routes to `-gateway` Service
- **admin mode** (`admin.enabled=true`): Ingress routes to `-admin` Service
- **all mode** (`all.enabled=true`): Two Ingresses (gateway + admin) route to `-all` Service on respective ports

### Gateway API (HTTPRoute)

```yaml
gatewayAPI:
  enabled: false
  gatewayName: maskchain-gateway
  gatewayNamespace: ""
  hostname: api.maskchain.local
  adminHostname: admin.maskchain.local
  tls:
    enabled: false
    certificateRef:
      name: ""
```

Requires a Gateway API controller in the cluster.

### ServiceMonitor (Prometheus Operator)

```yaml
servicemonitor:
  enabled: false
  interval: 15s
  scrapeTimeout: 10s
  labels: {}
```

Creates a `ServiceMonitor` for prometheus-operator. Gateway metrics on port `http` (8080), admin metrics on port `admin-http` (9090). Each component uses its own Prometheus registry.

### PodDisruptionBudget

```yaml
pdb:
  enabled: false
  minAvailable: 1
```

Creates a PDB for each enabled component with `minAvailable: 1`.

### NetworkPolicy

```yaml
networkPolicy:
  enabled: false
  ingressControllerNamespace: ingress-nginx
  externalCIDRs: []
```

Restricts ingress traffic:
- Port 8080 (gateway) from ingress controller namespace
- Port 9090 (admin) from ingress controller namespace
- DNS (port 53) from cluster
- Optionally from `externalCIDRs`

---

## Installing with Custom Values

Use a custom values file:

```bash
helm install maskchain . -f my-values.yaml
```

Override individual values:

```bash
helm install maskchain . \
  --set gateway.enabled=false \
  --set admin.enabled=true \
  --set ingress.enabled=true \
  --set ingress.gateway.host=api.mycompany.com \
  --set apiKeys.OPENAI_API_KEY=sk-real-key
```

### Helm install with dependency update (one-liner):

```bash
helm dependency update && helm install maskchain .
```

---

## Upgrading

```bash
helm upgrade maskchain . -f my-values.yaml
```

Helm tracks changes; the ConfigMap checksum annotation triggers a rolling restart when config changes.

---

## Uninstall

```bash
helm uninstall maskchain
```

PersistentVolumeClaims are kept by default. To remove them:

```bash
kubectl delete pvc -l app.kubernetes.io/instance=maskchain
```

---

## Troubleshooting

### Pod not starting

Check logs and events:

```bash
kubectl describe pod -l app.kubernetes.io/instance=maskchain
kubectl logs -l app.kubernetes.io/instance=maskchain
```

### PostgreSQL/Valkey connection refused

Ensure the subchart services are running:

```bash
kubectl get svc -l app.kubernetes.io/instance=maskchain
```

For internal mode, the `initContainers.wait-postgres` waits for PostgreSQL. Check its logs:

```bash
kubectl logs -l app.kubernetes.io/component=gateway -c wait-postgres
```

### Config changes not applied

The Deployment annotation `checksum/config` triggers a rollout when ConfigMap changes. If you edit the ConfigMap manually, restart the pods:

```bash
kubectl rollout restart deployment -l app.kubernetes.io/instance=maskchain
```

### Duplicate Ingress warning when both gateway and all modes are enabled

`all.enabled=true` conflicts with `gateway.enabled` and `admin.enabled`. Set `gateway.enabled=false` and `admin.enabled=false` when using `all` mode.

---

## Development

### Template validation

```bash
helm lint .
helm template test-release . --dependency-update
```

### Render specific modes

```bash
# Admin only
helm template test-release . --set gateway.enabled=false,admin.enabled=true

# Combined all mode with ingress
helm template test-release . --set all.enabled=true,gateway.enabled=false,admin.enabled=false,ingress.enabled=true

# External database
helm template test-release . --set postgresql.enabled=false,postgresql.external.enabled=true,postgresql.external.dsn=...
```

### Updating dependencies

```bash
helm dependency update
```

---

## Values Reference

See [values.yaml](values.yaml) for the complete set of configurable parameters.
