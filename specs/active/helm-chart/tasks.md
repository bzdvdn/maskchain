# Helm Chart для MaskChain — Задачи

## Phase Contract

Inputs: plan.md, spec.md.
Outputs: упорядоченные исполнимые задачи с покрытием AC.
Stop if: нет.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `deployments/helm/maskchain/Chart.yaml` | T1.1, T2.5, T7.1 |
| `deployments/helm/maskchain/.helmignore` | T1.1 |
| `deployments/helm/maskchain/values.yaml` | T1.1, T2.1, T2.2, T3.1, T4.1, T5.1, T5.5 |
| `deployments/helm/maskchain/templates/_helpers.tpl` | T1.1 |
| `deployments/helm/maskchain/templates/configmap.yaml` | T2.1 |
| `deployments/helm/maskchain/templates/secret.yaml` | T2.2, T7.3 |
| `deployments/helm/maskchain/templates/deployments/gateway.yaml` | T2.3 |
| `deployments/helm/maskchain/templates/deployments/admin.yaml` | T4.1 |
| `deployments/helm/maskchain/templates/deployments/all.yaml` | T4.1 |
| `deployments/helm/maskchain/templates/services/gateway.yaml` | T2.4 |
| `deployments/helm/maskchain/templates/services/admin.yaml` | T4.1 |
| `deployments/helm/maskchain/templates/services/all.yaml` | T4.1 |
| `deployments/helm/maskchain/templates/ingresses/gateway.yaml` | T5.1 |
| `deployments/helm/maskchain/templates/ingresses/admin.yaml` | T5.1 |
| `deployments/helm/maskchain/templates/ingresses/all.yaml` | T5.1 |
| `deployments/helm/maskchain/templates/httproute.yaml` | T5.5 |
| `deployments/helm/maskchain/templates/pdb.yaml` | T5.2 |
| `deployments/helm/maskchain/templates/servicemonitor.yaml` | T5.3 |
| `deployments/helm/maskchain/templates/networkpolicy.yaml` | T5.4 |
| `deployments/helm/maskchain/templates/tests/test-connection.yaml` | T6.1 |
| `deployments/helm/maskchain/README.md` | T6.2 |

## Implementation Context

- Цель MVP: Helm chart `deployments/helm/maskchain/` рендерит валидные манифесты (ConfigMap + Secret + Deployment + Service) для gateway с internal PostgreSQL/Valkey из Bitnami subchart'ов.
- Инварианты/семантика:
  - ConfigMap.data["config.yaml"] — полный YAML-конфиг с `${VAR}` placeholders для sensitive значений
  - Secret `api-keys` — base64-encoded env vars, инжектятся через `envFrom[].secretRef` в Deployment
  - PostgreSQL/Valkey: `postgresql.external.enabled=true/false` переключает subchart vs external
  - `gateway.enabled`/`admin.enabled`/`all.enabled` — флаги включения компонентов. `all.enabled=true` конфликтует с `gateway` и `admin`.
- Контракты/протокол:
  - ConfigMap: `{{ .Values.config | toYaml }}` → `config.yaml` ключ
  - Secret: `{{ .Values.apiKeys | toYaml }}` → env-переменные
  - Image: `{{ .Values.image.repository }}:{{ .Values.image.tag }}`
- Границы scope: не меняем `deployments/docker-compose/` и исходный код
- Proof signals: `helm lint` exit 0, `helm template` выводит все 4+ Kind, `helm install` в kind = Ready pods

## Фаза 1: Scaffold

Цель: создать структуру chart'а, базовые файлы и подключить Bitnami dependencies.

- [x] T1.1 Создать scaffold chart'а: `deployments/helm/maskchain/` с `Chart.yaml`, `.helmignore`, `values.yaml` (defaults для config, apiKeys, postgresql, valkey, image), `templates/_helpers.tpl` (name, labels, selectorLabels).
  Touches: deployments/helm/maskchain/Chart.yaml, .helmignore, values.yaml, templates/_helpers.tpl
  AC: AC-001

