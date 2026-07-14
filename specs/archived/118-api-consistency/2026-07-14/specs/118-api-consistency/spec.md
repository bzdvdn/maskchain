# API Consistency — единый /api/v1/ стандарт, OpenAPI, Swagger UI, единый envelope

## Scope Snapshot

- In scope: унификация всех HTTP API маршрутов под префикс `/api/v1/`, единый JSON envelope для ответов и ошибок, единый формат пагинации, OpenAPI 3.1 спецификация, Swagger UI, SPA NoRoute обработчик.
- Out of scope: health/readiness/liveness/metrics endpoints, pprof, SSE streaming формат, аутентификация/авторизация, rate limiting.

## Цель

Разработчик и оператор MaskChain получают предсказуемый, единообразный API: все бизнес-эндпоинты доступны под `/api/v1/`, каждый JSON ответ обёрнут в стандартный envelope `{"data": ..., "error": ...}` со структурированными ошибками, пагинированные списки следуют формату `{"data": ..., "pagination": {...}}`, а спецификация описана в OpenAPI 3.1 и доступна через Swagger UI. Успех фичи будет виден по тому, что любой клиент может взять `docs/openapi.yaml`, сгенерировать клиент и корректно обработать все ответы без ручного разбора исключений.

## Основной сценарий

1. Существующие маршруты `/api/v1/profiles/*`, `/api/v1/incidents/*`, `/api/v1/tenants/*`, `/api/v1/shield/*` сохраняют свои URL.
2. Маршруты `/v1/chat/completions` и `/v1/completions` становятся доступны как `/api/v1/chat/completions` и `/api/v1/completions`; старые пути возвращают HTTP 301.
3. Каждый JSON ответ (200, 201, 4xx, 5xx) оборачивается в единый envelope: успех — `{"data": ..., "error": null}`, ошибка — `{"data": null, "error": {"code": "...", "message": "..."}}`.
4. Пагинированные списки возвращают `{"data": [...], "pagination": {"page": 1, "per_page": 20, "total": 100}, "error": null}`.
5. MaskHandler переходит с text/plain на JSON envelope.
6. Разработчик открывает `GET /api/v1/docs` в admin binary и видит интерактивную Swagger UI.
7. SPA-запросы с `Accept: text/html` (кроме известных API-путей) обслуживаются NoRoute-обработчиком, возвращающим `index.html`.
8. OpenAPI 3.1 файл `docs/openapi.yaml` описывает все публичные эндпоинты.

## User Stories

- P1 Story: Как разработчик интеграции, я хочу единый префикс `/api/v1/` для всех API-вызовов и единый формат ответа, чтобы не писать разный парсинг для каждого эндпоинта.
- P1 Story: Как разработчик платформы, я хочу OpenAPI 3.1 спецификацию и Swagger UI, чтобы изучать и тестировать API без чтения исходного кода.
- P2 Story: Как администратор, я хочу, чтобы браузерные переходы на неизвестные пути внутри SPA не приводили к 404, а корректно загружали приложение.

## MVP Slice

- Единый префикс `/api/v1/` и redirect 301 со старых `/v1/` путей.
- Единый JSON envelope для успешных ответов и ошибок на всех JSON-эндпоинтах.
- OpenAPI 3.1 спецификация с описанием хотя бы одного эндпоинта.
- AC-001, AC-002, AC-003, AC-006.

## First Deployable Outcome

После первого implementation pass можно выполнить `curl /api/v1/chat/completions` и получить ответ в едином envelope, выполнить `curl /v1/chat/completions` и получить 301 redirect, открыть `GET /api/v1/docs` и увидеть Swagger UI, а также запросить несуществующий HTML-путь в admin и получить SPA-страницу (не 404 JSON).

## Scope

- Все HTTP API маршруты gateway и admin binary: профили, инциденты, тенанты, shield, chat/completions, completions.
- Единый JSON envelope для ответов 2xx, 4xx, 5xx.
- Единый формат пагинации (замена существующего `PaginatedResponse`).
- OpenAPI 3.1 файл `docs/openapi.yaml`.
- Swagger UI endpoint `GET /api/v1/docs` в admin binary.
- NoRoute SPA обработчик с проверкой `Accept: text/html` для admin binary.
- Маппинг `/v1/` → `/api/v1/` (301 redirect) в gateway binary.
- Переход MaskHandler с text/plain на JSON envelope.

## Контекст

- Существующие клиенты UI (React) уже используют `/api/v1/profiles/`, `/api/v1/incidents/` — эти маршруты не меняются.
- Gateway binary содержит прокси-маршруты на `/v1/` без `/api` префикса — их нужно продублировать с redirect.
- Admin binary уже имеет NoRoute для статики, но без проверки `Accept: text/html`.
- `docs/` директория существует и пуста — в неё будет записан OpenAPI файл.
- OpenAPI 3.1 выбран в соответствии с современным стандартом (поддержка JSON Schema draft-2020-12).

