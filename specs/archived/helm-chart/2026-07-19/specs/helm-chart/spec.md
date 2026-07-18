# Helm Chart для MaskChain

## Scope Snapshot

- In scope: Helm chart для деплоя MaskChain в Kubernetes с зависимостями PostgreSQL и Valkey из Bitnami-чартов, образами Bitnami из registry `bitnamilegacy`.
- Out of scope: управление жизненным циклом chart'а (registry, CI/CD publish), мониторинг/алертинг как часть chart'а (prometheus-operator, grafana dashboards).

## Цель

DevOps-инженер получает единственный `helm install` для развёртывания MaskChain (gateway, admin, combined) в Kubernetes. PostgreSQL и Valkey поднимаются как subchart'ы Bitnami с образами из `bitnamilegacy`. Успех определяется тем, что после `helm install` все поды переходят в Ready, и gateway отвечает 200 на `/health`.

## Основной сценарий

1. DevOps клонирует репозиторий, переходит в `deployments/helm/`.
2. Заполняет `values.yaml` (или `--values prod.yaml`): образы MaskChain, пароли, размеры PVC, реплики, ingress.
3. Запускает `helm dependency update` — Pullтинг PostgreSQL/Valkey subchart'ы из Bitnami с registry `bitnamilegacy`.
4. Запускает `helm install maskchain .` — создаются namespace, ConfigMap с конфигом, Deployments/StatefulSets, Services, Ingress, PVC.
5. Все поды переходят в Ready, gateway обслуживает трафик, admin-UI доступен.
6. Fallback: если dependency update не находит bitnamilegacy registry — инструкция по настройке `helm repo add bitnamilegacy ...`.

## User Stories

- P1 Story: Как DevOps, я хочу развернуть MaskChain одной командой `helm install`, чтобы не писать манифесты вручную.
- P2 Story: Как DevOps, я хочу переключать режим (gateway+admin отдельно vs combined) через values, чтобы гибко выбирать topologii.
- P3 Story: Как DevOps, я хочу видеть ConfigMap с полным конфигом MaskChain (все поля из config.go), чтобы не монтировать внешний файл.

## MVP Slice

P1 Story: `helm install maskchain .` поднимает PostgreSQL + Valkey + MaskChain (deployment gateway). AC-001, AC-003, AC-004, AC-006.

## First Deployable Outcome

Chart проходит `helm lint` и `helm template` без ошибок. После `helm install` с минимальным values (только пароли) все поды Ready, gateway отвечает 200 на `/health`.

## Scope

- Helm chart в `deployments/helm/maskchain/` с Chart.yaml, values.yaml, шаблонами.
- Subchart'ы: bitnami/postgresql, bitnami/valkey (через зависимости, registry bitnamilegacy) — опционально, отключаются через `postgresql.external.enabled=true` / `valkey.external.enabled=true`.
- Компоненты: `gateway.enabled`, `admin.enabled`, `all.enabled` (combined). `all` конфликтует с `gateway` и `admin` — если `all.enabled=true`, остальные игнорируются.
- ConfigMap из values: все секции `src/internal/infra/config/config.go` (log, server, database, valkey, mask, shield, routing, egress, session, admin, otel, ratelimit, analytics, dictionary_cache, tenants). Sensitive значения (пароли, ключи) — через `${VAR}` placeholder, резолвятся из env.
- Secret `api-keys` с env-переменными для подстановки `${VAR}` в ConfigMap (OPENAI_API_KEY, DEFAULT_API_KEY, ADMIN_PASSWORD, database.dsn, valkey.password и т.д.).
- Ingress-шаблон для gateway (основной) и admin (опционально).
- Gateway API шаблон: HTTPRoute для gateway и admin как альтернатива Ingress (включение через `gatewayAPI.enabled=true`).
- ServiceMonitor шаблон (для prometheus-operator, опционально).
- PodDisruptionBudget (опционально).
- NetworkPolicy (базовая, allow-same-namespace).

