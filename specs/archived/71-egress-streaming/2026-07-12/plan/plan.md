# Egress Streaming — План реализации

## Phase Contract

Inputs: spec, inspect (pass), минимальный repo-контекст.
Outputs: plan.md, data-model.md.
Stop if: spec too vague — нет, spec конкретна.

## Цель

Реализовать `src/internal/adapters/egress/` — production HTTP/HTTPS-клиент, реализующий `ports.ProviderClient` и добавляющий `Stream()` для SSE. Адаптер читает proxy из env/config, использует настраиваемый connection pool, поддерживает retry (exponential backoff + jitter), per-provider timeout и context cancellation. Существующий `FallbackHandler` и routing engine не меняют интерфейс вызова для blocking-запросов.

## MVP Slice

AC-001 (proxy), AC-002 (pool), AC-004 (timeout), AC-005 (cancellation). Без retry и SSE — базовый транспортный уровень, подменяющий stub.

## First Validation Path

Интеграционный тест: запуск локального `httptest`-сервера + `httptest`-прокси, вызов `egress.Client.Call()` через proxy, проверка что запрос достиг сервера и ответ получен. Ручная проверка: gateway с конфигом `HTTP_PROXY=http://localhost:3128` и провайдером OpenAI, curl `POST /v1/chat/completions` возвращает not-streaming ответ.

## Scope

- `src/internal/ports/provider.go` — новый тип `ProviderChunk`, метод `Stream()` в `ProviderClient`
- `src/internal/adapters/egress/` — новый пакет: client, proxy dialer, transport pool, retry, SSE streaming
- `src/internal/infra/config/config.go` — новая секция `EgressConfig` (MaxIdleConns, IdleTimeout, MaxRetries, BaseBackoff, RetryOn5xx)
- `src/internal/adapters/provider/stub.go` — заглушка `Stream()` для тестовой совместимости
- `src/internal/domain/routing/service/fallback.go` — не меняется (blocking Call остаётся тем же)
- DI (`main.go`) — инициализация egress-клиента и регистрация в map провайдеров (явно не входит в spec scope, но потребуется; deferred до implement-фазы)

## Performance Budget

- `SC-003`: overhead retry-логики ≤1ms при успехе с первой попытки (zero allocation on happy path)
- `SC-002`: при 50 последовательных запросах к одному хосту — ≤3 TCP-соединений
- `SC-001`: SSE streaming — первый байт клиенту ≤10ms после получения от провайдера

## Implementation Surfaces

| Surface | Статус | Почему |
|---------|--------|--------|
| `src/internal/ports/provider.go` | existing, changes | добавить `ProviderChunk`, `Stream()` в интерфейс |
| `src/internal/adapters/egress/` | **new** | новый пакет — `egress.Client` |
| `src/internal/infra/config/config.go` | existing, changes | добавить `EgressConfig` |
| `src/internal/adapters/provider/stub.go` | existing, changes | заглушка `Stream()` |
| `src/cmd/gateway/main.go` | existing, touches | DI-инициализация (minimal, deferred) |

## Bootstrapping Surfaces

- `src/internal/adapters/egress/` — создать директорию и файлы
- `src/internal/infra/config/config.go` — добавить `EgressConfig` + defaults + в `Config`

## Влияние на архитектуру

- Порт `ProviderClient` расширяется новым методом `Stream()` — все существующие реализации (stub) требуют минимального обновления
- Новая конфигурационная секция `egress` — не ломает существующие конфиги (zero-value defaults)
- Routing engine не требует изменений: `FallbackHandler.Call()` продолжает работать с `Call()` через старый интерфейс
- SSE streaming — новая семантика: `Stream()` не встраивается в `Call()`, вызывается напрямую

## Acceptance Approach

