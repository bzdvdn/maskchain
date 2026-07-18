# provider-adapters-expansion — План

## Phase Contract

Inputs: spec provider-adapters-expansion, repo context.
Outputs: plan, data-model.
Stop if: нет.

## Цель

Добавить три новых `api_type` в фабрику провайдеров: `gemini` (Google), `bedrock` (AWS), `proxy` (generic OpenAI-compatible). Каждый — отдельный файл адаптера, реализующий существующий `ProviderClient` порт. Никаких изменений в routing, shield, analytics — только новые листья в `factory.go`.

## MVP Slice

**MVP = Gemini + Proxy.** Покрывает ~95% рынка по числу провайдеров. Bedrock — отдельный инкремент из-за AWS SDK зависимости.

MVP AC: AC-001, AC-002, AC-003, AC-007, AC-008, AC-009.
Инкремент 2 AC: AC-004, AC-005, AC-006, AC-008, AC-009.

## First Validation Path

```bash
# 1. Build
go build ./src/...

# 2. Test
go test ./src/internal/adapters/provider/...

# 3. Manual: запустить gateway с config.yaml, где провайдеры gemini + groq (proxy)
#    и проверить /v1/chat/completions с моделью, которая роутится на них
```

## Scope

- Gemini adapter: конвертация OpenAI-формата ↔ Gemini-формат, SSE streaming
- Proxy adapter: форвард тела + auth injection, SSE passthrough
- Bedrock adapter: AWS SigV4, InvokeModel / InvokeModelWithResponseStream
- Factory: три новых case в switch
- Config: поля aws_region/aws_access_key_id/aws_secret_access_key в ProviderConfig
- Тесты: unit-тесты каждого адаптера с httptest/mock

**Вне scope:**
- Tool/function calling — только chat completions
- Multimodal (images, audio) — только text
- Azure OpenAI Entra ID auth — отдельная фича
- Batch inference

## Performance Budget

`none` — адаптеры выполняют HTTP-запросы, overhead конвертации формата < 1ms.

## Implementation Surfaces

| Surface | Почему меняется | Тип |
|---------|----------------|-----|
| `src/internal/adapters/provider/gemini.go` | Новый Gemini adapter | новый |
| `src/internal/adapters/provider/bedrock.go` | Новый Bedrock adapter | новый |
| `src/internal/adapters/provider/proxy.go` | Новый Proxy adapter | новый |
| `src/internal/adapters/provider/factory.go` | +3 case в switch | существующая |
| `src/internal/infra/config/config.go` | +3 поля в ProviderConfig | существующая |
| `src/internal/adapters/provider/provider.go` | Общие хелперы (buildAuthHeader, ParseProviderError) | существующая (без изменений) |
| `src/internal/ports/provider.go` | Не меняется | — |

## Bootstrapping Surfaces

`none` — структура репозитория уже есть (рядом с openai.go, anthropic.go).

## Влияние на архитектуру

**Локальное.** Новые файлы в существующем пакете `provider`. Никаких изменений в routing, shield, middleware, API handlers, egress, analytics.

Единственное изменение вне адаптеров — `ProviderConfig` (3 новых поля). Сериализация/десериализация через существующий mapstructure/yaml механизм без изменений.

## Acceptance Approach

| AC | Подход | Surfaces | Наблюдение |
|----|--------|----------|------------|
| AC-001 | Factory test: `api_type: gemini` → `*GeminiClient` | factory.go, gemini.go | go test |
| AC-002 | httptest server + GeminiClient.Call, проверка тела и заголовков запроса | gemini.go | go test |
| AC-003 | Mock SSE сервер + GeminiClient.Stream | gemini.go | go test |
| AC-004 | Factory test: `api_type: bedrock` → `*BedrockClient` | factory.go, bedrock.go | go test |
| AC-005 | Mock сервер + проверка Authorization header (SigV4) | bedrock.go | go test |
| AC-006 | Mock Bedrock Runtime + Stream | bedrock.go | go test |
| AC-007 | httptest server + ProxyClient, проверка заголовков (auth добавлен, tenant отсутствует) | proxy.go | go test |
| AC-008 | `go build ./...` | все | exit 0 |
| AC-009 | `go test ./src/internal/adapters/provider/...` | все | go test pass |

## Данные и контракты

См. `data-model.md`. Изменения минимальны: 3 поля в `ProviderConfig`.

- `ProviderClient` интерфейс — без изменений
- `ProviderRequest` / `ProviderResponse` — без изменений
- OpenAPI spec — не меняется (конфигурация провайдеров не часть API)
- Миграции БД — не требуется

## Стратегия реализации

### DEC-001: Один файл = один адаптер

