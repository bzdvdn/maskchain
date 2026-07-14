# Critical Test Coverage — План

## Phase Contract

Inputs: spec, inspect (pass), минимальный repo-контекст (test patterns, mock подход, code surfaces).
Outputs: plan.md, data-model.md (no-change).
Stop if: intent spec неоднозначна — нет, spec и inspect verify aligned.

## Цель

Добавить тесты для закрытия критических пробелов в provider_handler, shield middleware, mask_handler, server и создать integration-тест полного цикла. Production-код не меняется; все тесты идут в существующие `_test.go` файлы (кроме нового `integration_test.go`). Работа безопасна: новые тесты только добавляют покрытие, не трогают логику.

## MVP Slice

Graceful shutdown (AC-001) + HandleUnmask полный цикл (AC-002, AC-007) + один integration golden path (AC-005). Эти три AC закрывают самые критичные пробелы и дают baseline для расширения.

## First Validation Path

```bash
go test ./src/internal/api/... ./src/internal/api/middleware/... -count=1 -run 'TestGracefulShutdown|TestMaskHandler_HandleUnmask|TestIntegration_GoldenPath'
```

После MVP те же пакеты: `go test ./src/internal/api/... -count=1`.

## Scope

- `src/internal/api/provider_handler_test.go` — новые тесты (tenant-scoped routing, таймауты, ProxyCompletionHandler, nil routingHandler, X-Request-ID propagation)
- `src/internal/api/middleware/shield_test.go` — graceful degradation default_action=block/allow, disabled shield, cancel context, ScanResult=(nil,nil), X-Shield-Incident-ID on degraded
- `src/internal/api/mask_handler_test.go` — HandleUnmask 3 состояния, HandleMask storage error, полный mask→unmask через HTTP
- `src/internal/api/server_test.go` — Shutdown graceful, 404, /metrics, route registration stubs
- `src/internal/api/integration_test.go` — НОВЫЙ файл: сквозной тест через in-process gin engine c auth + shield + routing + egress mock
- Production-код не изменяется

## Performance Budget

- `none` — тесты не затрагивают performance-critical paths; добавление тестовых функций не влияет на latency/memory production-профиля.

## Implementation Surfaces

| Surface | Статус | Почему |
|---------|--------|--------|
| `src/internal/api/provider_handler_test.go` | existing | добавляются новые test functions |
| `src/internal/api/middleware/shield_test.go` | existing | добавляются новые test functions |
| `src/internal/api/mask_handler_test.go` | existing | добавляются новые test functions |
| `src/internal/api/server_test.go` | existing | добавляются новые test functions |
| `src/internal/api/integration_test.go` | **new** | сквозной тест требует изолированного файла (не смешивать с модульными тестами одного компонента) |

## Bootstrapping Surfaces

- `none` — вся необходимая структура (gin test mode, httptest, in-file моки) уже существует.

## Влияние на архитектуру

- Локальное: только добавление тестовых функций в существующие файлы + один новый `_test.go` файл в том же пакете.
- Архитектурное: нет. Ни один production-файл не меняется.
- Migration/compatibility: нет.

## Acceptance Approach

- **AC-001** (graceful shutdown): `server_test.go` — запуск Server с `net.Listen("tcp", "127.0.0.1:0")`, долгий handler через sleep, программный вызов Shutdown с таймаутом, assert на возврат и completion запроса.
- **AC-002** (mask→unmask cycle): `mask_handler_test.go` — gin engine с MaskHandler, два последовательных HTTP запроса (mask → unmask), assert на сохранение и восстановление.
- **AC-003** (graceful degradation): `middleware/shield_test.go` — mock Scanner c err, shield config default_action=allow и block, assert на 200/403 + X-Shield-Status + X-Shield-Incident-ID.
- **AC-004** (fallback chain): `provider_handler_test.go` — два mock ProviderClient (unhealthy primary + healthy fallback), assert на 200 и вызов fallback.
- **AC-005** (integration golden path): `integration_test.go` — полный in-process chain: gin engine → auth middleware (valid key) → shield middleware (clean mock) → routing handler (healthy mock) → assert 200 + X-Shield-Status: clean + X-Request-ID.
- **AC-006** (ProxyCompletionHandler): `provider_handler_test.go` — POST /v1/completions, assert 200 + валидное тело ответа.
- **AC-007** (HandleUnmask states): `mask_handler_test.go` — mock storage с тремя сценариями (success, not-found, error) через табличный тест, assert на HTTP статус и тело.
- **AC-008** (HandleMask storage error): `mask_handler_test.go` — mock storage.Save returns error, POST /mask, assert 500.

## Данные и контракты

- Changes: `data-model.md` — status: no-change (тесты не меняют модель данных).
- API contracts не расширяются.
- Для integration-теста используется in-process chain без реальных external dependencies.

## Стратегия реализации

- **DEC-001 Integration-тест в отдельном файле без build tag**
  Why: отдельный `integration_test.go` в пакете `api` — изоляция сквозного сценария от модульных тестов каждого компонента. Без build tag, чтобы тест выполнялся в стандартном `go test ./...` (не требует external deps).
  Tradeoff: небольшой overhead при каждом `go test` — оправдан, т.к. тест in-process и быстрый.
  Affects: `src/internal/api/integration_test.go`
  Validation: `go test ./src/internal/api/... -run TestIntegration` проходит.