- [x] T1.2 Добавить Bitnami dependencies в Chart.yaml (postgresql, valkey, registry bitnamilegacy, conditional через `condition: postgresql.enabled` / `condition: valkey.enabled`) и выполнить `helm dependency update`.
  Touches: deployments/helm/maskchain/Chart.yaml
  AC: AC-001, AC-006

## Фаза 2: Core templates MVP

Цель: ConfigMap + Secret + Deployment (gateway) + Service + internal DB.

- [x] T2.1 Реализовать `templates/configmap.yaml`: ConfigMap с ключом `config.yaml`, содержимое — `{{ .Values.config | toYaml }}`. Все секции из config.go (log, server, database, valkey, mask, shield, routing, egress, session, admin, otel, ratelimit, analytics, dictionary_cache, tenants). Sensitive значения через `${VAR}` placeholder в values.
  Touches: deployments/helm/maskchain/templates/configmap.yaml, values.yaml
  AC: AC-001, AC-003, AC-005

- [x] T2.2 Реализовать `templates/secret.yaml`: Secret `{{ .Release.Name }}-api-keys`. При internal subchart (postgresql/valkey enabled + не external) — авто-генерация `POSTGRES_DSN`, `VALKEY_ADDR`, `VALKEY_PASSWORD` из настроек subchart. При external mode — из postgresql.external.dsn / valkey.external.addr. User override в `apiKeys` перезаписывает авто-генерацию. Каждый ключ — env-переменная, значение — b64enc.
  Touches: deployments/helm/maskchain/templates/secret.yaml, values.yaml
  AC: AC-001, AC-004, AC-005

- [x] T2.3 Реализовать `templates/services/gateway.yaml`: Deployment + Service + Ingress для gateway. Container image, ports, envFrom, probes, securityContext. Conditional: рендерится только при `gateway.enabled=true`. Ingress — при `ingress.enabled=true` и `all.enabled=false`.
  Touches: deployments/helm/maskchain/templates/services/gateway.yaml
  AC: AC-001, AC-004, AC-006

- [x] T2.4 Реализовать `templates/services/gateway.yaml`: Service для gateway (port из config.server.port). Selector с component: gateway.
  Touches: deployments/helm/maskchain/templates/services/gateway.yaml
  AC: AC-001, AC-006

## Фаза 3: External DB mode

Цель: conditional external postgresql/valkey.

- [x] T3.1 Реализовать conditional режим для postgresql/valkey: при `postgresql.external.enabled=true` subchart postgresql не рендерится (через condition в dependencies), ConfigMap содержит `${POSTGRES_DSN}` placeholder, Secret api-keys содержит POSTGRES_DSN значение. Аналогично для valkey. При external mode NetworkPolicy egress должен разрешать target CIDR (опционально).
  Touches: deployments/helm/maskchain/Chart.yaml, values.yaml, templates/configmap.yaml, templates/secret.yaml, templates/networkpolicy.yaml
  AC: AC-010, AC-005

## Фаза 4: Admin и Combined флаги

Цель: admin и combined режимы.

- [x] T4.1 Реализовать `templates/services/admin.yaml` и `templates/services/all.yaml`: каждый содержит Deployment + Service + Ingress для своего компонента. Conditional на admin.enabled / all.enabled.
  Touches: deployments/helm/maskchain/templates/services/admin.yaml, templates/services/all.yaml
  AC: AC-002

## Фаза 5: Optional resources

Цель: Ingress, PDB, ServiceMonitor, NetworkPolicy.

- [x] T5.1 Ingress встроен в `templates/services/gateway.yaml`, `admin.yaml`, `all.yaml` — каждый компонент содержит свой Ingress блок, conditional на `ingress.enabled`.

