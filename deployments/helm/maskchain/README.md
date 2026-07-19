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
├── configmap-base.yaml      # Infrastructure config (base, rarely changes)
├── configmap-runtime.yaml   # Business-logic config (runtime, changes more often)
├── secret.yaml              # Shared API keys + auto-generated DB secrets
├── deployments/             # Component Deployments
│   ├── gateway.yaml         (gateway.enabled)
│   ├── admin.yaml           (admin.enabled)
│   └── all.yaml             (all.enabled)
├── services/                # Component Services
│   ├── gateway.yaml
│   ├── admin.yaml
│   └── all.yaml
├── ingresses/               # Per-component Ingresses
│   ├── gateway.yaml
│   ├── admin.yaml
│   └── all.yaml
├── gateway-api/             # Per-component Gateway API HTTPRoutes
│   ├── gateway.yaml
│   ├── admin.yaml
│   └── all.yaml
├── servicemonitor.yaml      # Prometheus ServiceMonitor
├── pdb.yaml                 # PodDisruptionBudget
├── networkpolicy.yaml       # NetworkPolicy
└── tests/test-connection.yaml
```

Each component is self-contained: a Deployment with its own selector label (`app.kubernetes.io/component: <name>`), paired Service, and optional Ingress / HTTPRoute.

All component-scoped settings (`replicaCount`, `service`, `resources`, `ingress`, `gatewayAPI`) are configured **inside** each component block — see [Configuration Reference](#configuration-reference).

---

## Component Modes

Three mutually exclusive component flags control what runs:

| Flag | Default | Description |
|------|---------|-------------|
| `gateway.enabled` | `true` | Separate gateway Deployment (port `configBase.server.port`) |
| `admin.enabled` | `false` | Separate admin Deployment (port `configBase.server.admin_port`) |
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

### Per-Component Settings

Each component (`gateway` / `admin` / `all`) has its own `replicaCount`, `service`, `resources`, `ingress`, and `gatewayAPI`:

```yaml
gateway:
  enabled: true
  image:
    repository: bzdvdn/maskchain-gateway
    tag: latest
    pullPolicy: IfNotPresent
  replicaCount: 1                              # component replicas
  service:
    type: ClusterIP
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 500m
      memory: 256Mi
  ingress:                                      # see Ingress section below
    enabled: false
    className: ""
    host: api.maskchain.local
    path: /
    annotations: {}
    tls: []
  gatewayAPI:                                   # see Gateway API section below
    enabled: false
    hostname: api.maskchain.local
    gatewayName: maskchain-gateway
    gatewayNamespace: ""
    tls:
      enabled: false
      certificateRef:
        name: ""
  priorityClassName: ""
  pdb:
    enabled: false
    minAvailable: 1
  networkPolicy:
    enabled: false
    ingressControllerNamespace: ingress-nginx
    externalCIDRs: []

admin:
  enabled: false
  image:
    repository: bzdvdn/maskchain-admin
    tag: latest
    pullPolicy: IfNotPresent
  replicaCount: 1
  service:
    type: ClusterIP
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 500m
      memory: 256Mi
  ingress:
    enabled: false
    className: ""
    host: admin.maskchain.local
    path: /
    annotations: {}
    tls: []
  gatewayAPI:
    enabled: false
    hostname: admin.maskchain.local
    gatewayName: maskchain-gateway
    gatewayNamespace: ""
  priorityClassName: ""
  pdb:
    enabled: false
    minAvailable: 1
  networkPolicy:
    enabled: false
    ingressControllerNamespace: ingress-nginx
    externalCIDRs: []

all:
  enabled: false
  image:
    repository: bzdvdn/maskchain
    tag: latest
    pullPolicy: IfNotPresent
  replicaCount: 1
  service:
    type: ClusterIP
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 500m
      memory: 256Mi
  ingress:
    enabled: false
    className: ""
    host: api.maskchain.local
    path: /
    annotations: {}
    tls: []
    admin_host: admin.maskchain.local          # admin Ingress settings in all mode
    admin_path: /
    admin_annotations: {}
    admin_tls: []
  gatewayAPI:
    enabled: false
    hostname: api.maskchain.local
    adminHostname: admin.maskchain.local       # admin hostname in all mode
    gatewayName: maskchain-gateway
    gatewayNamespace: ""
  priorityClassName: ""
  pdb:
    enabled: false
    minAvailable: 1
  networkPolicy:
    enabled: false
    ingressControllerNamespace: ingress-nginx
    externalCIDRs: []
