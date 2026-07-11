# Shield Gateway Integration

## Scope Snapshot

- In scope: интеграция ShieldEngine в gateway request lifecycle — перехват входящих LLM-прокси-запросов, сканирование промпта через ShieldEngine, блокировка/редирекшн до отправки провайдеру, установка заголовков X-Shield-*.
- Out of scope: реализация самого LLM-прокси (роутинг к провайдерам, стриминг, fallback), панель логов инцидентов, post-response scan (отложен).

## Цель

Оператор AI-шлюза получает возможность автоматически сканировать входящие промпты на чувствительные данные через настроенные профили Shield до того, как запрос уйдёт к LLM-провайдеру. Успех фичи измеряется тем, что запрос с триггером critical-детектора получает HTTP 403 + X-Shield-Status: blocked, а чистый запрос проходит к провайдеру с X-Shield-Status: clean в response.

## Основной сценарий

1. Внешний клиент отправляет POST `/v1/chat/completions` с JSON body, содержащим `model` и `messages`.
2. Middleware `ShieldMiddleware` перехватывает запрос, читает body, резолвит профиль Shield по заголовку `X-Shield-Profile-Slug` (primary). Если заголовок отсутствует — fallback на `X-Tenant-ID` + `model`.
3. Middleware прогоняет текст промпта (`messages[].content`) через `ShieldEngine.Scan()`.
4. Если `ScanResult.Status == blocked` — возвращается HTTP 403 с JSON-ответом и заголовком `X-Shield-Status: blocked` и `X-Shield-Incident-ID`.
5. Если `status == clean` — запрос продолжается к провайдеру (через существующий handler/proxy), устанавливается `X-Shield-Status: clean`.
6. Если `status == suspicious` — поведение определяется конфигурацией: блокировать или пропустить с предупреждением.
7. Ошибки сканирования (таймаут, profile not found) возвращают HTTP 502/404 и `X-Shield-Status: error`.

## User Stories

- P1 (MVP): Оператор настраивает профиль для tenant, отправляет тестовый запрос с PII, получает блокировку 403 + заголовки. Чистый запрос проходит.
- P2: Оператор видит в логах все заблокированные запросы с указанием детектора и фрагмента.

## MVP Slice

Middleware + profile resolution + pre-request scan + блокировка + заголовки. Без post-response scan. AC-001, AC-002, AC-003, AC-004.

## First Deployable Outcome

POST `/v1/chat/completions` с `{"model":"gpt-4","messages":[{"role":"user","content":"my SSN is 123-45-6789"}]}` возвращает HTTP 403 с `X-Shield-Status: blocked`.

## Scope

- `ShieldMiddleware` как `gin.HandlerFunc` в `src/internal/api/middleware/`
- Профиль резолвится по заголовку `X-Shield-Profile-Slug` (primary); fallback — `X-Tenant-ID` + `model` из body
- Proxy handler для `/v1/chat/completions`, `/v1/completions` (заглушка, передающая управление после middleware)
- Headers: `X-Shield-Status`, `X-Shield-Incident-ID` в response
- Интеграционные тесты: in-memory server + mock provider backend
- Конфигурация: tenant mapping model→profile slug, action on suspicious (block/pass)

## Контекст

- ShieldEngine уже реализован и принимает `ScanRequest{Text, ProfileSlug}`
- ScanUseCase требует `ProfileRepository` и `TenantID`
- Middleware слой уже используется: RequestID, Logger, Recovery, CORS, ErrorHandler
- Gateway не имеет прокси-роутинга к LLM-провайдерам — фича создаёт первый прокси-endpoint как заглушку
- Profile slug может быть передан напрямую через заголовок `X-Shield-Profile-Slug`
- Tenant ID извлекается из заголовка `X-Tenant-ID`; если не указан — используется default tenant

## Зависимости

- Зависит от `ShieldEngine` из `50-shield-engine` (spec уже выполнен)
- Зависит от `ProfileRepository` для поиска профиля по tenant + model
- Внешних сервисов нет

## Требования

- RQ-001 Middleware ДОЛЖЕН перехватывать POST запросы на `/v1/chat/completions` до вызова handler провайдера.
- RQ-002 Middleware ДОЛЖЕН резолвить профиль Shield: primary — заголовок `X-Shield-Profile-Slug`, fallback — `X-Tenant-ID` + `model` из JSON body.
- RQ-003 Middleware ДОЛЖЕН извлекать текст промпта из `messages[].content` и передавать в `ShieldEngine.Scan()`.
- RQ-004 При `ScanResult.Status() == blocked` middleware ДОЛЖЕН возвращать HTTP 403 с JSON-телом `{"error":"request blocked by content shield","shield_status":"blocked","incident_id":"<uuid>"}`.
- RQ-005 При `ScanResult.Status() == error` middleware ДОЛЖЕН возвращать HTTP 502.
- RQ-006 При успешном проходе middleware ДОЛЖЕН устанавливать заголовки `X-Shield-Status: clean` и `X-Shield-Incident-ID: <uuid>`.
- RQ-007 Middleware ДОЛЖЕН логировать каждый scan результат (status, profile_slug, model, latency).
- RQ-008 Если профиль не найден или отключён — middleware ДОЛЖЕН возвращать HTTP 404 с `X-Shield-Status: error`.
- RQ-009 Post-response scan (сканирование ответа провайдера) — отложено, не входит в MVP.

