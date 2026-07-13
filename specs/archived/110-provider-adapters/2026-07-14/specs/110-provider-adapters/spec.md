# Provider Adapters: реальные HTTP-клиенты для LLM-провайдеров

## Scope Snapshot

- In scope: реализация адаптеров-клиентов для OpenAI (и совместимых: DeepSeek, Mistral, Groq) и Anthropic Messages API, реализующих порт `ports.ProviderClient`, с фабрикой по `api_type` и преобразованием ошибок провайдера в стандартный формат.
- Out of scope: secrets management (шифрование/ротация ключей), health-check провайдеров, provider failover, провайдеры кроме OpenAI-compatible и Anthropic.

## Цель

Gateway должен общаться с реальными LLM-провайдерами, а не только со stub. Разработчик указывает `api_type` в конфигурации провайдера — система создаёт нужный адаптер, который преобразует внутренние запросы в формат провайдера и парсит ответы обратно в единый внутренний формат. Успех фичи: `curl` через gateway к любому из supported провайдеров возвращает real completion, а не заглушку.

## Основной сценарий

1. Оператор конфигурирует провайдера в YAML: `api_type: openai` (или `anthropic`), `base_url`, `api_key`, прочие параметры.
2. Gateway при старте читает конфиг, для каждого провайдера вызывает `NewProviderClient(cfg)`, которая возвращает адаптер по значению `api_type`.
3. Routing Engine вызывает `adapter.Call()` с универсальным `ProviderRequest`. Адаптер формирует HTTP-запрос в формате провайдера, шлёт через egress-клиент, парсит ответ в `ProviderResponse`.
4. Для стриминга routing engine вызывает `adapter.Stream()`. Адаптер открывает SSE-соединение (в формате провайдера), преобразует каждый chunk в `ProviderChunk` и отдаёт в канал.
5. При HTTP-ошибке провайдера адаптер преобразует тело ошибки в стандартный `ProviderResponse` с кодом и сообщением; при таймауте/сетевой ошибке возвращает ошибку Go.

## User Stories

none — grouping по историям не добавляет ясности; функциональность линейна: каждый адаптер — вариация одного паттерна.

## MVP Slice

OpenAI-compatible адаптер (`openai.go`): Call + Stream через `/v1/chat/completions`, Bearer auth, стандартный SSE. Закрывает AC-001, AC-002, AC-003, AC-005. Anthropic — второй приоритет (AC-004, AC-006).

## First Deployable Outcome

После первого implementation pass оператор может:
- указать `api_type: openai` для DeepSeek/Mistral/Groq с их `base_url` и получить работающий chat completion;
- увидеть в логах, что gateway использует реальный адаптер, а не stub.

## Scope

- Добавление полей `api_type` и `api_key` в `ProviderConfig` для выбора типа адаптера и аутентификации.
- `openai.go`: адаптер для OpenAI-compatible API (DeepSeek, Mistral, Groq) — Bearer auth header, `/v1/chat/completions`, стандартное SSE.
- `anthropic.go`: адаптер для Anthropic Messages API — `x-api-key` header, `/v1/messages`, свой формат стриминга.
- Фабричная функция `NewProviderClient(cfg)`, выбирающая адаптер по `api_type`.
- Преобразование ошибок провайдера (HTTP 4xx/5xx) в стандартный формат.
- Unit-тесты и/или интеграционные тесты для каждого адаптера.
- Пакет `src/internal/adapters/provider/`.

## Контекст

- Адаптеры — это тонкий слой преобразования форматов; реальный HTTP transport предоставляет egress-клиент (`adapters/egress/client.go`).
- Формат запроса/ответа провайдера должен быть приведён к OpenAI-совместимому внутреннему формату (единая модель для routing engine).
- Stream-формат различается: OpenAI — SSE с `data: ...` строками, Anthropic — SSE со своей структурой event.
- Конфигурация провайдера `ProviderConfig` не содержит `api_type` на момент фичи — поле будет добавлено.

