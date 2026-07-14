# Connection Pool Fixes — План

## Phase Contract

Inputs: spec.md, inspect.md (pass), repo surfaces (config.go, pool.go, client.go, factory.go)
Outputs: plan.md, data-model.md
Stop if: spec неоднозначна для безопасного планирования (pass на inspect)

## Цель

Исправить баг `MaxIdleConnsPerHost` в pool.go, добавить per-provider timeout (парсинг ProviderConfig.Timeout), TLS-конфигурацию (custom CA, insecure, mTLS), простой circuit breaker (N errors → skip T sec) и выделенный `http.Transport` на провайдера. Все изменения локальны в `src/internal/adapters/egress/`, `src/internal/infra/config/config.go` и `src/internal/adapters/provider/factory.go`.

## MVP Slice

AC-001 + AC-002 + AC-008 (баг, timeout, изоляция транспорта) — реализуются первым инкрементом.
AC-003–AC-005 (TLS) — второй инкремент.
AC-006–AC-007 (circuit breaker) — третий инкремент.

## First Validation Path

После первого инкремента: `go test ./src/internal/adapters/egress/...` — существующие тесты зелёные, новый тест проверяет `MaxIdleConnsPerHost != MaxIdleConns`. Ручная проверка: разная конфигурация timeout в ProviderConfig → разное время ожидания в Call().

## Scope

- `src/internal/infra/config/config.go` — добавление `EgressTLSConfig`, `CircuitBreakerConfig` в `EgressConfig`; парсинг `ProviderConfig.Timeout` в `time.Duration`
- `src/internal/adapters/egress/pool.go` — исправление бага, подключение `DisableKeepAlives`, TLS-настройка транспорта
- `src/internal/adapters/egress/client.go` — добавление timeout-контекста в Call/Stream, интеграция circuit breaker
- `src/internal/adapters/egress/circuit_breaker.go` — новый файл
- `src/internal/adapters/provider/factory.go` — создание per-provider транспорта, передача timeout в egress.Client
- `src/internal/adapters/egress/egress_test.go` — новые тесты
- Не меняется: proxy.go, retry.go, stream.go, интерфейсы ports

## Performance Budget

- `none` — соединения в пуле не меняют latency при успешном сценарии; circuit breaker добавляет один `atomic.Load` на горячий путь (O(1), zero alloc)

## Implementation Surfaces

1. **`src/internal/infra/config/config.go`** (сущ.) — добавить `EgressTLSConfig`, `CircuitBreakerConfig`; парсить `ProviderConfig.Timeout`; проставить defaults
2. **`src/internal/adapters/egress/pool.go`** (сущ.) — исправить баг, добавить `buildTLSConfig()`, применить `DisableKeepAlives`
3. **`src/internal/adapters/egress/client.go`** (сущ.) — добавить поле `timeout time.Duration`, поле `cb *CircuitBreaker`; в `Call()`/`Stream()` — `ctxWithTimeout` и check circuit breaker
4. **`src/internal/adapters/egress/circuit_breaker.go`** (нов.) — реализация: `atomic.Int64` для счётчика ошибок + `atomic.Int64` для UnixNano cooldown deadline, метод `Allow()`/`Fail()`/`Reset()`
5. **`src/internal/adapters/provider/factory.go`** (сущ.) — `NewProviderClient` принимает `pcfg` и создаёт `egress.NewClientWithTransport(cfg, transport, timeout, cb)`

## Bootstrapping Surfaces

- `none` — все файлы уже существуют (circuit_breaker.go создаётся в процессе)

## Влияние на архитектуру

- Локальное: `Client` расширяется полями, `NewClient` меняет сигнатуру (или добавляется `NewClientWithTransport`)
- Изоляция транспорта нарушает текущую модель (один Transport на все вызовы) — это ожидаемое изменение по RQ-005
- Circuit breaker — новый компонент; не влияет на существующие границы, если не интегрирован с HealthStatus (опционально)

## Acceptance Approach

- **AC-001**: assertion в unit-тесте `pool.go`; подтверждение в существующем `TestConnectionReuse`
  - Surfaces: pool.go, egress_test.go
- **AC-002**: новый тест `TestPerProviderTimeout` проверяет, что client.Call завершается по таймауту
  - Surfaces: client.go, factory.go, egress_test.go
- **AC-003**: тест создаёт транспорт с кастомным CA, проверяет `RootCAs`
  - Surfaces: pool.go, config.go
- **AC-004**: тест проверяет `InsecureSkipVerify` после создания транспорта
  - Surfaces: pool.go, config.go
- **AC-005**: тест с тестовыми сертификатами проверяет `Certificates`
  - Surfaces: pool.go, config.go
- **AC-006**: unit-тест circuit breaker: N ошибок → `Allow() = false`
  - Surfaces: circuit_breaker.go
- **AC-007**: unit-тест circuit breaker: cooldown → `Allow() = true`
  - Surfaces: circuit_breaker.go
