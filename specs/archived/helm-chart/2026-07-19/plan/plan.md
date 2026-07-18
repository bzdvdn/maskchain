# Helm Chart для MaskChain — План

## Phase Contract

Inputs: spec (pass), inspect (pass), repo context.
Outputs: plan.md, data-model.md (no-change).
Stop if: нет.

## Цель

Создать Helm chart в `deployments/helm/maskchain/`, который одной командой `helm install` разворачивает MaskChain (gateway/admin/combined) + опционально PostgreSQL + Valkey из Bitnami-чартов (registry `bitnamilegacy`) либо подключается к внешним managed-инстансам. ConfigMap содержит все секции конфига с `${VAR}` placeholders, Secret `api-keys` инжектит env-переменные для подстановки.

## MVP Slice

Chart проходит `helm lint`, `helm template` для gateway.enabled, admin.enabled, all.enabled, gateway+admin, и `helm install` в kind/minikube поднимает gateway + postgres + valkey (internal) ИЛИ gateway с external postgres/valkey. AC-001, AC-002, AC-003, AC-004, AC-006, AC-010.

## First Validation Path

```bash
helm repo add bitnamilegacy https://charts.bitnami.com/bitnami
cd deployments/helm/maskchain
helm dependency update
helm lint .
helm template --set gateway.enabled=true . | grep -E 'Kind: (Deployment|Service|ConfigMap|Secret)'
kind create cluster
helm install test . --set postgresql.auth.password=test,valkey.auth.password=test
kubectl wait --for=condition=ready pod -l app.kubernetes.io/instance=test --timeout=120s
```

## Scope

- `deployments/helm/maskchain/` — Chart.yaml, values.yaml, templates/
- Bitnami subchart dependencies (postgresql, valkey) — опционально, conditional on `postgresql.external.enabled=false` / `valkey.external.enabled=false`
- External postgresql/valkey mode: `postgresql.external.enabled=true` + connection params через Secret api-keys
- Шаблоны: ConfigMap, Secret, Deployment, Service, Ingress, HTTPRoute (Gateway API, опционально), ServiceMonitor (опционально), PDB (опционально), NetworkPolicy (опционально)
- Маппинг values.config → ConfigMap.data["config.yaml"] (все секции из config.go)
- Маппинг values.apiKeys → Secret + envFrom в Deployment
- Существующий `deployments/docker-compose/` не меняется

## Performance Budget

- none — Helm chart не является runtime-компонентом

## Implementation Surfaces

- `deployments/helm/maskchain/Chart.yaml` — новая, metadata + dependencies
- `deployments/helm/maskchain/values.yaml` — новая, defaults для всех опций
- `deployments/helm/maskchain/templates/_helpers.tpl` — новая, именные шаблоны (name, labels, selectorLabels)
- `deployments/helm/maskchain/templates/configmap.yaml` — новая, ConfigMap с config.yaml
- `deployments/helm/maskchain/templates/secret.yaml` — новая, Secret api-keys
- `deployments/helm/maskchain/templates/deployment.yaml` — новая, Deployment с conditional container
- `deployments/helm/maskchain/templates/service.yaml` — новая, Service
- `deployments/helm/maskchain/templates/ingress.yaml` — новая, опционально
- `deployments/helm/maskchain/templates/servicemonitor.yaml` — новая, опционально
- `deployments/helm/maskchain/templates/pdb.yaml` — новая, опционально
- `deployments/helm/maskchain/templates/networkpolicy.yaml` — новая, опционально
- `deployments/helm/maskchain/templates/httproute.yaml` — новая, Gateway API HTTPRoute, опционально

## Bootstrapping Surfaces

- `deployments/helm/` — создать директорию
- `deployments/helm/maskchain/` — создать директорию
- `deployments/helm/maskchain/templates/` — создать директорию

## Влияние на архитектуру

- Нет — Helm chart изолирован в `deployments/`, не затрагивает исходный код
- Нет migration/compatibility — chart чистый деплоймент

## Acceptance Approach

- AC-001: `helm lint` → exit code + stdout check
- AC-002: `helm template --set gateway.enabled=true`, `--set admin.enabled=true`, `--set all.enabled=true`, `--set gateway.enabled=true,admin.enabled=true` → grep Kind для каждой
- AC-003: `helm template` → grep всех секций в ConfigMap
- AC-004: `helm template` → Secret существует, Deployment ссылается на `api-keys`
- AC-005: `helm template` → ConfigMap содержит только `${...}` для sensitive полей
- AC-006: `helm install` в kind → `kubectl wait --for=condition=ready`
- AC-007: `helm template --set ingress.enabled=false/true` → grep Ingress
- AC-008: `helm template --set servicemonitor.enabled=true/false` → grep ServiceMonitor
- AC-009: `helm template --set networkPolicy.enabled=true` → grep NetworkPolicy
- AC-010: `helm template --set postgresql.external.enabled=true,valkey.external.enabled=true` → нет StatefulSet для pg/valkey, ConfigMap содержит `${}` для DSN/addr
- AC-011: `helm template --set gatewayAPI.enabled=true` → HTTPRoute присутствует; `--set gatewayAPI.enabled=false` → HTTPRoute отсутствует

## Данные и контракты

- Data model не меняется — data-model.md будет no-change stub
- API-контракты не меняются
- Единственный контракт: структура values.yaml → ConfigMap.data["config.yaml"] должна соответствовать полям из `src/internal/infra/config/config.go`

## Стратегия реализации