## Зависимости

- `egress.Client` (реализован в 71-egress-streaming) — для выполнения HTTP-запросов и стриминга.
- `ports.ProviderClient` и `ports.ProviderChunk` (реализованы в 70-routing-engine, 71-egress-streaming).
- Внешних сервисных зависимостей нет; адаптеры обращаются к API провайдеров через egress.

## Требования

- RQ-001 Система ДОЛЖНА поддерживать `api_type: openai` и `api_type: anthropic` в конфигурации провайдера.
- RQ-002 Система ДОЛЖНА создавать нужный адаптер через фабрику по значению `api_type`.
- RQ-003 OpenAI-совместимый адаптер ДОЛЖЕН отправлять Bearer-токен в заголовке `Authorization`, беря значение из `cfg.api_key`.
- RQ-004 Anthropic адаптер ДОЛЖЕН отправлять API-ключ в заголовке `x-api-key`, беря значение из `cfg.api_key`.
- RQ-005 Каждый адаптер ДОЛЖЕН реализовывать Call (unary) и Stream (SSE).
- RQ-006 Каждый адаптер ДОЛЖЕН преобразовывать HTTP-ошибки провайдера (4xx/5xx) в `ProviderResponse` с кодом и телом ошибки; сетевые/таймаут-ошибки возвращать как Go-ошибку.

## Вне scope

- Secrets management (шифрование, ротация, Vault-интеграция) — отдельная фаза.
- Health-check провайдеров (простое /health или кастомный endpoint) — отдельная фаза.
- Provider failover / fallback при ошибке — фаза routing engine.
- Провайдеры Google Gemini, Cohere, AWS Bedrock — только если появятся в требованиях.
- Rate limiting и retry-логика на уровне адаптера (уже есть в egress).

## Критерии приемки

### AC-001 Фабрика создаёт адаптер по api_type

- Почему это важно: без фабрики routing engine не может динамически выбрать адаптер.
- **Given** конфиг провайдера с `api_type: openai`
- **When** вызвана `NewProviderClient(cfg)`
- **Then** возвращён `ProviderClient`, чья реализация — `*OpenAIClient`
- Evidence: type assertion `client.(*OpenAIClient)` успешен для `api_type: openai`; для `api_type: anthropic` — `client.(*AnthropicClient)`; для неизвестного `api_type` — ошибка.

### AC-002 OpenAI-адаптер: Call возвращает completion

- Почему это важно: базовый use-case chat completion через OpenAI-совместимый API.
- **Given** экземпляр `OpenAIClient` с `base_url: https://api.openai.com/v1` (или совместимого провайдера)
- **When** вызван `Call` с `ProviderRequest`, содержащим тело chat completion запроса
- **Then** возвращён `ProviderResponse` со статусом 200 и телом, содержащим `choices[0].message.content`
- Evidence: ответ парсится и содержит непустой контент; для stub-теста — замоканный HTTP-ответ.

### AC-003 OpenAI-адаптер: Stream возвращает SSE-чанки

- Почему это важно: streaming — ключевое requirement для LLM gateway.
- **Given** экземпляр `OpenAIClient`
- **When** вызван `Stream` с chat completion запросом
- **Then** возвращён канал `ProviderChunk`, по которому приходят `data:...` SSE-события, и финальный chunk с `Done: true`
- Evidence: канал закрывается после `Done`; каждый chunk содержит валидный Data с part of choice delta.

### AC-004 Anthropic-адаптер: Call возвращает completion через Messages API

- Почему это важно: Anthropic использует свой API-формат, несовместимый с OpenAI.
- **Given** экземпляр `AnthropicClient` с `base_url: https://api.anthropic.com/v1`
- **When** вызван `Call` с запросом в Anthropic-формате
- **Then** возвращён `ProviderResponse` со статусом 200 и телом, содержащим `content[0].text`
- Evidence: ответ парсится корректно; заголовок `x-api-key` присутствует в запросе.