## Контекст

- Маппинг полей ConfigMap соответствует структурам в `src/internal/infra/config/config.go`.
- Bitnami-зависимости используют registry `bitnamilegacy` — `helm repo add bitnamilegacy https://charts.bitnami.com/bitnami`.
- Образы MaskChain собираются из Dockerfile.gateway, Dockerfile.admin, Dockerfile.combined и публикуются в registry пользователя — указываются в `values.image.*`.
- Существующий `deployments/docker-compose/` остаётся без изменений.

## Зависимости

- bitnami/postgresql (>=15.x) — PostgreSQL StatefulSet и сервис (опционально, только если `postgresql.external.enabled=false`).
- bitnami/valkey (>=8.x) — Valkey StatefulSet и сервис (опционально, только если `valkey.external.enabled=false`).
- MaskChain Docker images — пользовательские, не Bitnami.

## Требования

- RQ-001 Chart ДОЛЖЕН поддерживать три флага включения: `gateway.enabled`, `admin.enabled`, `all.enabled`. `all.enabled=true` исключает gateway и admin.
- RQ-002 PostgreSQL и Valkey ДОЛЖНЫ поддерживать два режима: `internal` (subchart Bitnami, registry bitnamilegacy) и `external` (пользовательский endpoint, credentials через Secret `api-keys`).
- RQ-003 ConfigMap ДОЛЖЕН генерироваться из values и содержать ВСЕ секции конфига (log, server, database, valkey, mask, shield, routing, egress, session, admin, otel, ratelimit, analytics, dictionary_cache, tenants). Sensitive значения — через `${VAR}` placeholder.
- RQ-004 Chart ДОЛЖЕН проходить `helm lint` без ошибок.
- RQ-005 Secret `api-keys` ДОЛЖЕН создаваться из values.apiKeys и инжектиться как env vars в Deployment/StatefulSet для подстановки `${VAR}` в ConfigMap.
- RQ-006 ConfigMap НЕ ДОЛЖЕН содержать plaintext secrets (passwords, tokens, API keys). Все sensitive значения — только в Secret `api-keys`.
- RQ-007 Service/Ingress/HTTPRoute ДОЛЖНЫ создаваться только для включённых компонентов (gateway.enabled=false → no gateway Service).
- RQ-008 PodDisruptionBudget ДОЛЖЕН быть опциональным (enabled/disabled в values).
- RQ-009 ServiceMonitor ДОЛЖЕН быть опциональным и поддерживать `prometheus.io/scrape` аннотации как fallback.
- RQ-010 Chart ДОЛЖЕН поддерживать external PostgreSQL/Valkey: при `postgresql.external.enabled=true` subchart postgresql НЕ рендерится, значения DSN берутся из `values.config.database.dsn` и инжектятся через Secret. Аналогично для `valkey.external.enabled=true`.
- RQ-011 Chart ДОЛЖЕН поддерживать Gateway API как альтернативу Ingress: при `gatewayAPI.enabled=true` создаются HTTPRoute ресурсы вместо Ingress. Параметры (host, TLS, backendRefs) из values.gatewayAPI.

## Вне scope

- Helm registry/chartmuseum для публикации chart'а.
- GitOps интеграция (ArgoCD/Flux manifests).
- Prometheus-operator установка (CRD предустановлены администратором кластера).
- Istio/ServiceMesh интеграция.
- Gateway API CRD установка (CRD предустановлены администратором кластера).
- Многоинстансная tenants-as-chart-values (каждый tenant = отдельный helm release).
- Управление жизненным циклом subchart'ов upgrade (пользователь делает `helm upgrade` для Bitnami chart'ов стандартным способом).

## Критерии приемки

### AC-001 Helm lint passes

