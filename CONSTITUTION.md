# Конституция проекта MaskChain

## Назначение

Построение production-grade платформы для маршрутизации, обеспечения безопасности (Content Shield) и управления AI-трафиком с управлением политиками безопасности на уровне тенантов.

Миссия — предоставить организациям единый gateway для AI-трафика с встроенным Content Shield (AI DLP): обнаружение и реагирование на PII, secrets, финансовые данные в промптах и ответах. Тенанты являются контейнером политик обнаружения: словари, PII-правила, препроцессоры конфигурируются непосредственно на тенанте.

## Ключевые принципы

### I. Content Shield — Core Domain

Content Shield (AI DLP) — основной домен системы, а не дополнительная функция. Gateway обязан перехватывать AI-запросы и ответы и анализировать их на наличие:
- PII (персональные данные: email, телефон, SSN, паспортные данные)
- Secrets (API keys, private keys, JWT, tokens)
- Финансовых данных (номера карт с Luhn, IBAN, SWIFT)
- Protected health information (PHI)

Действия при обнаружении: block, redact, mask, alert. Политики настраиваются через тенантов.

### II. Tenant-Driven Policy Management

Политики Content Shield управляются через тенантов — каждый тенант содержит словари (dictionaries), PII-правила (piiConfig) и препроцессоры. Тенанты хранятся в PostgreSQL, управляются через REST API и React UI. Профили справочников удалены; вся конфигурация политик инкапсулирована в тенанте.

### III. Infrastructure, Not Chatbot

Проект — инфраструктурный gateway, а не chatbot, prompt playground, AI IDE или low-code workflow. Фокус: networking, security, traffic management, policy enforcement, observability.

### IV. AI Traffic Is Network Traffic

AI-запросы обрабатываются как HTTP/gRPC трафик с пониманием AI-семантики: token economics, prompt semantics, provider health, inference latency, model capabilities, compliance.

### V. Runtime Before Platform

Runtime гейтвея должен существовать до Kubernetes-абстракций. Порядок разработки: Runtime → Routing → Shield → Policies → Egress → API → UI → Operator. Envoy-режим — PostMVP.

### VI. Native-Only Data Plane (MVP)

Текущий data plane — встроенный Go runtime (native). Один бинарник: Gin HTTP server + `http.Client` + egress dialers. Никаких external зависимостей для обработки запросов. Envoy-режим — PostMVP (не планируется до стабилизации native-режима).

### VII. Local Development Matters

Каждая major feature запускаема локально через Docker Compose. React UI — через dev-режим (Vite/HMR) или compose-сервис. Native-режим runtime — основной deployment для локальной разработки.

### VIII. Streaming — обязательное требование

SSE, chunk forwarding, streaming retries, cancellation propagation, low-latency token delivery. Streaming должен быть стабильным через retries, failover, observability.

### IX. Наблюдаемость обязательна

Каждый запрос должен быть наблюдаем: distributed traces, metrics, structured logs, token accounting, shield visibility, provider health. Никаких чёрных ящиков.

### X. Extensibility Over Hardcoding

Предпочтение: plugins, interfaces, adapters, declarative policies, расширяемые детекторы для Content Shield. Избегать: provider-specific hacks, giant monolith logic, tightly coupled integrations.

## Непересматриваемые правила

- Реализация `MUST` идти по активным spec/plan/tasks и оставаться в заявленном scope.
- Работа `MUST NOT` продолжаться из неоднозначных требований или placeholder-контента.
- Изменения публичного поведения `MUST` отражаться в spec/tasks до merge.
- Если реализация конфликтует с конституцией, сначала обновляется конституция.

## Ограничения

- Content Shield — обязательная возможность gateway, а не opt-in.
- Тенанты (словари, PII-правила, препроцессоры) хранятся в PostgreSQL; Valkey — только для кэширования.
- React UI — только для управления тенантами и просмотра логов/инцидентов; не является панелью управления AI-трафиком в реальном времени.
- Профили справочников удалены и не должны возвращаться; вся конфигурация политик — на уровне тенанта.
- Envoy-режим — PostMVP; native-режим единственный до стабилизации core-доменов.
- No chatbot UI, no prompt playground, no agent framework, no low-code platform.
- Система должна работать в enterprise-сетях с outbound proxy и air-gapped окружениями.
- Gateway и возможный будущий Operator — раздельные компоненты.
- Каждая major feature должна быть запускаема локально (Docker Compose).