- **DEC-002 Production-код не трогать; если нужно — экспорт через `_test.go` helper**
  Why: spec оговаривает минимальное изменение production-кода как допустимое. Предпочтительный путь — тестирование через публичные API; если struct/function неэкспортирована и блокирует тест — добавляем helper в `export_test.go` (Go pattern: `//go:build test` или просто `_test.go` с type alias). Менять `.go` файлы вне `_test.go` — только если иначе невозможно.
  Tradeoff: `export_test.go` — общепринятый Go-паттерн, не влияет на production-бинарник.
  Affects: потенциально `src/internal/api/export_test.go`
  Validation: `go build ./...` не меняется.

- **DEC-003 In-file моки, как в существующих тестах**
  Why: кодовая база последовательно использует in-file manual mocks (см. mockEngine, mockPortClient, mockStorage). Вводить gomock/mockgen для одной фичи — избыточно и нарушает соглашение.
  Tradeoff: ручные моки — чуть больше boilerplate, но zero dependency и проще читаются.
  Affects: все затронутые `_test.go` файлы
  Validation: mocks реализуют требуемые интерфейсы (проверка через компилятор: `var _ ports.ProviderClient = (*mockPortClient)(nil)`).

- **DEC-004 Server.Start() не тестируется напрямую; достаточно Shutdown теста**
  Why: Start() — обёртка над `http.Server.ListenAndServe`, тестирование которого через случайный порт уже покрыто существующим graceful shutdown тестом. Добавление изолированного теста Start() не даёт новой уверенности.
  Tradeoff: минимальный риск, что ListenAndServe вернёт ошибку, отличную от http.ErrServerClosed; принимается.
  Affects: ни один файл не затрагивается
  Validation: AC-001 (graceful shutdown) подтверждает, что Server корректно стартует и останавливается.

## Incremental Delivery

### MVP (Первая ценность)

1. **server_test.go** — TestGracefulShutdown (AC-001)
2. **mask_handler_test.go** — TestHandleUnmask (AC-007) + TestMaskUnmaskCycle (AC-002) + TestHandleMaskStorageError (AC-008)
3. **integration_test.go** — TestIntegrationGoldenPath (AC-005)
4. **provider_handler_test.go** — TestFallbackChain (AC-004)

Validation: `go test ./src/internal/api/... -run 'TestGracefulShutdown|TestHandleUnmask|TestMaskUnmaskCycle|TestHandleMaskStorageError|TestIntegrationGoldenPath|TestFallbackChain'`

### Итеративное расширение

5. **middleware/shield_test.go** — TestGracefulDegradation (AC-003) + disabled shield + cancel context + nil scan result
6. **provider_handler_test.go** — TestProxyCompletionHandler (AC-006) + tenant-scoped routing + timeout + nil routingHandler + X-Request-ID propagation
7. **server_test.go** — TestNotFound + TestMetricsRoute + TestRouteRegistration

Validation: `go test ./src/internal/api/... ./src/internal/api/middleware/... -count=1`

## Порядок реализации

1. Server graceful shutdown — самый независимый тест, не требует моков (кроме health.Service)
2. Mask handler тесты — изолированы от middleware и routing
3. Shield middleware тесты — следующий слой после handler
4. Provider handler тесты — после shield (чтобы не было dependency confusion)
5. Integration тест — последним, т.к. собирает все компоненты вместе

Шаги 1-2 можно параллелить. Шаги 3-4 можно параллелить. Шаг 5 строго после 1-4.

Никаких feature flags не требуется — тесты не влияют на runtime.

## Риски

- **Риск 1: MaskHandler и RoutingProxyHandler зависят от concrete structs, не от интерфейсов**
  Mitigation: в тестах создаём реальные struct-заглушки через конструкторы (NewMaskHandler) с mock-зависимостями нижнего уровня (mockStorage, mockDetector). Если concrete struct требует реальной инициализации — DEC-002 (export_test.go helper).
- **Риск 2: graceful shutdown тест нестабилен в CI из-за таймингов**
  Mitigation: использовать `assert.Eventually` или разумные таймауты (2s); тест должен работать с `-count=10` без flaky.
- **Риск 3: integration-тест может потребовать изменения production-кода для подмены middleware**
  Mitigation: все middleware регистрируются через `Register*` методы; тест создаёт Server, регистрирует mock middleware через те же публичные методы. Если какая-то middleware захардкожена — DEC-002.

## Rollout и compatibility

- `none` — тесты не влияют на production-сборку, бинарник, конфигурацию или поведение.

## Проверка

| Шаг | Automated checks | Manual/Review | AC/DEC |
|-----|-----------------|---------------|--------|
| После MVP | `go test -count=1 ./src/internal/api/... -run 'GracefulShutdown\|HandleUnmask\|MaskUnmaskCycle\|HandleMaskStorage\|IntegrationGoldenPath\|FallbackChain'` | review: каждый AC имеет observable assertion | AC-001, AC-002, AC-004, AC-005, AC-007, AC-008, DEC-001 |
| После расширения | `go test -count=1 ./src/internal/api/... ./src/internal/api/middleware/...` | review: в каждом test file есть trace-маркеры `@sk-test` над новыми test функциями | AC-003, AC-006, DEC-002, DEC-003 |
| Финальная | `go test -count=1 -race ./src/internal/api/... ./src/internal/api/middleware/...` | review: spec ACs checklist (8/8) | все AC |

## Соответствие конституции

- нет конфликтов. Фича следует DDD: тесты кладутся рядом с кодом (в пакет `api`, `middleware`). Не нарушает language policy (comments=en). Не затрагивает Core Domain (Content Shield) production-код. Data model не меняется.