## Вне scope

- Реализация LLM-прокси (роутинг, балансировка, стриминг)
- Post-response scan (может стать отдельной фичей)
- Dashboard/UI для просмотра shield-логов и инцидентов
- Masking/redaction в response (замена чувствительных данных в ответе)
- Кеширование результатов сканирования

## Критерии приемки

### AC-001 ShieldMiddleware перехватывает запрос и блокирует при critical детекции

- Почему это важно: основная функция shield — предотвратить утечку до отправки провайдеру
- **Given** запущенный gateway с настроенным профилем, содержащим critical-детектор
- **When** клиент отправляет POST `/v1/chat/completions` с JSON body, содержащим фрагмент, триггерящий critical-детектор
- **Then** HTTP response status code 403, `X-Shield-Status: blocked`, тело содержит `shield_status: "blocked"`
- Evidence: curl-запрос возвращает 403 с указанными заголовками

### AC-002 Чистый запрос проходит к провайдеру

- Почему это важно: легитимные запросы не должны блокироваться
- **Given** запущенный gateway с настроенным профилем
- **When** клиент отправляет POST `/v1/chat/completions` с безвредным промптом
- **Then** запрос достигает handler провайдера (или заглушки), response содержит `X-Shield-Status: clean`
- Evidence: mock провайдера получает запрос; response содержит заголовок clean

### AC-003 Профиль резолвится по X-Shield-Profile-Slug

- Почему это важно: клиент явно указывает, какой профиль применить, без привязки к tenant+model
- **Given** gateway с настроенным профилем `project-alpha`
- **When** клиент отправляет POST `/v1/chat/completions` с заголовком `X-Shield-Profile-Slug: project-alpha`
- **Then** для сканирования используется профиль `project-alpha`
- Evidence: в логах middleware указан `profile_slug: project-alpha`

### AC-004 Профиль не найден — ошибка 404

- Почему это важно: оператор должен знать, что политика не настроена
- **Given** gateway без профиля для данного tenant+model
- **When** клиент отправляет запрос
- **Then** HTTP 404 с `X-Shield-Status: error`
- Evidence: curl возвращает 404

### AC-005 Ошибка ShieldEngine — HTTP 502

- Почему это важно: graceful degradation при недоступности зависимостей
- **Given** ShieldEngine возвращает ошибку (напр., недоступен детектор)
- **When** клиент отправляет запрос
- **Then** HTTP 502 с `X-Shield-Status: error`
- Evidence: mock ShieldEngine возвращает ошибку; gateway отвечает 502

### AC-006 Заголовки X-Shield-* присутствуют в response

- Почему это важно: клиент и observability могут идентифицировать статус shield
- **Given** любой запрос к `/v1/chat/completions`
- **When** middleware отработал
- **Then** response содержит `X-Shield-Status` и `X-Shield-Incident-ID`
- Evidence: curl -v показывает оба заголовка для blocked и clean кейсов

### AC-007 Интеграционный тест: полный цикл запрос → shield → block/allow

- Почему это важно: гарантирует корректную работу всей цепочки
- **Given** тест с in-memory gin сервером, mock ShieldEngine и mock provider handler
- **When** отправляются blocked и clean запросы
- **Then** blocked возвращает 403, clean возвращает ответ от mock провайдера
- Evidence: go test проходит с проверкой status code и заголовков для обоих кейсов

### AC-008 Middleware логирует результат каждого сканирования

- Почему это важно: аудит и отладка инцидентов
- **Given** любой запрос к `/v1/chat/completions`
- **When** middleware завершает обработку
- **Then** в лог записаны поля: shield_status, profile_slug, model, latency_ms, incident_id
- Evidence: тест на logger spy проверяет наличие этих полей

## Допущения

- Profile slug передаётся через HTTP заголовок `X-Shield-Profile-Slug` (primary механизм)
- Tenant ID передаётся через HTTP заголовок `X-Tenant-ID`; при отсутствии используется значение `default`
- Default tenant существует и имеет настроенный профиль (или запрос вернёт 404)
- Fallback `X-Tenant-ID + model` используется только если `X-Shield-Profile-Slug` не указан
- Proxy handler для `/v1/chat/completions` — заглушка, возвращающая 200 OK; реальный прокси-роутинг реализуется отдельно
- Post-response scan отложен; spec описывает только pre-request scan
- Middleware работает синхронно; для высоконагруженных сценариев может потребоваться async scan в будущем

## Критерии успеха

- SC-001 Время выполнения middleware scan < 500ms p95 для typical промпта (<4K токенов)
- SC-002 Error rate shield middleware < 0.1% (исключая ожидаемые blocked-ответы)

## Краевые случаи

- Пустой `messages` массив: middleware пропускает без сканирования, X-Shield-Status: clean
- `Content-Type` не JSON: возвращать HTTP 415
- Body превышает лимит (напр., >1MB): возвращать HTTP 413
- Отсутствующий `model` в body: HTTP 400 с X-Shield-Status: error
- Несколько tenant с разными профилями на один model: резолвится корректный профиль
- Request body read — требуется buffering для повторного чтения (middleware consumes body)
- Профиль найден, но отключён (Enabled() == false): HTTP 404, X-Shield-Status: error

## Открытые вопросы

1. Post-response scan — включать в эту фичу как опциональный режим или выделить в отдельный spec? Решение: отложено, не входит в MVP.
none