## Технологический стек

- **Язык:** Go (backend)
- **Web framework:** Gin
- **Config:** viper + cobra
- **Архитектура:** DDD, Clean Architecture (ports/adapters)
- **UI:** React (TypeScript, Vite)
- **Data Plane:** Native (Go, in-process). Envoy — PostMVP.
- **Content Shield / AI DLP:** Microsoft Presidio (PII detection), custom patterns engine (secrets, API keys, financial data)
- **Observability:** OpenTelemetry, Prometheus, Grafana, Loki, Tempo
- **Persistence:** PostgreSQL (tenants, audit, incidents)
- **Cache:** Valkey (Redis-compatible)
- **Локальная разработка:** Docker Compose, mock-провайдеры

## Основная архитектура

```
Client → Gateway Runtime → Shield Engine → Routing → Egress → AI Providers
                            ↕
                   Tenant Repository (PG)
                            ↕
                    React UI (tenants, logs)
```

### Gateway Runtime
HTTP API, streaming, retries, failover, routing execution, observability. In-process Go: Gin HTTP server, `http.Client` с egress dialer wrappers, in-process SSE streaming. Single binary, zero external dependencies.

### Shield Engine
Content inspection pipeline: PII redaction, secrets detection, AI DLP. Проверяет входящие промпты и исходящие ответы. Управляется конфигурацией тенанта (словари, PII-правила) через Tenant Repository.

### Tenant Repository
Хранилище тенантов для Content Shield:
- Tenants: именованные контейнеры политик
- Dictionaries: списки сущностей для точного детектирования
- PII Config: regex-правила детекции PII, secrets, financial, PHI
- Preprocessors: CSV/JSON препроцессоры для структурированных данных
- Reactions: block, redact, mask, alert

### Routing Engine
Model selection, provider selection, fallback decisions, semantic routing.

### Egress Engine
Outbound proxy routing, retry orchestration, timeout management.

### Observability Layer
OpenTelemetry, Prometheus, structured logging, distributed tracing.

### React UI
Управление тенантами (словари, PII-правила), просмотр инцидентов Shield, логи аудита.

## Языковая политика

- Язык документации: русский
- Язык общения с агентом: русский
- Язык комментариев в коде: английский

## Процесс разработки

- Каждая фича ДОЛЖНА разрабатываться в отдельной git-ветке.
- Именование веток SHOULD следовать `feature/<slug>`.
- Реализация SHOULD начинаться с явной спецификации до начала кодинга.
- Планы и задачи SHOULD выводиться из актуальной спецификации и оставаться с ней согласованными.
- Реализация, спецификации, планы и задачи ДОЛЖНЫ соответствовать этой конституции.
- Если работа выявляет конфликт с этой конституцией, конституция ДОЛЖНА быть изменена до продолжения несовместимой реализации.

## Definition of Done

- Задача считается завершенной только при observable proof: измененные файлы, вывод целевых тестов или результат команды.
- Для нетривиальных правок обязательны traceability-маркеры:
  - код: `@sk-task <slug>#<TASK_ID>: <short> (<AC_ID>)`
  - тесты: `@sk-test <slug>#<TASK_ID>: <TestName> (<AC_ID>)`
  - если одну задачу подтверждают несколько тестов/кейсов, `@sk-test <slug>#<TASK_ID>` должен стоять на каждом таком тесте/кейсе, а не только на одном representative тесте.
- Правило размещения маркеров:
  - Маркер ВСЕГДА ставится **над объявлением** символа, которому принадлежит.
  - Если у символа есть GoDoc, маркер(ы) идут **первыми**, затем **пустая строка** (`//`), затем GoDoc, затем объявление.
  - Если GoDoc нет, маркер ставится непосредственно над объявлением.
  - Запрещено ставить trace-маркеры на уровень `package`, `import` или file-header comment.
  - Маркер всегда относится к нижестоящему символу; если маркеров несколько — все относятся к одному символу.