## Зависимости

- Swagger UI dist-файлы (HTML, JS, CSS) для встраивания в Go binary через `//go:embed`.
- Все потребители `dto.PaginatedResponse` (profile, incident handlers) должны быть обновлены синхронно при смене формата.
- none

## Требования

- RQ-001 Все бизнес-эндпоинты ДОЛЖНЫ быть доступны под префиксом `/api/v1/`.
- RQ-002 Старые пути `/v1/chat/completions` и `/v1/completions` ДОЛЖНЫ возвращать HTTP 301 на `/api/v1/chat/completions` и `/api/v1/completions` соответственно.
- RQ-003 Каждый JSON ответ (2xx, 4xx, 5xx) ДОЛЖЕН использовать единый envelope: успех — `{"data": <payload>, "error": null}`, ошибка — `{"data": null, "error": {"code": "<code>", "message": "<message>"}}`.
- RQ-004 Пагинированные списки ДОЛЖНЫ использовать формат `{"data": [...], "pagination": {"page": <int>, "per_page": <int>, "total": <int>}, "error": null}`.
- RQ-005 MaskHandler (POST /api/v1/shield/mask, /unmask) ДОЛЖЕН возвращать JSON envelope вместо text/plain.
- RQ-006 OpenAPI 3.1 спецификация ДОЛЖНА быть записана в `docs/openapi.yaml` и покрывать все публичные эндпоинты.
- RQ-007 Admin binary ДОЛЖЕН обслуживать Swagger UI по `GET /api/v1/docs`.
- RQ-008 Admin binary ДОЛЖЕН иметь NoRoute обработчик, возвращающий `index.html` для запросов с `Accept: text/html` (SPA fallback).
- RQ-009 HTTP 204 (No Content) и не-JSON ответы (CSV, SSE) НЕ ДОЛЖНЫ оборачиваться в envelope.
- RQ-010 Swagger UI ДОЛЖЕН быть встроен в admin binary через `//go:embed` (CDN или внешний контейнер не используются).

## Вне scope

- Health/Liveness/Readiness (`/health`, `/ready`, `/live`) и `/metrics` — остаются на корневых путях без изменения формата.
- Debug/pprof маршруты — не меняются.
- SSE streaming формат данных — меняется только мета-информация ответа, содержимое `data: ...` строк остаётся неизменным.
- Аутентификация, авторизация, API key management.
- Rate limiting и token budget.
- Изменение backend-логики (domain, app, repository слои) — только API слой.
- Миграция React UI на новый envelope — адаптируется отдельной задачей.
- Gateway binary SPA (gateway не обслуживает UI).

## Критерии приемки

### AC-001 Все маршруты доступны под /api/v1/

- Почему это важно: единый префикс упрощает конфигурацию reverse proxy, API gateway и клиентских SDK.
- **Given** запущенный gateway binary
- **When** выполняются запросы к каждому существующему эндпоинту через `/api/v1/` префикс
- **Then** каждый запрос возвращает HTTP 200, 201 или 400+ (не 404)
- Evidence: curl-скрипт, проходящий по списку эндпоинтов, не получает ни одного 404.

### AC-002 Redirect /v1/ → /api/v1/

- Почему это важно: существующие клиенты, использующие `/v1/chat/completions`, продолжают работать без немедленного обновления.
- **Given** запущенный gateway binary
- **When** клиент отправляет POST /v1/chat/completions (или /v1/completions)
- **Then** ответ содержит HTTP 301 и заголовок `Location: /api/v1/chat/completions` (или `/api/v1/completions`)
- Evidence: `curl -v /v1/chat/completions` показывает 301 и правильный Location.

### AC-003 Единый JSON envelope для успешных ответов

- Почему это важно: клиент может парсить любой ответ по одному шаблону, не заботясь об исключениях.
- **Given** запущенный gateway/admin binary
- **When** клиент отправляет запрос к любому бизнес-эндпоинту под `/api/v1/`, возвращающему 200/201
- **Then** тело ответа содержит `{"data": <payload>, "error": null}`
- Evidence: `curl /api/v1/profiles | jq` показывает ровно два ключа верхнего уровня: `data` и `error`.

### AC-004 Структурированный ответ ошибки

- Почему это важно: клиент может программно обрабатывать ошибки по коду, а не парсить текстовое сообщение.
- **Given** запущенный gateway/admin binary
- **When** запрос приводит к 4xx или 5xx ошибке
- **Then** тело ответа содержит `{"data": null, "error": {"code": "<code>", "message": "<message>"}}`
- Evidence: `curl /api/v1/profiles/nonexistent | jq` показывает `error.code` = `NOT_FOUND`.

### AC-005 Единый формат пагинации

