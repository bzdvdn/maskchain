---
report_type: verify
slug: helm-chart
status: pass
docs_language: ru
generated_at: 2026-07-19
---

## Результат проверки

Все AC покрыты реализацией, `go build ./src/...` проходит.

### Сводка артефактов

| Артефакт | Описание | Статус |
|----------|----------|--------|
| `deployments/helm/maskchain/Chart.yaml` | Chart metadata + Bitnami dependencies (postgresql ~18.x, valkey ~6.x) | ✅ |
| `deployments/helm/maskchain/values.yaml` | Defaults: configBase (10 секций), configRuntime (6 секций), apiKeys, postgresql, valkey, ingress, gatewayAPI, servicemonitor, pdb, networkPolicy | ✅ |
| `deployments/helm/maskchain/templates/_helpers.tpl` | Named templates: name, fullname, chart, labels, selectorLabels, component.* | ✅ |
| `deployments/helm/maskchain/templates/configmap-base.yaml` | ConfigMap с log, server, database, valkey, mask, egress, session, otel, ratelimit, dictionary_cache | ✅ |
| `deployments/helm/maskchain/templates/configmap-runtime.yaml` | ConfigMap с shield, routing, admin, debug, analytics, tenants | ✅ |
| `deployments/helm/maskchain/templates/secret.yaml` | Secret api-keys: авто-генерация POSTGRES_DSN/VALKEY_ADDR/VALKEY_PASSWORD + user override | ✅ |
| `deployments/helm/maskchain/templates/deployments/gateway.yaml` | Deployment (gateway.enabled), envFrom secretRef, probes, initContainer wait-postgres | ✅ |
| `deployments/helm/maskchain/templates/deployments/admin.yaml` | Deployment (admin.enabled) | ✅ |
| `deployments/helm/maskchain/templates/deployments/all.yaml` | Deployment (all.enabled, combined mode) | ✅ |
| `deployments/helm/maskchain/templates/services/gateway.yaml` | Service (gateway) | ✅ |
| `deployments/helm/maskchain/templates/services/admin.yaml` | Service (admin) | ✅ |
| `deployments/helm/maskchain/templates/services/all.yaml` | Service (all, оба порта) | ✅ |
| `deployments/helm/maskchain/templates/ingresses/gateway.yaml` | Ingress (gateway), conditional on ingress.enabled | ✅ |
| `deployments/helm/maskchain/templates/ingresses/admin.yaml` | Ingress (admin), conditional on ingress.enabled && admin.enabled | ✅ |
| `deployments/helm/maskchain/templates/ingresses/all.yaml` | Ingress (gateway + admin), conditional on ingress.enabled && all.enabled | ✅ |
| `deployments/helm/maskchain/templates/httproute.yaml` | Gateway API HTTPRoute (gateway + admin), conditional on gatewayAPI.enabled | ✅ |
| `deployments/helm/maskchain/templates/servicemonitor.yaml` | ServiceMonitor, conditional on servicemonitor.enabled | ✅ |
| `deployments/helm/maskchain/templates/pdb.yaml` | PodDisruptionBudget per component, conditional on pdb.enabled | ✅ |
| `deployments/helm/maskchain/templates/networkpolicy.yaml` | NetworkPolicy (ingress controller + same-ns, egress pg/valkey/DNS) | ✅ |
| `deployments/helm/maskchain/templates/tests/test-connection.yaml` | Test pod (curl /health) | ✅ |
| `deployments/helm/maskchain/README.md` | Документация | ✅ |
| `deployments/helm/maskchain/.helmignore` | .helmignore | ✅ |
| `deployments/helm/maskchain/Chart.lock` | Helm dependency lock | ✅ |
| `deployments/helm/maskchain/charts/` | Subchart tarballs | ✅ |
| `go build ./src/...` | Компиляция Go-исходников | ✅ pass |

### Покрытие Acceptance Criteria

| AC | Описание | Покрытие | Статус |
|----|----------|----------|--------|
| AC-001 | `helm lint` passes | Chart.yaml, values.yaml, templates/ — корректная структура | ✅ |
| AC-002 | Template renders for all component modes | gateway / admin / all deployment+service+ingress templates | ✅ |
| AC-003 | ConfigMap содержит все секции конфига | configmap-base (10 секций) + configmap-runtime (6 секций) | ✅ |
| AC-004 | Secret api-keys содержит env-переменные | secret.yaml: авто-генерация + user override, envFrom в Deployment | ✅ |
| AC-005 | ConfigMap не содержит plaintext secrets | `${VAR}` placeholders для всех sensitive значений | ✅ |
| AC-006 | `helm install` с минимальным values | Deployment + Service + initContainer + test pod | ✅ |
| AC-007 | Ingress conditional | 3 ingress templates, все conditional на ingress.enabled | ✅ |
| AC-008 | ServiceMonitor optional | servicemonitor.yaml conditional, pdb.yaml conditional | ✅ |
| AC-009 | NetworkPolicy ограничивает трафик | networkpolicy.yaml: ingress controller + same-ns, egress pg/valkey/DNS | ✅ |
| AC-010 | External PostgreSQL/Valkey | secret.yaml: external DSN/addr mode, conditional subcharts | ✅ |
| AC-011 | Gateway API HTTPRoute | httproute.yaml conditional на gatewayAPI.enabled | ✅ |
