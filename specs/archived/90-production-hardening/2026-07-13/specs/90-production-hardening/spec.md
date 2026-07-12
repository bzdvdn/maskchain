# Production Hardening

## Scope Snapshot

- In scope: performance tuning, profiling infrastructure, connection pool tuning, load testing capability, security hardening checklist with CI automation, production docker-compose profile, and operational runbook.
- Out of scope: Kubernetes manifests, horizontal pod autoscaling, Envoy data plane migration, chaos engineering, penetration testing, SOC2/ISO certification.

## Цель

Команда эксплуатации и разработчики получают возможность профилировать, нагружать, диагностировать и безопасно запускать gateway в production-окружении. Успех измеряется наличием воспроизводимых артефактов: pprof-эндпоинты за auth, оптимизированные пулы соединений, load-test скрипт, security CI-проверки, production-ready docker-compose и runbook для startup/health-check/debug/recovery.

## Основной сценарий

1. Разработчик или ops-инженер разворачивает gateway в production-профиле docker-compose.
2. При необходимости профилирования — запрашивает pprof-эндпоинты через admin-аутентикацию на основном порту.
3. При подозрении на утечку соединений или таймауты — проверяет и корректирует PG и HTTP connection pool parameters через конфигурацию.
4. Перед релизом — прогоняет load-тестирование через предоставленный скрипт.
5. Security-проверки выполняются в CI: TLS-конфигурация, отсутствие захардкоженных секретов, audit целостности.
6. При инциденте — следует runbook: startup, health check, debug, recovery.

## User Stories

- P1 Story: Как ops-инженер, я хочу иметь pprof-эндпоинты за аутентикацией, чтобы профилировать production без риска утечки метаданных.
- P1 Story: Как разработчик, я хочу видеть в CI security-проверки (TLS, secrets, audit), чтобы не допускать регрессий безопасности.
- P2 Story: Как разработчик, я хочу запустить load-тест перед релизом, чтобы убедиться в отсутствии performance-регрессий.
- P2 Story: Как ops-инженер, я хочу иметь runbook для типовых операций (startup, health check, debug, recovery).

## MVP Slice

Connection pool tuning + pprof endpoints за admin auth + security checklist как документ. Эти три компонента дают немедленную ценность для эксплуатации.

## First Deployable Outcome

После первого implementation pass можно продемонстрировать:
- `/debug/pprof/` доступен с admin-токеном; можно снять профиль в production-сборке.
- docker-compose profile `production` поднимает gateway с production-параметрами пулов и health-check.
- CI прогоняет security-чеклист (TLS verify, secrets scan).
- Load-test скрипт воспроизводит базовый сценарий.

## Scope

- pprof endpoints за admin-аутентикацией на основном порту gateway, включаемые config-флагом `debug.enabled`
- Connection pool tuning: PostgreSQL (`MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime`) и HTTP transport (`MaxIdleConnsPerHost`, `IdleConnTimeout`, `DisableKeepAlives`)
- Логирование конфигурации пулов при старте
- Load testing script (Python): сценарий прокси-запроса `/v1/chat/completions` через routing proxy на mock-провайдер + конфигурация
- Security checklist как документ + CI-шаги (проверка TLS endpoints, сканирование секретов, audit конфигурации)
- Production docker-compose profile (resources limits, healthcheck, restart policy)
- Runbook: startup sequence, health-check endpoints, debug procedure, recovery steps
- Prometheus metrics для connection pool stats (pgx pool metrics)
- Graceful shutdown таймауты для production

## Контекст

- Gateway уже имеет базовые health-check и metrics эндпоинты (см. 61-observability)
- Пулы соединений PG и HTTP используют стандартные значения из библиотек; production может упираться в лимиты
- Security-аудит ранее не автоматизирован — только ad-hoc проверки
- Docker-compose production-профиля не существовало — только dev-профиль
- Production окружение предполагает outbound proxy и enterprise-сеть
- pprof должен быть доступен для отладки, но защищён от внешнего доступа
- Debug-режим конфигурируется через `debug.enabled` (bool) в конфиг-файле и/или env `MASKCHAIN_DEBUG_ENABLED`

## Зависимости

- `net/http/pprof` — стандартная библиотека Go
- Python 3 + `urllib` (stdlib) — для load testing
- Имеющиеся metrics-эндпоинты из 61-observability
- Имеющийся конфиг `src/internal/infra/config/` для параметров пулов

## Требования

- RQ-001 Gateway ДОЛЖЕН экспортировать pprof-эндпоинты на основном HTTP-порту, доступные только после admin-аутентикации.
- RQ-002 Gateway ДОЛЖЕН логировать актуальные параметры connection pool (PG + HTTP) при старте.
- RQ-003 Gateway ДОЛЖЕН предоставлять Prometheus-метрики состояния PG connection pool (acquire count, idle, in-use).
- RQ-004 CI ДОЛЖЕН выполнять security-скан: проверка TLS-конфигурации, поиск захардкоженных секретов, аудит конфигурационных файлов.
- RQ-005 Репозиторий ДОЛЖЕН содержать Python-скрипт базового load-теста с конфигурацией для локального запуска.
- RQ-006 Репозиторий ДОЛЖЕН содержать docker-compose профиль `production` с resource limits, healthcheck и restart policy.
- RQ-007 Репозиторий ДОЛЖЕН содержать runbook (MARKDOWN) для startup, health check, debug и recovery.

## Вне scope