### AC-005 Преобразование ошибок провайдера

- Почему это важно: gateway должен возвращать внятную ошибку, а не сырой HTTP-статус.
- **Given** адаптер (openai или anthropic)
- **When** провайдер возвращает HTTP 400/401/429/500 с JSON-телом ошибки
- **Then** адаптер возвращает `ProviderResponse` с соответствующим `StatusCode` и разобранным телом ошибки
- Evidence: структура ошибки содержит код и message из ответа провайдера.

### AC-006 Anthropic-адаптер: Stream возвращает SSE-чанки в Anthropic-формате

- Почему это важно: Anthropic использует event-based SSE с иной структурой, чем OpenAI.
- **Given** экземпляр `AnthropicClient`
- **When** вызван `Stream` с chat completion запросом
- **Then** возвращён канал `ProviderChunk`, по которому приходят event-ы (`event: content_block_delta`, `event: message_stop`), и финальный chunk с `Done: true`
- Evidence: канал закрывается после `Done`; ошибка парсинга неизвестного event-а не возникает.

### AC-007 Конфигурация: поле api_type добавлено в ProviderConfig

- Почему это важно: без поля в конфиге фабрика не может определить тип адаптера.
- **Given** YAML-конфиг с `routing.providers[0].api_type: openai`
- **When** конфиг загружен в `ProviderConfig`
- **Then** поле `APIType` заполнено значением `"openai"`
- Evidence: `cfg.Routing.Providers[0].APIType == "openai"` после Unmarshal.

### AC-008 Конфигурация: поле api_key добавлено в ProviderConfig

- Почему это важно: адаптер должен знать ключ для заголовка авторизации; ключ хранится в конфиге провайдера.
- **Given** YAML-конфиг с `routing.providers[0].api_key: sk-...`
- **When** конфиг загружен в `ProviderConfig`
- **Then** поле `APIKey` заполнено значением `"sk-..."`
- Evidence: `cfg.Routing.Providers[0].APIKey == "sk-..."` после Unmarshal.

## Допущения

- Поле `api_key` добавляется в `ProviderConfig` на этой фазе; фабрика передаёт его адаптеру при создании. Ключ хранится в открытом виде (YAML/ENV) — шифрование и ротация вне scope данной фазы.
- Ответы провайдеров соответствуют официальным спецификациям API на момент реализации.
- Все OpenAI-совместимые провайдеры (DeepSeek, Mistral, Groq) соблюдают контракт `/v1/chat/completions` со стандартным SSE.
- Egress-клиент уже корректно обрабатывает timeout, retry, proxy — адаптер не дублирует эту логику.

## Критерии успеха

- SC-001 Каждый адаптер покрыт unit-тестами с замоканным HTTP (Call + Stream).
- SC-002 Интеграционный тест (e2e с реальным или записанным ответом провайдера) проходит для openai и anthropic.

## Краевые случаи

- Неизвестный `api_type` в конфиге — фабрика возвращает ошибку.
- Пустой `base_url` — ошибка валидации при создании адаптера.
- Провайдер возвращает не-JSON тело ошибки — адаптер возвращает raw body как текст ошибки.
- SSE-соединение разрывается до получения `[DONE]` — канал закрывается с Err.
- Anthropic stream без `content_block_delta` (только `message_start`/`message_stop`) — канал должен корректно завершиться.

## Открытые вопросы

- Где разместить фабрику: новый файл `adapters/provider/factory.go` или в `main.go` (которого ещё нет)? Рекомендация: `factory.go` в пакете `provider`, вызываемый из инициализации приложения.
- Нужен ли `api_type: auto` (определение по base_url)? Пока нет — явное указание обязательное.
- Как быть с провайдерами, у которых часть OpenAI-совместима, а часть нет (Mistral имеет и свой API, и OpenAI-compatible)? Пока используем `api_type: openai` для OpenAI-совместимого режима.