```

### Application Config

Config is split into two layers in `values.yaml` — `configBase` (infrastructure) and `configRuntime` (business logic). Each maps to its own ConfigMap in Kubernetes.

| Values section | ConfigMap | Contents | Change frequency |
|----------------|-----------|----------|-----------------|
| `configBase` | `config-base` | `log`, `server`, `database`, `valkey`, `mask`, `egress`, `session`, `otel`, `ratelimit`, `dictionary_cache` | Rarely (infrastructure) |
| `configRuntime` | `config-runtime` | `shield`, `routing`, `admin`, `debug`, `analytics`, `tenants` | More often (business logic) |

Both are mounted to `/etc/maskchain/conf.d/` in the container. The binary reads all `*.yaml` files from this directory and deep-merges them (last file wins — `99-config-runtime.yaml` overrides `00-config-base.yaml`). This means changing routing or analytics does **not** trigger a Pod restart (the filesystem syncs automatically via ConfigMap volume).

ConfigRuntime blocks are empty by default — uncomment and fill as needed:

```yaml
configRuntime:
  # shield:
  #   action_on_suspicious: mask
  #   tenant_model_mapping:
  #     default:
  #       "gpt-4o": "strict"
  shield: {}

  # routing:
  #   providers:
  #     - name: openai
  #       api_type: openai
  #       base_url: "https://api.openai.com"
  #       api_keys:
  #         - "${OPENAI_API_KEY}"
  #       timeout: 120s
  #       priority: 1
  #   rules:
  #     - tenant: default
  #       routes:
  #         - model: "gpt-4o"
  #           providers: ["openai"]
  routing:
    providers: []
    rules: []

  # admin:
  #   username: admin
  #   password: "${ADMIN_PASSWORD}"
  #   session_ttl: 30m
  admin: {}
  debug: {}
  analytics: {}
  tenants: {}
```

Sensitive values (`${POSTGRES_DSN}`, `${VALKEY_PASSWORD}`, `${OPENAI_API_KEY}`, `${ADMIN_PASSWORD}`, `${DEFAULT_API_KEY}`) are resolved at runtime from environment variables injected via the `apiKeys` Secret. See [API Keys & Secrets](#api-keys--secrets).

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

Configured **per-component** inside each component block:

```yaml
gateway:
  ingress:
    enabled: false
    className: "nginx"
    host: api.maskchain.local
    path: /
    annotations:
      nginx.ingress.kubernetes.io/ssl-redirect: "true"
    tls:
      - hosts: [api.maskchain.local]
        secretName: maskchain-tls

admin:
  ingress:
    enabled: false
    className: "nginx"
    host: admin.maskchain.local
    path: /
    annotations: {}
    tls: []

all:
  ingress:
    enabled: false
    className: "nginx"
    host: api.maskchain.local
    path: /
    annotations: {}
    tls: []
    admin_host: admin.maskchain.local            # separate admin Ingress settings
    admin_path: /
    admin_annotations: {}
    admin_tls: []
```

- **gateway mode** (`gateway.enabled=true`, `all.enabled=false`): single Ingress → `-gateway` Service
- **admin mode** (`admin.enabled=true`): single Ingress → `-admin` Service
- **all mode** (`all.enabled=true`): two Ingresses (gateway + admin) → `-all` Service on respective ports

### Gateway API (HTTPRoute)

Also configured **per-component**:

```yaml
gateway:
  gatewayAPI:
    enabled: false
    hostname: api.maskchain.local
    gatewayName: maskchain-gateway
    gatewayNamespace: ""
    tls:
      enabled: false
      certificateRef:
        name: ""

admin:
  gatewayAPI:
    enabled: false
    hostname: admin.maskchain.local
    gatewayName: maskchain-gateway

all:
  gatewayAPI:
    enabled: false
    hostname: api.maskchain.local
    adminHostname: admin.maskchain.local
    gatewayName: maskchain-gateway
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

Configured **per-component** inside each component block:

```yaml
gateway:
  pdb:
    enabled: false
    minAvailable: 1
admin:
  pdb:
    enabled: false
    minAvailable: 1
all:
  pdb:
    enabled: false
    minAvailable: 1
```

Creates a PDB for each component that has `pdb.enabled=true`.

### NetworkPolicy

Configured **per-component** inside each component block:

```yaml
gateway:
  networkPolicy:
    enabled: false
    ingressControllerNamespace: ingress-nginx
    externalCIDRs: []
admin:
  networkPolicy:
    enabled: false
    ingressControllerNamespace: ingress-nginx
    externalCIDRs: []
all:
  networkPolicy:
    enabled: false
    ingressControllerNamespace: ingress-nginx
    externalCIDRs: []
```

Restricts ingress traffic per component:
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

Override individual values (note per-component paths):

```bash
helm install maskchain . \
  --set gateway.enabled=false \
  --set admin.enabled=true \
  --set admin.ingress.enabled=true \
  --set admin.ingress.host=admin.mycompany.com \
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

The Deployment annotation `checksum/config-base` and `checksum/config-runtime` trigger a rollout when corresponding ConfigMaps change. If you edit the ConfigMap manually, restart the pods:

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
helm template test-release . --set all.enabled=true,gateway.enabled=false,admin.enabled=false,all.ingress.enabled=true

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