- Почему это важно: клиент обрабатывает постраничную навигацию единообразно на всех списковых эндпоинтах.
- **Given** запущенный gateway/admin binary с данными
- **When** клиент запрашивает список с `?page=1&per_page=20`
- **Then** ответ содержит `{"data": [...], "pagination": {"page": 1, "per_page": 20, "total": <int>}, "error": null}`
- Evidence: `curl /api/v1/profiles?page=1&per_page=20 | jq .pagination` показывает `page`, `per_page`, `total`; `jq .error` равен `null`.

### AC-006 MaskHandler возвращает JSON envelope

- Почему это важно: единый формат ответа — клиенту не нужен специальный парсер для shield-эндпоинтов.
- **Given** запущенный gateway binary
- **When** клиент отправляет POST /api/v1/shield/mask с текстом
- **Then** ответ — JSON с `{"data": {"masked_text": "...", "mask_id": "..."}, "error": null}`
- Evidence: `curl -X POST /api/v1/shield/mask -d 'text' | jq` содержит `data.masked_text`.

### AC-007 OpenAPI 3.1 спецификация

- Почему это важно: документация API всегда актуальна и может быть использована для генерации клиента.
- **Given** репозиторий MaskChain
- **When** проверяется файл `docs/openapi.yaml`
- **Then** файл существует, валиден по схеме OpenAPI 3.1, описывает не менее одного эндпоинта
- Evidence: `openapi validate docs/openapi.yaml` (или эквивалентная команда) завершается успешно.

### AC-008 Swagger UI доступен

- Почему это важно: разработчик может интерактивно изучать и тестировать API без отдельного инструмента.
- **Given** запущенный admin binary
- **When** клиент открывает `GET /api/v1/docs`
- **Then** ответ — HTML страница Swagger UI, загружающая `openapi.yaml`
- Evidence: браузер или `curl /api/v1/docs` возвращает HTML, содержащий `swagger-ui` или `#swagger-ui`.

### AC-009 NoRoute SPA fallback

- Почему это важно: прямой ввод URL в адресную строку SPA не приводит к 404.
- **Given** запущенный admin binary
- **When** клиент с заголовком `Accept: text/html` запрашивает несуществующий путь (не начинающийся с `/api/`)
- **Then** сервер возвращает `index.html` (HTTP 200)
- Evidence: `curl -H 'Accept: text/html' /some/unknown/path` возвращает HTML (содержит `<div id="root">` или аналогичный маркер SPA).

### AC-010 204 No Content и не-JSON ответы не обёрнуты

- Почему это важно: удаление (DELETE 204) и экспорт (CSV) не должны ломаться из-за envelope.
- **Given** запущенный admin binary
- **When** DELETE /api/v1/profiles/{slug} выполняется успешно
- **Then** ответ — HTTP 204 без тела
- **Given** запущенный gateway/admin binary
- **When** GET /api/v1/incidents/export?format=csv
- **Then** ответ — Content-Type: text/csv (не JSON envelope)
- Evidence: `curl -X DELETE /api/v1/profiles/{slug} -w '%{http_code}'` возвращает 204 с пустым телом; `curl /api/v1/incidents/export?format=csv -w '%{content_type}'` содержит `text/csv`.

## Допущения

- Health/Liveness/Readiness endpoints остаются на корневых путях — они не являются бизнес-API и используются infra probe.
- Существующие клиенты (React UI) продолжают использовать `/api/v1/`; адаптация формата ответа на стороне UI — отдельная задача.
- Все изменения касаются только API слоя (`src/internal/api/`), domain/app слои не затрагиваются.
- Для обратной совместимости сервер принимает query params пагинации как `page_size`, так и `per_page`; ответ всегда использует `per_page`.
- `/metrics` остаётся на `/metrics` как стандартный путь Prometheus scraping.

## Критерии успеха

- SC-001 Все существующие модульные и интеграционные тесты API проходят после миграции (регрессия отсутствует).
- SC-002 OpenAPI спецификация проходит валидацию без ошибок.

## Краевые случаи

- Запрос к `/api/v1/` (без path) — должен возвращать 404 (нет корневого эндпоинта).
- `Accept: text/html` на реальный API-путь — должен возвращать JSON (Content-Type ответа имеет приоритет).
- Неизвестный путь, начинающийся с `/api/`, с `Accept: text/html` — должен возвращать JSON 404, а не SPA fallback.
- DELETE 204 не должен иметь Content-Type application/json.
- Экспорт CSV с ошибкой (неверный tenant) — должен возвращать JSON ошибку, а не CSV.
- MaskHandler с невалидным телом — JSON ошибка, не text/plain.
- Запрос к `/v1/chat/completions` без `Content-Type: application/json` — 400 ошибка в едином envelope.

## Открытые вопросы

- Должны ли health endpoints (/health, /ready, /live) тоже получить /api/v1/ префикс или остаться на корне? Текущее допущение — остаются на корне.
- Должны ли SSE data-строки оборачиваться в envelope? Текущее допущение — нет, только HTTP-ответ целиком.

