# Ollama Provider Задачи

## Phase Contract

Inputs: plan, data-model (no-change).
Outputs: исполнимые задачи с покрытием AC-001–AC-005.
Stop if: нет.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/infra/config/config.go` | T1.1 |
| `src/internal/adapters/provider/ollama.go` | T2.1 |
| `src/internal/adapters/provider/factory.go` | T2.2 |
| `src/internal/adapters/provider/ollama_test.go` | T3.1 |

## Implementation Context

- Цель MVP: адаптер OllamaClient + фабрика + тесты; локальная модель работает через стандартный proxy pipeline
- Инварианты:
  - Ollama предоставляет OpenAI-совместимый `/v1/chat/completions`
  - Никаких изменений в routing domain, egress, API handlers
  - `ProviderConfig` уже покрывает все поля (BaseURL, APIType, APIKeys, AuthScheme...)
- Ошибки/коды: недоступный Ollama → 503 NO_HEALTHY_PROVIDER
- Контракты/протокол: Call → POST `{baseURL}/v1/chat/completions`; Stream → SSE на том же endpoint
- Вне scope: raw `/api/generate`, `/api/chat`; health check; UI; Docker Compose
- Proof signals: `go test ./internal/adapters/provider/ -run TestOllama -count=1` pass; `curl` к реальному Ollama через gateway
- References: DEC-001 (thin adapter), DEC-002 (relaxed validation), DEC-003 (reuse egress)

## Фаза 1: Config

Цель: ослабить валидацию `api_keys` для `api_type=ollama`.

- [x] T1.1 Ослабить `validateProviderAuth` — разрешить пустой `api_keys` для `api_type: ollama`. Добавить warning в лог, если `api_keys` пуст для non-ollama провайдера.
  Touches: `src/internal/infra/config/config.go`
  AC: AC-001

## Фаза 2: MVP Slice

Цель: адаптер + фабрика — минимальная ценность end-to-end.

- [x] T2.1 Реализовать `OllamaClient` в `ollama.go` — структура с `baseURL`, `ec *egress.Client`; конструктор `newOllamaClient(cfg *config.ProviderConfig, ec *egress.Client)`; методы `Call` и `Stream` через `{baseURL}/v1/chat/completions`. Auth-заголовки не отправляются, если `api_keys` пуст.
  Touches: `src/internal/adapters/provider/ollama.go`
  AC: AC-002, AC-003, AC-004

- [x] T2.2 Добавить `case "ollama":` в `NewProviderClient` фабрики.
  Touches: `src/internal/adapters/provider/factory.go`
  AC: AC-001

## Фаза 3: Проверка

Цель: automated tests для всех AC.

- [x] T3.1 Написать тесты:
  - `TestOllamaClient_ValidConfig` — создание клиента через фабрику с `api_type:ollama` без `api_keys` (AC-001)
  - `TestOllamaClient_Call` — httptest-сервер, non-streaming запрос (AC-002)
  - `TestOllamaClient_Stream` — httptest-сервер SSE, streaming запрос (AC-003)
  - `TestOllamaClient_NoAuthHeaders` — перехват запроса, проверка отсутствия Authorization/X-API-Key (AC-004)
  - `TestOllamaClient_Unreachable` — закрытый порт → ошибка (AC-005)
  Использованы `@sk-test` маркеры над каждой test function.
  Touches: `src/internal/adapters/provider/ollama_test.go`
  AC: AC-001, AC-002, AC-003, AC-004, AC-005

- [x] T4.1 Выполнить verify: `go build ./...`, `go vet ./...`, `go test ./internal/adapters/provider/ -count=1 -run TestOllama`. Проверить `@sk-task`/`@sk-test` маркеры. Записать `verify.md`.
  Touches: `specs/active/ollama-provider/verify.md`
  AC: AC-001, AC-002, AC-003, AC-004, AC-005

## Покрытие критериев приемки

- AC-001 -> T1.1, T2.2, T3.1, T4.1
- AC-002 -> T2.1, T3.1, T4.1
- AC-003 -> T2.1, T3.1, T4.1
- AC-004 -> T2.1, T3.1, T4.1
- AC-005 -> T2.1, T3.1, T4.1
