# Ollama Provider План

## Phase Contract

Inputs: spec `specs/active/ollama-provider/spec.md`, inspect `pass`.
Outputs: plan, data-model stub.
Stop if: нет.

## Цель

Добавить Ollama как провайдера LLM через OpenAI-совместимый REST API. Минимальные изменения — новый адаптер + одна строчка в фабрике + ослабление валидации `api_keys`. Без изменений в routing domain, egress, API handlers.

## MVP Slice

Адаптер OllamaClient + фабрика + тесты. AC-001–AC-005.

## First Validation Path

```shell
ollama run llama3.2 &
# в другом окне:
curl -X POST http://localhost:8080/api/v1/chat/completions \
  -d '{"model":"llama3.2","messages":[{"role":"user","content":"hi"}],"stream":false}'
# → 200 с ответом от llama3.2
```

## Scope

- Новый файл `src/internal/adapters/provider/ollama.go` — OllamaClient (Call + Stream)
- Регистрация `case "ollama"` в `src/internal/adapters/provider/factory.go`
- Ослабление валидации `validateProviderAuth` в `src/internal/infra/config/config.go` для `api_type=ollama`
- Тесты `src/internal/adapters/provider/ollama_test.go` с mock HTTP-сервером
- **Нетронуто**: routing domain, egress client, API handlers, middleware, data model

## Performance Budget

- none (overhead пренебрежим — один HTTP вызов через существующий egress.Client)

## Implementation Surfaces

- `src/internal/adapters/provider/ollama.go` — **новая**, адаптер
- `src/internal/adapters/provider/factory.go` — **существующая**, +1 case
- `src/internal/infra/config/config.go` — **существующая**, ослабить валидацию
- `src/internal/adapters/provider/ollama_test.go` — **новая**, тесты

## Bootstrapping Surfaces

- none — структура `adapters/provider/` уже существует

## Влияние на архитектуру

- Локальное: +1 провайдер в фабрике, 0 изменений в интерфейсах
- Конфигурация: `api_keys` перестаёт быть строго обязательным для всех провайдеров
- Обратная совместимость: существующие openai/anthropic провайдеры не меняются

## Acceptance Approach

- AC-001: конфиг + фабрика + валидация. Тест `TestOllamaClient_ValidConfig` + тест валидатора
- AC-002: Call с httptest. Тест `TestOllamaClient_Call`
- AC-003: Stream с httptest SSE. Тест `TestOllamaClient_Stream`
- AC-004: перехват запроса, проверка отсутствия auth-заголовков. Тест `TestOllamaClient_NoAuthHeaders`
- AC-005: закрытый порт → 503. Тест `TestOllamaClient_Unreachable`
- AC-006: manual — описание в README или CONTRIBUTING (post-MVP)

## Данные и контракты

- Data model не меняется — см. `data-model.md: no-change`
- ProviderConfig (BaseURL, APIType, APIKeys, AuthScheme...) уже покрывает Ollama
- Контракты API не меняются — OllamaClient реализует существующий `ports.ProviderClient`

## Стратегия реализации

- DEC-001 Тонкий адаптер поверх OpenAI-совместимого API
  Why: Ollama `/v1/chat/completions` идентичен OpenAI по формату запроса/ответа. Отдельный клиент с нуля избыточен — достаточно общей логики из `provider.go` (buildAuthHeader, mergeHeaders) + egress.Client
  Tradeoff: не поддерживаются Ollama-native поля (`options`, `keep_alive`) как first-class citizens — они передаются как есть через proxy body
  Affects: ollama.go, factory.go
  Validation: AC-002, AC-003 проходят с httptest

- DEC-002 Ослабление валидации api_keys
  Why: Ollama не требует API-ключа, но текущая валидация требует `len(APIKeys) > 0` для всех провайдеров
  Tradeoff: если в будущем появится провайдер, требующий api_key, но указанный с опечаткой, валидация не поймает. Сейчас это acceptable — ключи проверяются runtime при 401
  Affects: config.go validateProviderAuth
  Validation: AC-001 — конфиг без api_keys проходит валидацию

- DEC-003 Переиспользование egress.Client
  Why: существующий клиент уже поддерживает circuit breaker, retry, proxy, TLS, connection pooling
  Tradeoff: специфичные для Ollama таймауты придётся настраивать через ProviderConfig.Timeout
  Affects: ollama.go (принимает *egress.Client как dependency)
  Validation: AC-002, AC-005

## Incremental Delivery

### MVP (Первая ценность)

1. Конфиг: ослабить валидацию `api_keys` для ollama
2. Адаптер: `ollama.go` с Call/Stream
3. Фабрика: `case "ollama"`
4. Unit-тесты: AC-001–AC-005

### Итеративное расширение

- post-MVP: поддержка raw Ollama API `/api/generate`, `/api/chat` (native format)
- post-MVP: health check endpoint `/api/tags` для определения загруженных моделей
- post-MVP: авто-документация manual test procedure (AC-006)

## Порядок реализации

1. **Config** — валидация, чтобы тесты адаптера могли создавать конфиг без api_keys
2. **OllamaClient** — Call + Stream
3. **Factory** — регистрация
4. **Tests** — все 5 AC

Параллелизация: config не зависит от адаптера, но логичнее начать с него чтобы тесты проходили.

## Риски

- Изменение OpenAI-совместимости в Ollama: низкий — endpoint стабилен с v0.1.x, breaking change маловероятен. Mitigation: тесты с httptest не зависят от реального Ollama
- Валидация без api_keys может пропустить опечатки в api_keys для openai/anthropic: низкий — ключи проверяются runtime. Mitigation: добавить warning в лог, если api_keys пуст для non-ollama провайдера

## Rollout и compatibility

- Специальных rollout-действий не требуется
- Новый тип провайдера — обратно совместим
- Если Ollama не запущен — gateway возвращает 503 (существующая логика FallbackHandler)

## Проверка

- `go test ./internal/adapters/provider/ -run TestOllama -count=1` — все 5 AC
- `go build ./...` — сборка без ошибок
- `go vet ./...` — статический анализ
- Manual: `ollama run llama3.2` + curl (AC-006)

## Соответствие конституции

- нет конфликтов