- Почему это важно: Базовый синтаксис и валидность chart'а.
- **Given** chart в `deployments/helm/maskchain/` с корректными dependencies
- **When** выполняется `helm lint deployments/helm/maskchain`
- **Then** exit code 0, stdout содержит `1 chart(s) linted, 0 chart(s) failed`
- Evidence: вывод `helm lint`

### AC-002 Template renders for all component modes

- Почему это важно: Гарантирует, что все комбинации флагов рендерят валидные манифесты.
- **Given** chart в `deployments/helm/maskchain/`
- **When** выполняется `helm template --set gateway.enabled=true .` и `helm template --set admin.enabled=true .` и `helm template --set all.enabled=true .` и `helm template --set gateway.enabled=true,admin.enabled=true .`
- **Then** каждая команда возвращает валидный YAML (непустой stdout, no errors в stderr)
- Evidence: stdout содержит корректные Kind: Deployment, Service, ConfigMap, Secret

### AC-003 ConfigMap содержит все секции конфига

- Почему это важно: Без полного конфига приложение не стартует корректно.
- **Given** chart и values с заполненными log, server, database, valkey, mask, shield, routing, egress, session, admin, otel, ratelimit, analytics, dictionary_cache, tenants
- **When** выполняется `helm template .`
- **Then** в stdout ConfigMap содержит YAML-ключ `config.yaml` со всеми перечисленными секциями, включая tenant `default` с `api_keys`
- Evidence: grep перечисленных секций в выводе ConfigMap

### AC-004 Secret api-keys содержит env-переменные для подстановки `${}`

- Почему это важно: ConfigMap содержит `${VAR}` placeholders, без env-переменных приложение не найдёт ключи.
- **Given** values.apiKeys = { OPENAI_API_KEY: "sk-test", ADMIN_PASSWORD: "test" }
- **When** выполняется `helm template .`
- **Then** stdout содержит Kind: Secret с именем, содержащим `api-keys`, data закодированы в base64 для указанных ключей, и Deployment содержит `envFrom[0].secretRef.name` указывающий на этот Secret
- Evidence: grep Secret и Deployment в выводе `helm template`

### AC-005 ConfigMap не содержит plaintext secrets

- Почему это важно: Безопасность — пароли и ключи только в Secret.
- **Given** values, где apiKeys и credentials заданы в values.apiKeys (не в values.config напрямую)
- **When** выполняется `helm template .`
- **Then** ConfigMap.data["config.yaml"] не содержит значений паролей/ключей — только `${...}` placeholders
- Evidence: grep -oE 'password:|api_keys:|token:' в ConfigMap не находит значений (только YAML-ключи структуры) или значения являются `${...}`

### AC-006 helm install с минимальным values успешен

- Почему это важно: End-to-end проверка деплоя в кластер.
- **Given** Kubernetes cluster (kind/minikube) и `helm repo add bitnamilegacy ...`
- **When** выполняется `helm install maskchain-release deployments/helm/maskchain --set postgresql.auth.password=test,valkey.auth.password=test`
- **Then** через 120s `kubectl get pods -l app.kubernetes.io/instance=maskchain-release` показывает все pod'ы в Ready
- Evidence: вывод `kubectl get pods` + `kubectl get svc`

### AC-007 Ingress шаблон рендерится только при включении

- Почему это важно: Избежать лишних и сломанных Ingress-ресурсов.
- **Given** `helm template --set ingress.enabled=false .`
- **When** фильтрация stdout по Kind: Ingress
- **Then** результат пуст
- **Given** `helm template --set ingress.enabled=true .`
- **When** фильтрация stdout по Kind: Ingress
- **Then** Ingress присутствует с gateway host (и admin host если admin.enabled=true или all.enabled=true)

### AC-008 ServiceMonitor опционален

- Почему это важно: Не плодить CRD-ресурсы без оператора.
- **Given** `helm template .` (default values — servicemonitor.enabled=false)
- **When** фильтрация stdout по ServiceMonitor
- **Then** результат пуст
- **Given** `helm template --set servicemonitor.enabled=true .`
- **When** фильтрация stdout по ServiceMonitor
- **Then** ServiceMonitor присутствует с правильными labels и port name