- **AC-008**: тест проверяет разные `*http.Transport` для разных провайдеров
  - Surfaces: factory.go, egress_test.go

## Данные и контракты

- Data model расширяется: `EgressConfig` получает `TLS *EgressTLSConfig` и `CircuitBreaker *CircuitBreakerConfig`. `ProviderConfig.Timeout` остаётся string, но парсится при создании клиента.
- `data-model.md` — обязателен, описывает новые структуры.

## Стратегия реализации

- DEC-001 Новый конструктор `NewClientWithTransport` вместо изменения `NewClient`
  - Why: существующие вызовы `NewClient(egressCfg)` не меняются; новый конструктор принимает транспорт, timeout, circuit breaker — это не ломает обратную совместимость
  - Tradeoff: два конструктора вместо одного; caller (factory) должен выбрать правильный
  - Affects: client.go, factory.go
  - Validation: старые вызовы `NewClient` продолжают работать без изменений

- DEC-002 Circuit breaker на `sync.Mutex` + два `atomic.Int64`
  - Why: не требует внешних библиотек; проще `sync.Mutex` чем каналы для такого примитивного скоупа; `atomic` позволяет читать `Allow()` без блокировки
  - Tradeoff: нет half-open; счётчик не различает типы ошибок
  - Affects: circuit_breaker.go (новый)
  - Validation: unit-тесты AC-006, AC-007

- DEC-003 TLS-конфигурация собирается в `buildTLSConfig()` внутри pool.go
  - Why: TLS относится к http.Transport, а не к Client; `newTransport()` уже собирает Transport — логично расширять там
  - Tradeoff: pool.go знает про TLS-конфиг; но это консистентно с текущей архитектурой
  - Affects: pool.go, config.go
  - Validation: unit-тесты AC-003, AC-004, AC-005

- DEC-004 Per-provider timeout применяется через `context.WithTimeout` внутри Call/Stream
  - Why: контекст — стандартный механизм управления таймаутами в Go; не требует изменения портов ProviderClient
  - Tradeoff: если caller уже передал ctx с deadline — приоритет у более короткого (стандартное поведение ctx)
  - Affects: client.go
  - Validation: AC-002

## Incremental Delivery

### MVP (AC-001 + AC-002 + AC-008)

- Исправить баг в pool.go
- Добавить `Timeout` field в `Client`, парсить из `ProviderConfig` в factory
- Добавить `NewClientWithTransport` — фабрика создаёт per-provider транспорт
- Тесты на AC-001, AC-002, AC-008

### Итеративное расширение 1 (AC-003, AC-004, AC-005)

- Добавить `EgressTLSConfig` в config.go
- Реализовать `buildTLSConfig()` в pool.go
- Тесты на AC-003, AC-004, AC-005

### Итеративное расширение 2 (AC-006, AC-007)

- Создать circuit_breaker.go
- Интегрировать в Client (проверка перед Call)
- Добавить `CircuitBreakerConfig` в config.go
- Тесты на AC-006, AC-007

## Порядок реализации

1. **config.go** — новые структуры и defaults (можно параллельно с circuit_breaker.go)
2. **pool.go** — фикс бага + buildTLSConfig
3. **circuit_breaker.go** — реализация
4. **client.go** — новые поля, timeout, circuit breaker check
5. **factory.go** — per-provider транспорт + вызов NewClientWithTransport
6. **egress_test.go** — тесты на каждый AC

Шаги 2–3 можно параллелить.

## Риски

- Существующие тесты `TestConnectionReuse` и `TestTimeout` ожидают определённое поведение — могут сломаться при изменении `newTransport`
  - Mitigation: запустить все тесты egress после каждого изменения; обновить expected values при необходимости
- Парсинг `ProviderConfig.Timeout` — невалидные значения в существующей конфигурации
  - Mitigation: fallback на `EgressConfig` default timeout при ошибке парсинга + log warning
- Изоляция транспорта может увеличить потребление соединений при большом числе провайдеров
  - Mitigation: каждый транспорт имеет свой `MaxIdleConns`; общий лимит = sum per-provider. Документировать в конфиге.

## Rollout и compatibility

- Новые поля `tls` и `circuit_breaker` в `EgressConfig` опциональны (nil-safe) — обратная совместимость полная
- `ProviderConfig.Timeout` не парсился раньше — после изменения начнёт работать; старые конфиги без timeout продолжат использовать default
- Специальных rollout-действий не требуется

## Проверка

- `go test ./src/internal/adapters/egress/...` — все тесты проходят
- `go test ./src/internal/adapters/provider/...` — фабрика корректно создаёт клиентов
- `go test ./src/internal/infra/config/...` — парсинг конфига с новыми полями
- Ручная проверка: поднять gateway с конфигом, где провайдеры имеют разные timeout и TLS-настройки — `/health` показывает все alive

## Соответствие конституции

- нет конфликтов: стек (Go stdlib) и архитектура (DDD) соблюдены