| AC | Подход | Surfaces |
|----|--------|----------|
| AC-001 | `egress.Client` создаёт `http.Transport` с `Proxy` function. Тест с `httptest.NewServer` как proxy | `egress/client.go`, `egress/proxy.go` |
| AC-002 | `http.Transport` настраивается через `EgressConfig` (MaxIdleConns, IdleTimeout). Тест-сервер логирует TCP conns | `egress/pool.go`, `config.go` |
| AC-003 | `egress.Client.Stream()` читает `text/event-stream`, шлёт чанки в канал. Тест с SSE-сервером | `egress/stream.go`, `ports/provider.go` |
| AC-004 | Per-provider timeout через контекст с deadline. Тест с медленным сервером | `egress/client.go`, `config.go` |
| AC-005 | Context cancellation прерывает in-flight и retry. Тест с cancel на таймере | `egress/client.go`, `egress/retry.go` |
| AC-006 | Backoff с jitter: random deviation от strict exponential. Тест логирует интервалы | `egress/retry.go` |
| AC-007 | Retry исчерпание: ровно N+1 запросов. Тест с сервером 503 | `egress/retry.go` |

## Данные и контракты

- **Port contract**: `ProviderClient` расширяется:
  - `ProviderChunk` — `{Data []byte; Err error; Done bool}`
  - `Stream(ctx, req) (<-chan ProviderChunk, error)`
- **Config contract**: новая секция `egress` в YAML — `max_idle_conns`, `idle_timeout`, `max_retries`, `base_backoff`, `retry_on_5xx`
- **Data model**: не меняется — `data-model.md: no-change`

## Стратегия реализации

### DEC-001 Порт: добавить `Stream()` в `ProviderClient`, а не отдельный интерфейс

- **Why**: единый контракт для всех провайдеров; consumer (routing engine, handler) может не знать, поддерживает ли адаптер streaming — вызывает `Stream()` и получает канал. Stub-client возвращает одно-чанковый канал.
- **Tradeoff**: все существующие реализации должны добавить `Stream()` (stub — тривиально). Если в будущем появятся не-streaming адаптеры, они всё равно должны реализовать заглушку.
- **Affects**: `ports/provider.go`, `adapters/provider/stub.go`, `adapters/egress/client.go`
- **Validation**: компиляция; stub-тест вызывает `Stream()` и получает один чанк.

### DEC-002 Egress как отдельный пакет `adapters/egress`, а не расширение `adapters/provider`

- **Why**: provider stubs — test doubles; egress — production HTTP-клиент с собственной ответственностью (transport, proxy, retry, pool). Смешивание в одном пакете нарушит SRP.
- **Tradeoff**: дополнительный импорт в DI. Плюс: чистые границы, тесты egress не зависят от stub-логики.
- **Affects**: `adapters/egress/` (new), `adapters/provider/` (unchanged except stub)
- **Validation**: `go test ./src/internal/adapters/egress/...`

### DEC-003 Retry — встроенный в egress-клиент, а не middleware-слой

- **Why**: retry требует доступа к transport (timeout, cancellation) и конфигу провайдера. Отдельный middleware умножит сущности без выгоды — retry-логика проста (backoff, jitter, max_attempts).
- **Tradeoff**: retry нельзя переиспользовать для других типов запросов (gRPC, etc). На данный момент нет других протоколов.
- **Affects**: `egress/retry.go`, `egress/client.go`
- **Validation**: AC-006, AC-007

### DEC-004 Config: новая секция `EgressConfig` top-level, а не поля в `ProviderConfig`

- **Why**: `ProviderConfig` — routing-конфигурация (name, base_url, priority). Параметры транспорта (pool, retry) — инфраструктурные, общие для всех провайдеров или задаются в `egress` секции с per-provider override в будущем.
- **Tradeoff**: два места конфигурации провайдера. Плюс: routing-конфиг не混шается с transport-конфигом.
- **Affects**: `infra/config/config.go`, `DefaultConfig()`
- **Validation**: `TestLoadConfig` с `egress.max_idle_conns=25`

### DEC-005 Backoff: full jitter, не фиксированные множители

- **Why**: full jitter (`rand.Intn(min * 2)`) эффективнее предотвращает thundering herd, чем fixed multipliers, при сопоставимом времени восстановления.
- **Tradeoff**: менее предсказуемые интервалы для тестов. Validation через stochastic test (AC-006).
- **Affects**: `egress/retry.go`
- **Validation**: AC-006