- [x] T5.2 Реализовать `templates/pdb.yaml`: опциональный PodDisruptionBudget (minAvailable или maxUnavailable из values).
  Touches: deployments/helm/maskchain/templates/pdb.yaml, values.yaml
  AC: AC-008

- [x] T5.3 Реализовать `templates/servicemonitor.yaml`: опциональный ServiceMonitor для prometheus-operator. Селектор по labels chart'а, endpoints port name "metrics", interval из values.
  Touches: deployments/helm/maskchain/templates/servicemonitor.yaml, values.yaml
  AC: AC-008

- [x] T5.4 Реализовать `templates/networkpolicy.yaml`: опциональная NetworkPolicy. Ingress: только от Ingress-контроллера (namespaceSelector) и внутри namespace. Egress: к postgresql/valkey (по имени сервиса) или external CIDR, и DNS (udp 53).
  Touches: deployments/helm/maskchain/templates/networkpolicy.yaml, values.yaml
  AC: AC-009

- [x] T5.5 Реализовать `templates/httproute.yaml`: Gateway API HTTPRoute, опционально (`gatewayAPI.enabled=true`). Gateway parentRef, hostname, backendRefs. Conditional: HTTPRoute для gateway при gateway.enabled или all.enabled; для admin при admin.enabled или all.enabled.
  Touches: deployments/helm/maskchain/templates/httproute.yaml, values.yaml
  AC: AC-011

## Фаза 6: Тесты и документация

Цель: тест-поды, README.

- [x] T6.1 Реализовать `templates/tests/test-connection.yaml`: pod с `curlimages/curl`, который делает GET на gateway /health и проверяет 200.
  Touches: deployments/helm/maskchain/templates/tests/test-connection.yaml
  AC: AC-006

- [x] T6.2 Написать README.md для chart'а: установка (helm repo add bitnamilegacy, dependency update, install), пример values для internal/external, gateway/admin/all modes, все опции.
  Touches: deployments/helm/maskchain/README.md
  AC: AC-001

## Фаза 7: Проверка

Цель: финальная валидация chart'а.

- [ ] T7.1 Проверить `helm lint` и `helm template` для комбинаций: gateway.enabled, admin.enabled, all.enabled, gateway+admin, × dbMode={internal,external} × routeMode={ingress,gatewayAPI}. ConfigMap содержит все секции, Secret — api-keys, шаблоны без ошибок.
  Touches: deployments/helm/maskchain/templates/*.yaml
  AC: AC-001, AC-002, AC-003, AC-004, AC-005, AC-007, AC-008, AC-009, AC-010, AC-011

- [ ] T7.2 Проверить `helm install` в kind (minikube) для internal mode: gateway + postgresql + valkey, все pod'ы Ready за <120s. Проверить external mode template (без install, т.к. нет external DB).
  Touches: deployments/helm/maskchain/
  AC: AC-006

## Покрытие критериев приемки

- AC-001 → T1.1, T1.2, T2.1, T2.2, T2.3, T2.4, T7.1
- AC-002 → T4.1, T7.1
- AC-003 → T2.1, T7.1
- AC-004 → T2.2, T7.1
- AC-005 → T2.1, T2.2, T3.1, T7.1
- AC-006 → T1.2, T2.3, T2.4, T6.1, T7.2
- AC-007 → T5.1, T7.1
- AC-008 → T5.2, T5.3, T7.1
- AC-009 → T5.4, T7.1
- AC-010 → T3.1, T7.1
- AC-011 → T5.5, T7.1

## Заметки

- T1.1–T2.4 — критическая цепочка (scaffold → core templates → dependencies). Параллельно: T2.1+T2.2, T2.3+T2.4.
- T3.1 можно делать параллельно с T4.1.
- T5.1–T5.4 независимы, можно в любом порядке.
- T6.1–T6.2 после T1.1–T2.4 (зависит от существования Service).
- T7.1 и T7.2 — финальная валидация после всех задач.