Why: Каждый `api_type` имеет разный формат запроса/ответа, разную аутентификацию. Один файл на адаптер изолирует complexity, упрощает тестирование и чтение. Текущая архитектура (openai.go, anthropic.go) уже задаёт этот паттерн.

Tradeoff: Небольшой дубляж общих хелперов (buildAuthHeader, mergeHeaders). Решение: оставить в `provider.go` как shared.

Affects: gemini.go, bedrock.go, proxy.go
Validation: go build

### DEC-002: AWS SDK v2 для Bedrock вместо raw HTTP + SigV4

Why: Подпись AWS Signature V4 — нетривиальный алгоритм (канонический запрос, подпись строки, credential scope, STS-сессии). AWS SDK v2 уже решает: resolution credentials (env → shared config → IAM role), автоматическое обновление STS токенов, правильная подпись streaming запросов.

Tradeoff: `module github.com/aws/aws-sdk-go-v2` добавляет ~5 MB к бинарнику (только нужные пакеты: bedrockruntime, config, credentials).

Affects: bedrock.go, go.mod, go.sum
Validation: go build, unit test с mock (через `bedrockruntimeiface`)

### DEC-003: Proxy adapter = OpenAI-адаптер без конвертации

Why: `api_type: proxy` решает ту же задачу что `openai` — форвард OpenAI-совместимого JSON. Разница только в naming и контракте: `proxy` не обещает конвертацию, просто форвардит. Реализация — копия OpenAIClient с переименованием.

Tradeoff: Дублирование кода с openai.go. Решение: если через месяц прокси-адаптер и openai-адаптер идентичны — сделать `ProxyClient = OpenAIClient` (alias). Пока раздельные файлы для независимого тестирования.

Affects: proxy.go, factory.go
Validation: go test

### DEC-004: Gemini формат — bi-directional конвертация

Why: MaskChain pipeline оперирует в OpenAI-формате (chat completion JSON). Gemini имеет другую структуру (`contents[].parts[].text` вместо `messages[].content`). Адаптер конвертирует на входе и на выходе.

Tradeoff: Конвертация может не покрыть edge cases (function calls, multimodal в будущем). Решение: только text content phase 1.

Affects: gemini.go
Validation: unit test с golden JSON

## Incremental Delivery

### MVP: Gemini + Proxy (5-6 дней)

1. `proxy.go` — самый простой, сразу показывает паттерн (0.5 дня)
2. `gemini.go` — основная работа, конвертация формата (2 дня)
3. `factory.go` — +2 case, тесты (0.5 дня)
4. `config.go` — без изменений (proxy использует существующие поля)
5. Тесты MVP (2 дня)

Критерий: go build + go test pass.

### Итеративное расширение: Bedrock (3-4 дня)

1. `go get github.com/aws/aws-sdk-go-v2/...`
2. `bedrock.go` — SigV4, InvokeModel, Stream
3. `factory.go` — +1 case
4. `config.go` — +3 поля
5. Тесты Bedrock (mock AWS SDK)

Критерий: go build + go test pass.

## Порядок реализации

1. **Proxy** — самый простой, без новых зависимостей. Задаёт ритм.
2. **Gemini** — основная ценность MVP. Независим от Proxy.
3. **Factory** — регистрация обоих. Тесты.
4. **Bedrock** — только после MVP. Требует AWS SDK dependency review.

Order: `proxy.go || gemini.go` (параллельно) → `factory.go + tests` → `bedrock.go`

## Риски

| Риск | Mitigation |
|------|------------|
| AWS SDK добавляет 5 MB к бинарнику | Bedrock — отдельный инкремент. Если размер критичен — можно raw SigV4. |
| Gemini API может изменить формат (Google) | Адаптер конвертирует только text/chat. Если API меняется — меняем конвертер, не трогая pipeline. |
| Proxy adapter дублирует OpenAI | Если через месяц код идентичен — сделать alias. Решение deferred. |
| SigV4 для streaming сложен | SDK v2 обрабатывает корректно. Если без SDK — streaming потребует кастомной подписи каждого chunk. |

## Rollout и compatibility

- Новые `api_type` значения — обратно совместимы (старые конфиги продолжают работать)
- Bedrock config поля — опциональны, не влияют на существующие провайдеры
- Feature flag не требуется — добавление новых типов не ломает существующие
- Специальных rollout действий не требуется

## Проверка

- `go build ./...` — компиляция
- `go test ./src/internal/adapters/provider/...` — unit тесты
- `go vet ./src/internal/adapters/provider/...` — статический анализ
- `golangci-lint run ./src/internal/adapters/provider/...` — линтер
- Ручная проверка: gateway с конфигом содержащим gemini + proxy провайдеров

## Соответствие конституции

Нет конфликтов. Фича расширяет адаптеры провайдеров, не меняя core domain (Content Shield).

Готово к: /spk.tasks provider-adapters-expansion