## Incremental Delivery

### MVP (Первая ценность) — AC-001, AC-002, AC-004, AC-005

1. Расширить порт: `ProviderChunk`, `Stream()` в интерфейс
2. Создать `egress.Client` с `Call()`: proxy dialer, connection pool, per-provider timeout, context cancellation
3. Добавить `EgressConfig` в конфиг
4. Интеграционные тесты: proxy, pool, timeout, cancellation
5. Заглушка `Stream()` в stub-клиенте

### Итеративное расширение

**Итерация 2 — Retry (AC-006, AC-007):**
1. Реализовать `retry` с full jitter, exponential backoff, max_attempts
2. Retry policy: сетевые ошибки — всегда; 5xx — только если `retry_on_5xx`
3. Тесты: jitter, exhaustion, cancellation во время retry-паузы

**Итерация 3 — SSE streaming (AC-003):**
1. Реализовать `Stream()`: SSE-парсер, chunk forwarding через канал
2. Graceful shutdown чанков при отмене контекста
3. Тесты: SSE-сервер с задержками, premature close

## Порядок реализации

1. **Шаг 1 (блокирующий)**: порт `ProviderChunk` + `Stream()` — компиляция без него невозможна
2. **Шаг 2 (MVP, параллелимо с 1)**: `EgressConfig` в конфиге
3. **Шаг 3 (MVP)**: `egress.Client` с `Call()`, proxy, pool, timeout
4. **Шаг 4 (MVP)**: интеграционные тесты MVP
5. **Шаг 5**: retry-логика + тесты
6. **Шаг 6**: SSE streaming + тесты

## Риски

- **R-001 — Proxy CONNECT для HTTPS может потребовать нестандартного dialer**: Go `net/http` Transport с `Proxy`-функцией корректно обрабатывает CONNECT, но enterprise-прокси с аутентификацией могут потребовать `Proxy-Authorization` header. Mitigation: тест с proxy, требующим basic auth; в DEC явно не закладываем, добавим если понадобится.
- **R-002 — SSE streaming требует фиксации формата чанка**: провайдеры могут отправлять нестандартный SSE (multiple data lines, event types). Mitigation: реализовать минимальный парсер (data: ...\n\n), остальное — raw forwarding. Усложнение — по факту совместимости.
- **R-003 — Per-provider timeout не защищает от slow headers**: timeout через `context.WithDeadline` срабатывает на чтение body, но не на установку соединения. Mitigation: использовать `net.Dialer.Timeout` в transport.
- **R-004 — Retry budget**: без ограничения retry в единицу времени можно усилить нагрузку на уже проблемный провайдер. Mitigation: на данном этапе фиксированное `max_retries` (3), budget deferred до появления наблюдаемости.

## Rollout и compatibility

- Новая конфигурационная секция `egress` — обратно совместима (zero defaults = предыдущее поведение с `http.DefaultTransport`)
- `ProviderClient` с новым `Stream()` — существующие реализации (stub) обновлены; runtime-совместимость: `FallbackHandler` не вызывает `Stream()`, только `Call()`
- DI-изменения: `egress.Client` создаётся в `main.go` и регистрируется в `FallbackHandler` вместо stub; rollout — перезапуск gateway
- Специальных rollout-действий не требуется

## Проверка

| Шаг | Проверка | AC/DEC |
|-----|----------|--------|
| Unit-тесты egress | `TestCallProxy`, `TestConnectionReuse`, `TestTimeout`, `TestCancelMidRequest` | AC-001, AC-002, AC-004, AC-005 |
| Unit-тесты retry | `TestRetryJitter`, `TestRetryExhaustion`, `TestRetryCancelDuringBackoff` | AC-006, AC-007 |
| Unit-тесты streaming | `TestSSEChunkDelivery`, `TestSSEPrematureClose` | AC-003 |
| Интеграционный тест | Gateway + httptest-прокси + httptest-провайдер | AC-001, AC-004 |
| Линтер | `golangci-lint run ./...` | — |

## Соответствие конституции

- нет конфликтов
