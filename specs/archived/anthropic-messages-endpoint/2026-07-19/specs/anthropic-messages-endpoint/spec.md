# Anthropic Native Messages Endpoint

## Scope Snapshot

- In scope: gateway принимает `POST /api/v1/messages`, AnthropicClient форвардит Anthropic-формат без конвертации, существующие OpenAI SDK клиенты не ломаются.
- Out of scope: авто-детекция формата тела, поддержка других provider-native форматов на разных путях.

## Цель

Разработчики, использующие Anthropic SDK в своих проектах, могут направлять трафик через MaskChain без изменения кода клиента. SDK шлёт `POST /v1/messages` с Anthropic-форматом тела — gateway принимает этот путь, определяет провайдера по модели и форвардит запрос напрямую в Anthropic API, минуя OpenAI→Anthropic конвертацию.

## Основной сценарий

1. Клиент (Anthropic SDK) отправляет `POST /api/v1/messages` с телом `{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hello"}]}`
2. Gateway определяет провайдера через `RouteSelector.Select(model, tenantID)` — находит `api_type: anthropic`
3. `AnthropicClient` видит `Path: "/v1/messages"` и форвардит тело как есть в Anthropic API
4. Ответ от Anthropic возвращается клиенту без конвертации
5. Ошибка: если модель не найдена в routing rules — `400 NO_ROUTE`. Если провайдер недоступен — fallback или `502`.

## User Stories

- P1 Story: Как разработчик с Anthropic SDK, я хочу отправлять запросы через MaskChain без изменения кода или формата тела.

## MVP Slice

Один инкремент: новый роут + Path field + AnthropicClient native-mode.

## First Deployable Outcome

Gateway принимает `POST /api/v1/messages`, AnthropicClient форвардит тело как есть. Тесты проходят.

## Scope

- Новый роут `/api/v1/messages` в gateway server.go (плюс `/v1/messages` → 301 redirect)
- Поле `Path` в `ports.ProviderRequest`
- `provider_handler.go`: передаёт оригинальный path из запроса
- `AnthropicClient`: при `Path == "/v1/messages"` форвардит тело без конвертации, ответ возвращает без конвертации
- Shield middleware: корректно обрабатывает `/api/v1/messages`
- Usage middleware: трекинг запросов для нового path
- Все существующие сценарии (OpenAI SDK → `/api/v1/chat/completions`) остаются без изменений

## Контекст

- Текущий handler хардкодит `URL: "/v1/chat/completions"` в ProviderRequest — нужно передавать реальный path
- Shield middleware применяется на уровне роута — новый роут получит ту же цепочку middleware
- Routing уже работает по model name — новый path не требует изменений в selector/fallback

## Зависимости

- `none`

## Требования

- RQ-001 Gateway ДОЛЖЕН принимать `POST /api/v1/messages` с тем же middleware chain (auth, shield, usage), что и `/api/v1/chat/completions`
- RQ-002 `ProviderRequest` ДОЛЖЕН содержать поле `Path` с оригинальным path запроса (напр. `/v1/messages`)
- RQ-003 `AnthropicClient.Call` при `Path == "/v1/messages"` ДОЛЖЕН форвардить тело запроса как есть без конвертации формата
- RQ-004 `AnthropicClient.Stream` при `Path == "/v1/messages"` ДОЛЖЕН форвардить streaming запрос как есть
- RQ-005 `AnthropicClient` при `Path == "/v1/chat/completions"` ДОЛЖЕН продолжать конвертировать OpenAI→Anthropic (обратная совместимость)
- RQ-006 Существующие провайдеры (OpenAI, Gemini, Bedrock, Proxy) НЕ ДОЛЖНЫ изменять поведение

## Вне scope

- Авто-детекция формата тела (явный признак — path, не content inspection)
- Поддержка Anthropic-формата на `/api/v1/chat/completions` (reverse conversion)
- Поддержка native-форматов для других провайдеров (Gemini, Bedrock) на их paths
- Tool calling (function calling) — только chat completions и messages

## Критерии приемки

### AC-001 Новый роут `/api/v1/messages` зарегистрирован

- Почему важно: без роута Anthropic SDK получит 404
- **Given** gateway запущен
- **When** клиент шлёт `POST /api/v1/messages` с валидным телом
- **Then** запрос обрабатывается тем же middleware chain (auth → shield → usage → routing)
- Evidence: тест с httptest сервером проверяет 200/не-404; middleware логи показывают shield scan

### AC-002 AnthropicClient форвардит native-формат

- Почему важно: клиентский SDK ожидает Anthropic-формат ответа, а не OpenAI
- **Given** `AnthropicClient` получает `ProviderRequest` с `Path: "/v1/messages"` и телом в Anthropic-формате
- **When** вызывается `Call` или `Stream`
- **Then** тело отправляется в upstream без конвертации; ответ возвращается без конвертации
- Evidence: unit test с httptest сервером — upstream получает тело как есть; ответ клиента — Anthropic-формат

### AC-003 Путь передаётся от handler к клиенту

- Почему важно: `Path` — единственный сигнал для решения о конвертации
- **Given** handler получает запрос на `/api/v1/messages`
- **When** конструируется `ProviderRequest`
- **Then** `Path` содержит `/v1/messages`
- Evidence: unit test handler проверяет поле Path

### AC-004 Обратная совместимость OpenAI-пути

- Почему важно: существующие OpenAI SDK клиенты не должны сломаться
- **Given** клиент шлёт `POST /api/v1/chat/completions` с OpenAI-форматом
- **When** запрос обрабатывается
- **Then** поведение идентично до-фича состоянию (конвертация, ответ)
- Evidence: существующие тесты `TestAnthropicClient_Call` и `TestAnthropicClient_Stream` продолжают проходить

### AC-005 Другие провайдеры не меняют поведение

- Почему важно: изменения не должны затронуть OpenAIClient, GeminiClient, ProxyClient, BedrockClient
- **Given** любой не-Anthropic клиент получает ProviderRequest
- **When** поле Path установлено
- **Then** клиент игнорирует Path и работает как раньше
- Evidence: `go build ./...`; существующие тесты всех провайдеров проходят

### AC-006 Shield middleware работает на новом пути

- Почему важно: content shield — core domain, не должен быть обойдён
- **Given** запрос на `/api/v1/messages` с PII в сообщении
- **When** middleware chain обрабатывает запрос
- **Then** shield сканирует тело и применяет policy
- Evidence: integration-style тест shield middleware с новым path

## Допущения

- Anthropic SDK всегда шлёт на `/v1/messages` — это стабильный контракт API
- Shield middleware не зависит от path (работает на уровне gin context)
- Модель в теле Anthropic-запроса имеет тот же формат (`model` field), что и в OpenAI — selector может её распарсить

## Критерии успеха

- `none`

## Краевые случаи

- Пустой `model` в теле запроса → `400 BAD_REQUEST`
- Неизвестная модель (нет routing rule) → `400 NO_ROUTE`
- Shield блокирует запрос → `403` с описанием
- Streaming запрос на `/api/v1/messages` с Anthropic-форматом → корректный SSE ответ
- Запрос на `/api/v1/messages` без shield (disabled) → форвардится напрямую

## Открытые вопросы

- Нужен ли `/v1/messages` → 301 redirect (как для `/v1/chat/completions`)? — Да, для обратной совместимости с SDK, которые не используют `/api/v1/` префикс.