- Kubernetes manifests, HPA, K8s probes — PostMVP
- Envoy data plane migration
- Chaos engineering / fault injection тесты
- Penetration testing (внешний)
- SOC2 / ISO 27001 сертификация и документация
- Автоматическое tuning пулов (adaptive pool sizing)
- Auth-прокси или WAF перед gateway
- Database migration для пулов (настройки не требуют миграций)
- Dashboard для pprof (только raw endpoints)

## Критерии приемки

### AC-001 Pprof endpoints доступны за admin auth

- Почему это важно: профилирование production без auth — уязвимость; без pprof — диагностика производительности вслепую.
- **Given** запущенный gateway с config-флагом `debug.enabled: true` (env: `MASKCHAIN_DEBUG_ENABLED=true`)
- **When** запрос `GET /debug/pprof/` выполняется с валидным admin-токеном
- **Then** возвращается `200 OK` и HTML-страница со списком профилей
- **When** тот же запрос выполняется без токена или с невалидным токеном
- **Then** возвращается `401 Unauthorized`
- Evidence: curl-запросы с/без токена показывают разный статус; HTTP-тест в CI проверяет оба сценария

### AC-002 Connection pool parameters логируются при старте

- Почему это важно: без лога пулов невозможно проверить, какие значения применяются в production.
- **Given** gateway сконфигурирован с `db.max_open_conns=50` и `http.max_idle_conns_per_host=10`
- **When** gateway запускается
- **Then** в логах на уровне INFO появляются строки, содержащие `max_open_conns=50` и `max_idle_conns_per_host=10`
- Evidence: запуск gateway и grep логов по ключевым параметрам

### AC-003 Prometheus метрики PG connection pool

- Почему это важно: мониторинг пулов позволяет обнаружить утечку соединений до деградации сервиса.
- **Given** запущенный gateway с включёнными метриками
- **When** выполняется `GET /metrics`
- **Then** ответ содержит метрики `pgx_pool_acquire_count`, `pgx_pool_idle_conns`, `pgx_pool_in_use_conns` (или аналогичные) с корректными значениями
- Evidence: curl `/metrics` и проверка наличия метрик; нагрузочный тест показывает изменение counters

### AC-004 CI выполняет security-скан

- Почему это важно: автоматические проверки предотвращают попадание уязвимостей в production.
- **Given** CI-пайплайн на PR или push в master
- **When** пайплайн выполняется
- **Then** в пайплайне присутствуют шаги: TLS config lint, secrets scan (trufflehog/gitleaks), config audit
- **When** в коде присутствует захардкоженный секрет (тестовый)
- **Then** соответствующий шаг завершается с ненулевым кодом
- Evidence: CI лог показывает все три шага; PR с тестовым секретом фейлится

### AC-005 Load-test скрипт воспроизводим

- Почему это важно: повторяемый load-тест — базовый инструмент для измерения performance-регрессий.
- **Given** Python 3 установлен, gateway запущен с mock-провайдером
- **When** выполняется `python3 ./deployments/loadtest/chat_completion.py`
- **Then** скрипт отправляет POST-запросы на `/v1/chat/completions` через routing proxy, получает ответы и выводит summary (RPS, p50/p95/p99 latency, error rate)
- Evidence: скрипт завершается exit 0 и печатает метрики

### AC-006 Production docker-compose профиль

- Почему это важно: production-профиль исключает ошибки конфигурации при деплое.
- **Given** docker-compose.yml с профилем `production`
- **When** выполняется `docker compose --profile production up -d`
- **Then** сервисы запускаются с resource limits (CPU/memory), healthcheck и restart policy `unless-stopped`
- Evidence: `docker inspect` показывает установленные limits, healthcheck и restart policy

### AC-007 Runbook существует и покрывает операции

- Почему это важно: runbook сокращает MTTR при инцидентах.
- **Given** репозиторий
- **When** выполняется поиск файла `deployments/runbook.md`
- **Then** файл содержит секции: startup sequence, health check endpoints, debug procedure (connection pool exhaustion, TLS handshake failure, provider timeout, startup crash), recovery steps
- Evidence: файл существует, все секции присутствуют и содержат команды/инструкции

## Допущения

- Админ-аутентикация уже реализована (или будет добавлена в рамках данной spec) — gateway может использовать shared secret или API key из конфига, без полноценной RBAC
- Для security-скана используется gitleaks/trufflehog — opensource, без лицензионных ограничений
- CI-пайплайн поддерживает кастомные шаги (Makefile targets)
- Production docker-compose запускается на single-host (не swarm/kubernetes)
- Load-тест не требует реальных LLM-провайдеров — достаточно mock-эндпоинта

## Критерии успеха

- SC-001 CI пайплайн выполняется не более 5 минут с учётом security-скана (без учёта load-теста)
- SC-002 Load-тест: p99 latency < 500ms при 50 RPS на mock-провайдере с response ~1KB
- SC-003 Graceful shutdown завершается за < 30 секунд при 10 активных streaming-соединениях

## Краевые случаи

- pprof endpoints отключены, если debug-режим выключен конфигом — возвращать 404
- Connection pool параметры с некорректными значениями (отрицательные, ноль) — валидация и fallback на значения по умолчанию с warning в лог
- Security-скан сработал false positive — возможность добавить исключение через config-файл
- docker-compose production профиль: если порты заняты — понятная ошибка в логе
- Load-test скрипт: если gateway не запущен — понятное сообщение об ошибке

## Открытые вопросы

- None
