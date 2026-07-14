# Connection Pool Fixes — Задачи

## Phase Contract

Inputs: plan.md (increments: MVP → TLS → CB), data-model.md (EgressTLSConfig, CircuitBreakerConfig)
Outputs: исполнимые задачи с Touches и coverage AC
Stop if: задачи расплывчаты — не остановлен (plan детален)

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/infra/config/config.go` | T1.1, T3.1 |
| `src/internal/adapters/egress/pool.go` | T2.1, T3.2 |
| `src/internal/adapters/egress/client.go` | T2.2, T3.4 |
| `src/internal/adapters/egress/circuit_breaker.go` (new) | T3.3 |
| `src/internal/adapters/provider/factory.go` | T2.3, T3.4 |
| `src/internal/adapters/egress/egress_test.go` | T2.4, T3.5, T4.1 |

## Implementation Context

- **Цель MVP:** исправить баг MaxIdleConnsPerHost, добавить per-provider timeout и выделенный http.Transport на провайдера
- **Инварианты/семантика:**
  - `NewClient(egressCfg)` сохраняет обратную совместимость (используется старым кодом)
  - `NewClientWithTransport(cfg, tp, timeout, cb)` — новый конструктор для per-provider клиентов
  - `ProviderConfig.Timeout` парсится `time.ParseDuration`; fallback на default EgressConfig timeout при ошибке/пустом значении
  - TLS-конфигурация nil-safe: если `EgressConfig.TLS == nil` — стандартный TLS без изменений
  - Circuit breaker: `atomic.Int64` для счётчика ошибок + `atomic.Int64` для cooldown deadline; `Allow()` без блокировки, `Fail()` под mutex
- **Ошибки/коды:**
  - Невалидный TLS-файл → fail-fast при создании транспорта (ошибка инициализации)
  - Circuit breaker open → `errors.New("provider skipped by circuit breaker")`
  - Невалидный ProviderConfig.Timeout → log.Warn + fallback на default из EgressConfig
- **Контракты/протокол:**
  - `EgressConfig.TLS` и `EgressConfig.CircuitBreaker` — опциональные указатели (nil-safe)
  - Stream() применяет timeout и CB так же, как Call()
- **Границы scope:** не меняем proxy.go, retry.go, stream.go, ports интерфейсы, domain/routing/health_status.go
- **Proof signals:**
  - `go test ./src/internal/adapters/egress/...` — зелёный
  - `go test ./src/internal/adapters/provider/...` — зелёный
  - `go test ./src/internal/infra/config/...` — новый конфиг парсится корректно
- **References:** DEC-001 (NewClientWithTransport), DEC-002 (CB на atomic), DEC-003 (buildTLSConfig), DEC-004 (context.WithTimeout), DM (EgressTLSConfig, CircuitBreakerConfig)

## Фаза 1: Подготовка конфигурации

Цель: добавить новые структуры конфигурации и defaults, чтобы последующие фазы имели типы для работы.

- [x] T1.1 Добавить `EgressTLSConfig` и `CircuitBreakerConfig` структуры в `config.go`, расширить `EgressConfig` полями `TLS` и `CircuitBreaker`, проставить defaults (CB MaxFailures=3, Cooldown=30s).
  Touches: `src/internal/infra/config/config.go`
  AC: — (bootstrapping)

## Фаза 2: MVP — баг, timeout, изоляция транспорта

Цель: AC-001 + AC-002 + AC-008 — минимальный срез.

- [x] T2.1 Исправить `pool.go:22`: `MaxIdleConnsPerHost: cfg.MaxIdleConns` → `cfg.MaxIdleConnsPerHost`. Подключить `cfg.DisableKeepAlives` к `Transport.DisableKeepAlives`.
  Touches: `src/internal/adapters/egress/pool.go`
  AC: AC-001

- [x] T2.2 Добавить в `Client` поля `timeout time.Duration`, `cb *CircuitBreaker`, конструктор `NewClientWithTransport(cfg, tp, timeout, cb)`. В `Call()`/`Stream()` применять `context.WithTimeout` если переданный ctx не имеет deadline и timeout > 0. Если `cb` не nil — проверять `cb.Allow()` перед HTTP-вызовом.
  Touches: `src/internal/adapters/egress/client.go`, `src/internal/adapters/egress/circuit_breaker.go` (compilation dependency)
  AC: AC-002, AC-006, AC-007

- [x] T2.3 В `factory.go`: для каждого провайдера создавать отдельный `http.Transport` через `NewTransport()` с его `EgressConfig`, парсить `ProviderConfig.Timeout`. Вызывать `NewClientWithTransport` вместо `NewClient`. Сигнатура `NewProviderClient` остаётся прежней.
  Touches: `src/internal/adapters/provider/factory.go`
  AC: AC-002, AC-008

- [x] T2.4 Написать тесты: `TestMaxIdleConnsPerHost` (AC-001), `TestPerProviderTimeoutFromClient` (AC-002), `TestPerProviderTransportIsolation` (AC-008). Проверить, что существующие тесты (`TestConnectionReuse`, `TestTimeout`) всё ещё проходят.
  Touches: `src/internal/adapters/egress/egress_test.go`
  AC: AC-001, AC-002, AC-008

## Фаза 3: TLS + Circuit Breaker

Цель: AC-003–AC-007 — защита соединений и отказоустойчивость.

- [x] T3.1 Добавить в `EgressConfig` defaults для `TLS` и `CircuitBreaker` (nil, если не заданы в YAML).
  Touches: `src/internal/infra/config/config.go`
  AC: — (data model завершение)

- [x] T3.2 Реализовать `buildTLSConfig(cfg *config.EgressTLSConfig) *tls.Config` в `pool.go`. Обработка: `CACert` → `RootCAs`, `InsecureSkipVerify`, `Cert+Key` → `Certificates`. Если `cfg == nil` — вернуть nil. Fail-fast при ошибке загрузки файлов.
  Touches: `src/internal/adapters/egress/pool.go`
  AC: AC-003, AC-004, AC-005

- [x] T3.3 Реализовать `CircuitBreaker` в новом `circuit_breaker.go`: поля `mu sync.Mutex`, `failures atomic.Int64`, `deadline atomic.Int64` (UnixNano). Методы: `Allow() bool` (проверяет deadline, не блокирует), `Fail()` (инкремент + open при >= MaxFailures), `Reset()` (сброс счётчика). Конструктор `NewCircuitBreaker(cfg)`.
  Touches: `src/internal/adapters/egress/circuit_breaker.go`
  AC: AC-006, AC-007

- [x] T3.4 В `factory.go`: создать `CircuitBreaker` для провайдера если `CircuitBreakerConfig` задан. В `NewTransport()` вызвать `buildTLSConfig()` если передан `TLSConfig`. Пробросить CB и TLS-настроенный транспорт в `NewClientWithTransport`. В `Client.Call()` вызывать `cb.Fail()` при ошибке и `cb.Reset()` при успехе.
  Touches: `src/internal/adapters/egress/client.go`, `src/internal/adapters/provider/factory.go`
  AC: AC-003, AC-004, AC-005, AC-006, AC-007

- [x] T3.5 Написать тесты: `TestTLSCustomCA` (AC-003), `TestTLSInsecureSkipVerify` (AC-004), `TestTLSMutualTLS` (AC-005), `TestCircuitBreakerOpen` (AC-006), `TestCircuitBreakerCooldown` (AC-007).
  Touches: `src/internal/adapters/egress/egress_test.go`
  AC: AC-003, AC-004, AC-005, AC-006, AC-007

## Фаза 4: Проверка

Цель: доказать, что фича работает, регрессий нет.

- [x] T4.1 Финальный прогон: `go test ./src/internal/adapters/egress/...` (все тесты зелёные), `go test ./src/internal/adapters/provider/...`, `go test ./src/internal/infra/config/...`. Убедиться, что существующие тесты не требуют обновления expected values (если требуют — обновить).
  Touches: `src/internal/adapters/egress/egress_test.go`
  AC: AC-001–AC-008 (проверка)

- [x] T4.2 Верифицировать отсутствие лишних trace-маркеров на package/import/file-header уровне; проверить, что `@sk-task` и `@sk-test` размещены над owning declaration.
  Touches: — (code review)
  AC: — (quality gate)

## Покрытие критериев приемки

- AC-001 -> T2.1, T2.4
- AC-002 -> T2.2, T2.3, T2.4
- AC-003 -> T3.2, T3.4, T3.5
- AC-004 -> T3.2, T3.4, T3.5
- AC-005 -> T3.2, T3.4, T3.5
- AC-006 -> T2.2, T3.3, T3.4, T3.5
- AC-007 -> T3.3, T3.4, T3.5
- AC-008 -> T2.3, T2.4

## Заметки

- T1.1 и T3.1 можно выполнить одним блоком (оба про config.go)
- T2.1 и T3.2 касаются одного файла pool.go — следить за merge при параллельной работе
- Тесты пишутся в той же фазе, что и реализация (T2.4 вместе с T2.1–T2.3, T3.5 вместе с T3.2–T3.4)
- Фаза 4 — проверочная, без нового кода фичи
