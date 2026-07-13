# Provider Adapters Задачи

## Phase Contract

Inputs: plan, data-model, spec.
Outputs: упорядоченные задачи с покрытием AC.
Stop if: AC нельзя привязать к задачам.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/infra/config/config.go` | T1.1 |
| `src/internal/infra/config/config_test.go` | T1.2 |
| `src/internal/adapters/provider/provider.go` | T2.1 |
| `src/internal/adapters/provider/factory.go` | T2.2 |
| `src/internal/adapters/provider/openai.go` | T3.1 |
| `src/internal/adapters/provider/provider_test.go` | T3.2, T4.2, T5.1 |
| `src/internal/adapters/provider/anthropic.go` | T4.1 |

## Implementation Context

- **Цель MVP:** openai.go + factory + config (api_type/api_key) — gateway может общаться с OpenAI-compatible провайдерами.
- **Инварианты:**
  - Адаптер получает api_key при создании из конфига, сам проставляет заголовок (Bearer для OpenAI, x-api-key для Anthropic).
  - HTTP transport делегируется `egress.Client` — адаптер только трансформирует запрос/ответ.
  - Неизвестный `api_type` → фабрика возвращает ошибку.
- **Ошибки:** общий тип `ProviderError{StatusCode, Type, Message}` — сериализуется в JSON в `ProviderResponse.Body` при HTTP 4xx/5xx.
- **Контракты:**
  - OpenAI-compatible: POST `/v1/chat/completions`, Bearer auth, SSE с `data: ...`.
  - Anthropic: POST `/v1/messages`, x-api-key auth, event-based SSE (`event: content_block_delta`, `event: message_stop`).
- **Границы scope:** не делаем health-check, failover, secrets management, провайдеры кроме OpenAI-compatible и Anthropic.
- **Proof signals:**
  - `go test ./src/internal/adapters/provider/... -v` проходит.
  - `go test ./src/internal/infra/config/... -v` проходит.
  - `go build ./...` успешен.
- **References:** DEC-001 (factory location), DEC-002 (key ownership), DEC-003 (ProviderError), DM-001 (ProviderConfig fields), DM-002 (ProviderError).

## Фаза 1: Конфигурация провайдера

Цель: добавить поля api_type и api_key в ProviderConfig для выбора адаптера и аутентификации.

- [x] T1.1 Добавить поля APIType и APIKey в структуру ProviderConfig (config.go). APIType — required, APIKey — optional. Touches: `src/internal/infra/config/config.go`
- [x] T1.2 Добавить тесты unmarshal для новых полей: YAML с api_type: openai / api_type: anthropic / пустой api_type. Touches: `src/internal/infra/config/config_test.go`

## Фаза 2: Общие типы и фабрика

Цель: создать ProviderError и фабрику NewProviderClient, от которой зависят адаптеры.

- [x] T2.1 Создать тип ProviderError{StatusCode int, Type string, Message string} в provider.go. Функция ParseProviderError(body []byte, apiType string) для парсинга ошибок OpenAI и Anthropic форматов. Touches: `src/internal/adapters/provider/provider.go`
- [x] T2.2 Реализовать NewProviderClient(cfg) в factory.go: switch по cfg.APIType, возвращает *OpenAIClient / *AnthropicClient / ошибку для неизвестного типа. Touches: `src/internal/adapters/provider/factory.go`

## Фаза 3: OpenAI-compatible адаптер (MVP)

Цель: реализовать openai.go — Call + Stream, Bearer auth, SSE.

- [x] T3.1 Реализовать OpenAIClient: конструктор с baseURL и apiKey, Call (POST /v1/chat/completions, Bearer auth, парсинг ответа в choices), Stream (SSE парсинг data: строк в ProviderChunk). Использует egress.Client для HTTP. Touches: `src/internal/adapters/provider/openai.go`
- [x] T3.2 Unit-тесты OpenAIClient: Call с mock HTTP (200 OK + choices, 400 ошибка), Stream с mock SSE (data: строки + [DONE]). Touches: `src/internal/adapters/provider/provider_test.go`

## Фаза 4: Anthropic адаптер

Цель: реализовать anthropic.go — Call + Stream, x-api-key auth, event-based SSE.

- [x] T4.1 Реализовать AnthropicClient: конструктор, Call (POST /v1/messages, x-api-key, парсинг content[0].text), Stream (парсинг event: content_block_delta / message_stop). Touches: `src/internal/adapters/provider/anthropic.go`
- [x] T4.2 Unit-тесты AnthropicClient: Call с mock HTTP, Stream с mock event SSE. Touches: `src/internal/adapters/provider/provider_test.go`

## Фаза 5: Проверка

Цель: финальная верификация — все AC покрыты, сборка чиста.

- [x] T5.1 Добавить сквозной verify-тест: фабрика + каждый адаптер через mock egress. Проверить AC-001 (type assertion), AC-005 (error transform), AC-002/AC-004 (response body). Touches: `src/internal/adapters/provider/provider_test.go`

## Покрытие критериев приемки

- AC-001 -> T2.2, T5.1
- AC-002 -> T3.1, T3.2
- AC-003 -> T3.1, T3.2
- AC-004 -> T4.1, T4.2
- AC-005 -> T2.1, T3.2, T4.2, T5.1
- AC-006 -> T4.1, T4.2
- AC-007 -> T1.1, T1.2
- AC-008 -> T1.1, T1.2

## Заметки

- T2.1 (ProviderError) и T1.1 (config поля) независимы — можно параллелить.
- T2.2 (фабрика) зависит от T2.1 (ProviderError для парсинга ошибок).
- T3.x (OpenAI) и T4.x (Anthropic) независимы — можно параллелить после T2.2.
- Trace-маркеры `@sk-task` / `@sk-test` добавляются на implement фазе.