- Примеры размещения и стиля по языкам:
  - Go:
    ```go
    // @sk-task slug#T1: description (AC-001)
    //
    // FunctionDoc describes what this function does.
    func DoSomething() { ... }

    // @sk-task slug#T2: description (AC-002)
    type Config struct { ... }

    // @sk-test slug#T1: TestSomething (AC-001)
    //
    // TestSomething проверяет поведение X.
    func TestSomething(t *testing.T) { ... }
    ```
    Без GoDoc:
    ```go
    // @sk-task slug#T1: description (AC-001)
    func Short() { ... }
    ```
    Если несколько `Test...` проверяют одну задачу, `@sk-test` нужен на каждом таком тесте.
  - Python: `#` первой строкой внутри тела `def` / `async def` / `class` / `def test_*`; не в module docstring и не над `import`. Если одну задачу покрывают несколько test functions, маркер нужен внутри каждой из них.
  - JavaScript / TypeScript: `//` над `function`, `async function`, class method, `class`; для `test(...)`/`it(...)` — первой строкой внутри callback/body. Если кейсов несколько, маркер нужен в каждом `test/it`.
  - Shell / Bash: `#` над `function name()` или первой строкой именованного behavior/test block; не в file header только ради trace.
  - SQL / migrations: `--` или `/* */` непосредственно над `CREATE FUNCTION|PROCEDURE|TRIGGER|VIEW` или первой строкой явно именованного migration block; не в верхнем комментарии файла без привязки к изменению.
- Существующие trace-маркеры сохраняются; покрытие новой задачи добавляется доп. маркерами (без перезаписи).
- Если один метод/тест покрывает несколько задач, на нем одновременно остаются несколько маркеров.
- Перед archive в verify должна быть подтверждена покрываемость acceptance criteria.

## Политика Repository Map

- `REPOSITORY_MAP.md` — компактный индекс навигации по коду, а не процессный документ.
- Карта обновляется только при существенном изменении структуры/навигации кода.
- Обновление карты выполняется in-place с минимальным diff; неизменные секции не переписываются.
- Операционные/spec-артефакты исключаются из индексации согласно политике проекта.

## DDD и Clean Architecture

### Domain Boundaries

Core domains: shield (content security, PII detection, secrets detection, dictionaries, preprocessors), routing, providers, egress, streaming, observability, tenants. Профили справочников удалены; все политики конфигурируются на тенанте. Архитектура организуется вокруг доменов, а не вокруг провайдеров.

### Clean Architecture

Следовать ports/adapters, dependency inversion, явным границам, изолированной domain logic. Domain logic не должен зависеть от HTTP-фреймворков, БД, Redis, React. Всё это — за портами.

## Структура репозитория

```
/src
    cmd/
        gateway/      — entrypoint runtime гейтвея
    internal/
        domain/       — domain logic (бизнес-сущности, value objects, domain services)
        app/          — application layer (use cases, application services)
        ports/        — port interfaces (inbound/outbound)
        adapters/     — adapter implementations (providers, persistence, shield detectors)
        infra/        — infrastructure (config, logging, metrics, tracing)
        api/          — API handlers и middleware
    pkg/              — переиспользуемые публичные библиотеки

/ui                   — React frontend (Vite + TypeScript)

/specs
    active/           — активные спецификации
    archived/         — архивированные спецификации

/deployments
    docker-compose/   — локальные окружения
    kubernetes/       — манифесты (PostMVP)

/docs
    en/               — документация на английском
    ru/               — документация на русском
    architecture/     — архитектурные документы
    adr/              — Architecture Decision Records

/examples             — примеры использования
/bin                  — артефакты сборки
```

## Управление

- Эта конституция является авторитетным источником для проектных решений.
- Изменения архитектуры, спецификаций, планов и задач ДОЛЖНЫ соответствовать этим принципам.
- Если реализация конфликтует с конституцией, приоритет у конституции, пока она явно не изменена.
- Изменяйте этот файл patch-обновлениями, сохраняя обязательные секции и делая правила конкретными и проверяемыми.

## Метаданные конституции

- Version: 1.2.0
- Ratified: 2026-07-10
- Last Amended: 2026-07-20

## Последнее обновление

2026-07-10 — начальная версия конституции MaskChain. Фокус: Content Shield (AI DLP), профили справочников, native-only data plane, React UI.
2026-07-15 — v1.1.0: профили справочников удалены, политики конфигурируются на тенанте (словари, PII-правила, препроцессоры).
2026-07-20 — v1.2.0: GoDoc clean — правило расположения маркеров: `@sk-task` → пустая строка → GoDoc → объявление. Маркер всегда над объявлением, GoDoc отделён пустой строкой снизу.