### AC-009 NetworkPolicy ограничивает трафик

- Почему это важно: Безопасность сети — минимальные привилегии.
- **Given** `helm template --set networkPolicy.enabled=true .`
- **When** фильтрация stdout по Kind: NetworkPolicy
- **Then** NetworkPolicy разрешает ingress только от Ingress-контроллера и внутри namespace, egress только к PostgreSQL и Valkey

### AC-010 External PostgreSQL/Valkey

- Почему это важно: Пользователь может иметь managed PostgreSQL/Valkey (RDS, ElastiCache) и не хочет запускать subchart'ы.
- **Given** `helm template --set postgresql.external.enabled=true --set valkey.external.enabled=true --set postgresql.external.dsn=postgres://user:pass@external-pg:5432/db --set valkey.external.addr=external-valkey:6379 .`
- **When** фильтрация stdout по Kind: StatefulSet с метками postgresql/valkey
- **Then** StatefulSet для postgresql и valkey отсутствуют; ConfigMap содержит `${POSTGRES_DSN}` и `${VALKEY_ADDR}`, Secret содержит эти значения
- Evidence: `helm template` не содержит postgresql/valkey StatefulSet, ConfigMap имеет `${...}` placeholders, Secret имеет base64 DSN и addr

### AC-011 Gateway API HTTPRoute рендерится при включении

- Почему это важно: Пользователи с Gateway API не должны использовать Ingress.
- **Given** `helm template --set gatewayAPI.enabled=true --set gatewayAPI.gatewayName=my-gw --set gatewayAPI.hostname=api.example.com .`
- **When** фильтрация stdout по Kind: HTTPRoute
- **Then** HTTPRoute присутствует с parentRef на `my-gw`, hostname `api.example.com`, backendRef на gateway Service. При admin.enabled=true или all.enabled=true — также HTTPRoute для admin.
- **Given** `helm template --set gatewayAPI.enabled=false .`
- **When** фильтрация stdout по Kind: HTTPRoute
- **Then** HTTPRoute отсутствует
- Evidence: grep HTTPRoute в `helm template`

## Допущения

- Kubernetes кластер уже существует и настроен (не требуется установка kubelet).
- Helm 3+ установлен на машине DevOps.
- `helm repo add bitnamilegacy https://charts.bitnami.com/bitnami` выполнен до dependency update.
- Prometheus-operator CRD (ServiceMonitor) предустановлен администратором кластера, если ServiceMonitor включён.
- Ingress-контроллер (nginx/contour) предустановлен, если Ingress включён.
- Gateway API CRD (gateway.networking.k8s.io) предустановлены администратором кластера, если Gateway API включён.

## Критерии успеха

- SC-001 `helm lint` и `helm template` — zero errors, zero warnings для gateway + admin + all + gateway+admin комбинаций.
- SC-002 После `helm install` с минимальным values в kind/minikube — все поды Ready за <120s.

## Краевые случаи

- Пустой values: chart использует разумные defaults (пароли — generate random в Helm, но для POC можно фиксированные).
- Смена флагов после установки: `helm upgrade` корректно удаляет/добавляет Deployment/Service/Ingress при изменении gateway.enabled/admin.enabled/all.enabled.
- Отсутствие bitnamilegacy registry: инструкция по добавлению repo в комментариях values.yaml или README.
- PVC retention при uninstall: по умолчанию `helm uninstall` НЕ удаляет PVC (persistence.resourcePolicy=keep).
- Переход с gateway+admin на all: включить all.enabled=true, выключить gateway.enabled=false, admin.enabled=false.

## Открытые вопросы

1. Должен ли chart поддерживать установку только PostgreSQL+Valkey без MaskChain (для нужд других сервисов)? — Пока нет, но можно расширить позже.
