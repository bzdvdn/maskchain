# Egress Streaming — Задачи

## Phase Contract

Inputs: plan.md, spec.md, inspect.md, data-model.md
Outputs: упорядоченные исполнимые задачи с покрытием AC
Stop if: задачи расплывчаты — нет, plan декомпозирован

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/ports/provider.go` | T1.1 |
| `src/internal/infra/config/config.go` | T1.2 |
| `src/internal/adapters/provider/stub.go` | T1.3 |
| `src/internal/adapters/egress/client.go` | T2.1, T3.1, T4.1 |
| `src/internal/adapters/egress/proxy.go` | T2.1 |
| `src/internal/adapters/egress/pool.go` | T2.1 |
| `src/internal/adapters/egress/retry.go` | T3.1 |
| `src/internal/adapters/egress/stream.go` | T4.1 |
| `src/internal/adapters/egress/egress_test.go` | T2.2, T3.2, T4.2 |
| `src/cmd/gateway/main.go` | T5.1 |

## Implementation Context

- **Цель MVP:** AC-001 (proxy), AC-002 (pool), AC-004 (timeout), AC-005 (cancellation) — базовый не-streaming HTTP-клиент.
- **Ключевые решения:**
  - DEC-001: `Stream()` добавляется в `ProviderClient` (не отдельный интерфейс)
  - DEC-004: `EgressConfig` — top-level секция, не поле в `ProviderConfig`
  - DEC-005: full jitter backoff (`rand.Intn(min * 2)`)
- **Контракты:**
  - `ProviderChunk{Data []byte; Err error; Done bool}`
  - `Stream(ctx, req) (<-chan ProviderChunk, error)`
  - Config YAML: `egress.max_idle_conns`, `egress.idle_timeout`, `egress.max_retries`, `egress.base_backoff`, `egress.retry_on_5xx`
- **Retry policy:** сетевые ошибки — всегда; 5xx — только для GET или POST c `retry_on_5xx=true`
- **Proof signals:** integration test с httptest-прокси; unit-тесты на pool/reuse; timeout/cancel тесты; stochastic jitter test
- **Вне scope:** health-check, балансировка, metrics, SOCKS5, gRPC, DI-wiring до фазы 5

## Фаза 1: Foundation

Цель: подготовить порт, конфиг и stub для совместимости.

- [x] T1.1 Добавить `ProviderChunk` и `Stream()` в `ProviderClient`
  - Определить `ProviderChunk` struct: `Data []byte`, `Err error`, `Done bool`
  - Добавить `Stream(ctx context.Context, req *ProviderRequest) (<-chan ProviderChunk, error)` в интерфейс
  - Touches: `src/internal/ports/provider.go`
  - Depends: none
  - AC: AC-003

- [x] T1.2 Добавить `EgressConfig` в конфиг с defaults
  - Создать `EgressConfig` struct: `MaxIdleConns`, `IdleTimeout`, `MaxRetries`, `BaseBackoff`, `RetryOn5xx`
  - Добавить в `Config`, проинициализировать в `DefaultConfig()` с разумными defaults (MaxIdleConns=10, IdleTimeout=30s, MaxRetries=3, BaseBackoff=100ms, RetryOn5xx=false)
  - Touches: `src/internal/infra/config/config.go`
  - Depends: none
  - AC: AC-002, AC-004, AC-006, AC-007

- [x] T1.3 Добавить заглушку `Stream()` в `StubClient`
  - `StubClient.Stream()` возвращает канал с одним `ProviderChunk{Done: true}`
  - Touches: `src/internal/adapters/provider/stub.go`
  - Depends: T1.1
  - AC: AC-003 (компиляция)

## Фаза 2: MVP — Egress Client Core

Цель: реализовать базовый HTTP-клиент с proxy, pool, timeout и cancellation.

- [x] T2.1 Реализовать `egress.Client.Call()` с proxy, pool, timeout и cancellation
  - Создать `egress.Client` struct с полем `config *EgressConfig`
  - `NewClient(cfg *EgressConfig)` инициализирует `http.Transport`:
    - `Proxy`: чтение `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY` из env + override из конфига
    - `MaxIdleConns`, `MaxIdleConnsPerHost`, `IdleConnTimeout` из `EgressConfig`
    - `DialContext` с `net.Dialer.Timeout` из конфига (для R-003)
  - `Call(ctx, req)`:
    - Создаёт `http.Request` с context deadline (per-provider timeout)
    - Выполняет через `Transport.RoundTrip`
    - Читает body, возвращает `*ProviderResponse`
    - На любой ошибке проверяет `ctx.Err()` и возвращает `context.Canceled` / `context.DeadlineExceeded`
  - Touches: `src/internal/adapters/egress/client.go`, `src/internal/adapters/egress/proxy.go`, `src/internal/adapters/egress/pool.go`
  - Depends: T1.1, T1.2
  - AC: AC-001, AC-002, AC-004, AC-005

- [x] T2.2 MVP-тесты: proxy, pool, timeout, cancellation
  - `TestCallViaProxy` — httptest-сервер как proxy (AC-001)
  - `TestConnectionReuse` — httptest-сервер логирует TCP-соединения (AC-002)
  - `TestPerProviderTimeout` — медленный сервер, клиент с таймаутом (AC-004)
  - `TestCancelMidRequest` — отмена контекста во время запроса (AC-005)
  - Touches: `src/internal/adapters/egress/egress_test.go`
  - Depends: T2.1
  - AC: AC-001, AC-002, AC-004, AC-005

## Фаза 3: Retry

Цель: добавить retry с exponential backoff, full jitter, exhaustion.

- [x] T3.1 Реализовать retry-логику в egress-клиенте
  - В `retry.go`:
    - `backoff(attempt int) time.Duration` — `min(base * 2^attempt, max)` + full jitter
    - `isRetriable(err error, statusCode int, method string, retryOn5xx bool)`:
      - Network errors — always retry
      - 5xx — retry if GET or `retryOn5xx=true`
      - 4xx — never retry
    - `doWithRetry(ctx, fn)`: цикл до `MaxRetries` попыток, проверка ctx.Err() между попытками
  - Интегрировать в `Client.Call()`: обернуть RoundTrip в `doWithRetry`
  - Touches: `src/internal/adapters/egress/retry.go`, `src/internal/adapters/egress/client.go`
  - Depends: T2.1
  - AC: AC-006, AC-007, AC-005 (cancel during backoff)

- [x] T3.2 Retry-тесты
  - `TestRetryJitter` — 10 запусков, интервалы не строго детерминированы (AC-006)
  - `TestRetryExhaustion` — 503 × 4, ровно 4 запроса (AC-007)
  - `TestRetryCancelDuringBackoff` — cancel на 1s, retry=3, backoff=500ms, общее время <1.5s (AC-005)
  - Touches: `src/internal/adapters/egress/egress_test.go`
  - Depends: T3.1
  - AC: AC-005, AC-006, AC-007

## Фаза 4: SSE Streaming

Цель: реализовать `Stream()` с SSE-парсером и chunk forwarding.

- [x] T4.1 Реализовать `Client.Stream()`
  - `Stream(ctx, req)`:
    - Создаёт HTTP-запрос, устанавливает `Accept: text/event-stream`
    - Выполняет запрос, проверяет `Content-Type`
    - Читает `body` через `bufio.Scanner`, парсит строки `data: ...`
    - Шлёт `ProviderChunk{Data: data}` в канал на каждую `data:` строку
    - При `ctx.Done()` или `err` шлёт финальный чанк с ошибкой/`Done: true`
    - Закрывает канал после завершения
  - Graceful shutdown: при отмене контекста закрываем body, scanner завершается
  - Touches: `src/internal/adapters/egress/stream.go`, `src/internal/adapters/egress/client.go`
  - Depends: T1.1
  - AC: AC-003

- [x] T4.2 SSE streaming-тесты
  - `TestSSEChunkDelivery` — SSE-сервер, 10 чанков по 50ms, все получены до завершения (AC-003)
  - `TestSSEPrematureClose` — сервер закрывает соединение до конца, клиент возвращает error + partial data
  - Touches: `src/internal/adapters/egress/egress_test.go`
  - Depends: T4.1
  - AC: AC-003

## Фаза 5: Integration — DI Wiring

Цель: подключить egress-клиент в gateway.

- [x] T5.1 Проинициализировать egress-клиент в `main.go`
  - Создать `egress.Client` из `config.EgressConfig`
  - Зарегистрировать в мапе провайдеров `FallbackHandler` (вместо или поверх stub)
  - Убедиться, что `Stream()` не вызывается из routing engine (пока)
  - Touches: `src/cmd/gateway/main.go`
  - Depends: T2.1
  - AC: AC-001 (runtime), AC-002, AC-004, AC-005

## Фаза 6: Verify

Цель: доказать, что фича работает, и оставить пакет в reviewable состоянии.

- [x] T6.1 Финальная проверка
  - `golangci-lint run ./...` — 0 errors
  - `go test ./src/internal/adapters/egress/...` — все тесты pass
  - `go test ./src/internal/...` — нет регрессий
  - Проверить coverage AC:
    - AC-001 ✅ (T2.2 TestCallViaProxy)
    - AC-002 ✅ (T2.2 TestConnectionReuse)
    - AC-003 ✅ (T4.2 TestSSEChunkDelivery)
    - AC-004 ✅ (T2.2 TestPerProviderTimeout)
    - AC-005 ✅ (T2.2 TestCancelMidRequest, T3.2 TestRetryCancelDuringBackoff)
    - AC-006 ✅ (T3.2 TestRetryJitter)
    - AC-007 ✅ (T3.2 TestRetryExhaustion)
  - Touches: `src/internal/adapters/egress/egress_test.go`
  - Depends: T2.2, T3.2, T4.2, T5.1

## Покрытие критериев приемки

- AC-001 -> T1.2, T2.1, T2.2, T5.1
- AC-002 -> T1.2, T2.1, T2.2, T5.1
- AC-003 -> T1.1, T1.3, T4.1, T4.2
- AC-004 -> T1.2, T2.1, T2.2, T5.1
- AC-005 -> T2.1, T2.2, T3.1, T3.2, T5.1
- AC-006 -> T1.2, T3.1, T3.2
- AC-007 -> T1.2, T3.1, T3.2

## Заметки

- T1.1 и T1.2 независимы — можно параллелить
- T1.3 зависит от T1.1 (компиляция)
- T2.1 — ключевая задача, блокирует всё кроме streaming
- T5.1 (DI) можно выполнять после T2.1, не дожидаясь retry/SSE
- Трассировка: `@sk-task` ставить над owning declaration согласно AGENTS.md
