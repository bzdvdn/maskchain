# Provider Adapters План

## Phase Contract

Inputs: spec (110-provider-adapters), inspect (concerns → resolved).
Outputs: plan.md, data-model.md.
Stop if: spec неоднозначна — пройдена inspect, ok.

## Цель

Реализовать openai.go и anthropic.go — адаптеры, реализующие ports.ProviderClient, с фабрикой NewProviderClient(cfg) и добавлением полей api_type/api_key в ProviderConfig. Адаптеры — тонкий слой трансформации запросов/ответов, HTTP транспорт делегируется egress.Client.

## MVP Slice

OpenAI-compatible адаптер (openai.go) + фабрика + конфиг (api_type, api_key).

Покрывает AC-001, AC-002, AC-003, AC-005, AC-007, AC-008.

## First Validation Path

1. Собрать gateway.
2. Запустить с config.yaml, где указан routing.providers[0] с api_type: openai, base_url любого OpenAI-compatible провайдера, api_key.
3. Вызвать Chat Completion с curl → получить real completion.
4. Запустить unit-тесты с замоканным HTTP — убедиться, что Call и Stream работают без реального провайдера.

## Scope

- `src/internal/infra/config/config.go` — добавить APIType, APIKey в ProviderConfig.
- `src/internal/adapters/provider/factory.go` — NewProviderClient(cfg).
- `src/internal/adapters/provider/openai.go` — OpenAI-compatible адаптер.
- `src/internal/adapters/provider/anthropic.go` — Anthropic Messages API адаптер.
- `src/internal/adapters/provider/provider_test.go` — unit-тесты для адаптеров.
- `specs/active/110-provider-adapters/data-model.md` — формат ошибок провайдера.
- Порт `ports.ProviderClient` и `egress.Client` не меняются — только используются.

## Performance Budget

- none — адаптеры делают один HTTP-вызов через egress, overhead < 1ms, аллокации на трансформацию незначительны.

## Implementation Surfaces

| Surface | Почему | Тип |
|---|---|---|
| `src/internal/infra/config/config.go` | добавить APIType, APIKey в ProviderConfig | существующий |
| `src/internal/adapters/provider/factory.go` | фабрика по api_type | новый |
| `src/internal/adapters/provider/openai.go` | OpenAI-compatible адаптер | новый |
| `src/internal/adapters/provider/anthropic.go` | Anthropic адаптер | новый |
| `src/internal/adapters/provider/provider_test.go` | тесты для обоих адаптеров | новый |
| `src/internal/adapters/provider/provider.go` | общие типы (структура ошибки провайдера) | новый |

## Bootstrapping Surfaces

- `src/internal/adapters/provider/` уже существует (stub.go). Ничего создавать не нужно.

## Влияние на архитектуру

- ProviderConfig расширяется двумя полями — лёгкое изменение.
- Новая фабрика становится точкой входа для всех provider-адаптеров.
- Адаптеры используют egress.Client как dependency (inject через конструктор).
- Формат ошибок провайдера нормализуется в общую структуру `ProviderError`.

## Acceptance Approach

| AC | Подход | Surfaces | Наблюдение |
|---|---|---|---|
| AC-001 | unit: фабрика возвращает конкретный тип по api_type | factory.go | type assertion |
| AC-002 | unit: OpenAIClient.Call с замоканным HTTP | openai.go, egress | response body содержит choices |
| AC-003 | unit: OpenAIClient.Stream с замоканным SSE | openai.go, egress | канал закрывается с Done |
| AC-004 | unit: AnthropicClient.Call с замоканным HTTP | anthropic.go, egress | response body содержит content[0].text |
| AC-005 | unit: оба адаптера с HTTP 4xx/5xx | openai.go, anthropic.go | ProviderResponse c кодом и parsed error |
| AC-006 | unit: AnthropicClient.Stream с замоканным event-SSE | anthropic.go, egress | канал закрывается с Done |
| AC-007 | unit: Unmarshal YAML с api_type | config.go | APIType заполнен |
| AC-008 | unit: Unmarshal YAML с api_key | config.go | APIKey заполнен |

## Данные и контракты