- DEC-001 Единый chart с conditional компонентами
  Why: Один chart вместо трёх отдельных. `gateway.enabled`/`admin.enabled`/`all.enabled` переключают, какие контейнеры/сервисы/ingress рендерятся. all конфликтует с gateway/admin.
  Tradeoff: Условные блоки в шаблонах сложнее читать, чем отдельные chart'ы. Решение: чёткие `if` блоки в deployment.yaml, по одному container definition на компонент.
  Affects: deployment.yaml, service.yaml, ingress.yaml
  Validation: AC-002

- DEC-002 ConfigMap + Secret api-keys + envFrom
  Why: ConfigMap содержит полный конфиг с `${VAR}` placeholders. Secret содержит значения для подстановки. Pod через `envFrom[].secretRef.name` инжектит их как env vars, viper/app резолвит `${}`. Это стандартный K8s-паттерн.
  Tradeoff: Нельзя использовать `helm --set` для отдельных api_keys — нужно заполнять values.apiKeys. Упрощает секьюрность (secrets в K8s Secret, не в ConfigMap).
  Affects: configmap.yaml, secret.yaml, deployment.yaml, values.yaml
  Validation: AC-004, AC-005

- DEC-003 Bitnami subchart dependencies (conditional)
  Why: PostgreSQL и Valkey — зрелые Bitnami-чарты с большим числом опций. Subchart'ы через `dependencies` в Chart.yaml проще, чем инлайн-темплейты. Registry `bitnamilegacy` — требование пользователя. Conditional rendering через `condition: postgresql.enabled` / `condition: valkey.enabled` в dependencies.
  Tradeoff: `helm dependency update` — дополнительный шаг перед install. При external mode subchart'ы не рендерятся, но всё равно присутствуют в Chart.lock. Conditional dependency — стандартное решение Helm.
  Affects: Chart.yaml, values.yaml
  Validation: AC-006 (internal), AC-010 (external)

- DEC-004 Плоская структура templates/ (без subchart внутри)
  Why: Один компонент MaskChain (gateway/admin/all) — нет смысла в subchart для каждого режима. conditional container достаточно.
  Tradeoff: Если в будущем gateway и admin потребуют radically разных ресурсов (HPA, different probe config), придётся рефакторить.
  Affects: вся templates/
  Validation: AC-002

- DEC-005 Values структура: config.configSection.key для ConfigMap, apiKeys.key для Secret
  Why: Чёткое разделение: что идёт в ConfigMap (config.*), что в Secret (apiKeys.*). Пользователь сразу видит, где чувствительные данные.
  Tradeoff: Все секции конфига должны быть перечислены в values.yaml явно, что увеличивает его размер. Компенсация: подробный README.
  Affects: values.yaml, configmap.yaml (toYaml на .Values.config), secret.yaml
  Validation: AC-003, AC-004

## Incremental Delivery

### MVP (Первая ценность)

Chart.yaml + values.yaml + dependency update → ConfigMap + Secret + Deployment (gateway.enabled=true) + Service + postgres/valkey subchart (internal) OR ConfigMap с `${POSTGRES_DSN}`/`${VALKEY_ADDR}` (external).
AC покрываются: AC-001, AC-002 (gateway), AC-003, AC-004, AC-005, AC-006, AC-010.

Валидация: `helm dependency update && helm lint && kind install && helm install && kubectl wait --for=condition=ready`

### Итеративное расширение

- Шаг 2: admin.enabled, all.enabled, gateway+admin → AC-002 (все комбинации)
- Шаг 3: Ingress + Gateway API (HTTPRoute) → AC-007, AC-011
- Шаг 4: PDB + ServiceMonitor + NetworkPolicy → AC-008, AC-009
- Шаг 5: test-connection pod, README, examples

## Порядок реализации

1. Scaffold: Chart.yaml + values.yaml + _helpers.tpl + пустые templates/
2. ConfigMap + Secret templates (без ConfigMap приложение не стартует)
3. Deployment + Service templates (gateway mode first, оба режима DB)
4. Зависимости Bitnami в Chart.yaml + values (conditional)
5. Проверка MVP: `helm dependency update && helm lint && helm template --set postgresql.external.enabled=true && helm template` (internal) && kind install
6. admin.enabled, all.enabled, gateway+admin.enabled
7. Ingress, PDB, ServiceMonitor, NetworkPolicy (опционально)
8. test-connection, README

Параллельно: 1+2, 3+4, 6+7

## Риски

- Отсутствие bitnamilegacy registry у пользователя
  Mitigation: инструкция `helm repo add` в README, error message в Chart.yaml (аннотация)
- Несовместимость версий Bitnami chart'ов
  Mitigation: указать version range в dependencies, протестировать с актуальными версиями
- Несоответствие структуры ConfigMap актуальным полям config.go
  Mitigation: values.yaml генерируется по config.go; при изменении config.go — обновлять values.yaml
- Kind не установлен для AC-006
  Mitigation: AC-006 — для CI; локально достаточно AC-001–005

## Rollout и compatibility

- Chart с нуля — rollout не требуется
- При обновлении: `helm upgrade` обрабатывает изменения (deployMode switch удалит/создаст ресурсы)
- Для перехода с gateway+admin на combined: включить `all.enabled=true`, выключить `gateway.enabled=false`, `admin.enabled=false`.

## Проверка

- Automated: `helm lint`, `helm template`, `helm install` в kind
- Manual: просмотр rendered YAML (`helm template`), проверка всех секций в ConfigMap
- AC-001–010 покрывают все проверки — каждая имеет однозначный observable outcome

## Соответствие конституции

- нет конфликтов — Helm chart в `deployments/` не затрагивает исходный код, архитектуру или workflow