- ProviderConfig расширяется: +APIType string, +APIKey string.
- Формат ошибки провайдера: `ProviderError {StatusCode int, Type string, Message string}` — используется внутри ProviderResponse.Body (JSON).
- ports.ProviderRequest/ProviderResponse не меняются.
- data-model.md описывает ProviderError и обновлённый ProviderConfig.

## Стратегия реализации

### DEC-001 Фабрика в adapters/provider/factory.go, а не в main.go

- Why: пакет provider владеет адаптерами — фабрика должна жить рядом с ними. main.go не существует и не должен зависеть от деталей адаптеров.
- Tradeoff: фабрика импортирует все адаптеры (cyclic deps нет — адаптеры не импортят друг друга).
- Affects: factory.go, main.go (при создании будет вызывать provider.NewProviderClient(cfg)).
- Validation: NewProviderClient вызывается без паники.

### DEC-002 Адаптер конфигурируется api_key при создании, сам проставляет заголовок

- Why: адаптер владеет знанием о том, в какой заголовок класть ключ (Authorization vs x-api-key). Клиент (routing engine) не должен об этом знать.
- Tradeoff: при смене ключа нужен перезапуск/recreate адаптера (не hot-reload).
- Affects: factory.go, openai.go, anthropic.go.
- Validation: AC-002/AC-004 проверяют, что заголовок установлен в исходящем запросе.

### DEC-003 Общая структура ProviderError в provider.go

- Why: AC-005 требует единого формата ошибок. Выносим `ProviderError {StatusCode, Type, Message}` в общий файл пакета provider.
- Tradeoff: небольшой общий тип — не worth отдельного пакета.
- Affects: provider.go (новый), openai.go, anthropic.go.
- Validation: ошибка 400 от провайдера парсится в ProviderError со StatusCode=400.

## Incremental Delivery

### MVP (AC-001, AC-002, AC-003, AC-005, AC-007, AC-008)

1. config: +APIType, +APIKey в ProviderConfig + тесты unmarshal.
2. factory.go: NewProviderClient + тест на тип по api_type.
3. openai.go: Call + Stream + error transform + unit-тесты с замоканным HTTP/SSE.

### Итеративное расширение (AC-004, AC-006)

4. anthropic.go: Call + Stream + error transform + unit-тесты.

## Порядок реализации

1. config.go — поля конфига (AC-007, AC-008) — необходимо для всего остального.
2. provider.go — ProviderError тип.
3. factory.go — фабрика (заглушка: возвращает StubClient для неизвестных типов, ошибку для неизвестных).
4. openai.go — MVP адаптер.
5. anthropic.go — второй адаптер.
6. provider_test.go — интеграционные тесты для обоих.

Параллельно: 1 и 2 независимы.

## Риски

- Разные провайдеры имеют нюансы в SSE-формате (напр., Anthropic использует event-ы, а не просто data: строки).
  Mitigation: антропик адаптер парсит event stream по спецификации Anthropic, тесты покрывают все event types.
- api_key в plaintext в YAML — безопасность.
  Mitigation: осознанное решение для этой фазы; secrets management вынесен в отдельную фазу.
- Mistral может иметь несовместимости в /v1/chat/completions.
  Mitigation: openai.go тестируется с реальным Mistral API в integ-тесте; при несовместимости — документируем и фиксим.

## Rollout и compatibility

- Новые поля api_type/api_key опциональны (по умолчанию пустая строка).
- Если api_type не указан — фабрика падает с ошибкой (не может определить адаптер).
- Существующие конфиги без api_type продолжат работать? Нет — нужно добавить api_type. На этой фазе stub всё ещё доступен для тестов (NewStubClient), но фабрика его не создаёт без явного api_type: stub.
- Специальных rollout-действий не требуется.

## Проверка

- `go test ./src/internal/adapters/provider/...` — unit-тесты для factory, openai, anthropic.
- `go test ./src/internal/infra/config/...` — unmarshal тесты для новых полей.
- `go build ./...` — компиляция без ошибок.
- Ручная проверка: curl к gateway с настроенным реальным провайдером.
- AC-001…AC-008 покрыты автоматическими тестами.

## Соответствие конституции

- нет конфликтов. Go + Clean Architecture соблюдены; trace-маркеры @sk-task будут добавлены в implement фазе.
