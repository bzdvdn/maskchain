# Shield Gateway Integration План

## Phase Contract

Inputs: spec, inspect (concerns — acceptance criteria resolved).
Outputs: plan, data-model (no-change stub).
Stop if: нет.

## Цель

Добавить `ShieldMiddleware` в Gin-цепочку для POST `/v1/chat/completions`. Middleware читает body, резолвит профиль (primary: заголовок `X-Shield-Profile-Slug`, fallback: `X-Tenant-ID` + `model` из конфига), прогоняет через `ShieldEngine.Scan()`, и либо прерывает запрос (403/404/502), либо передаёт управление заглушке-прокси с установленными `X-Shield-*` заголовками. Data model не меняется — расширяется только конфиг.

## MVP Slice

Middleware + profile resolution + pre-request scan + block/allow + headers. Без fallback-пути в MVP (AC-003 покрывает primary). AC-001, AC-002, AC-003, AC-006.

## First Validation Path

Запустить `go test ./src/internal/api/middleware/...` — unit test с mock ShieldEngine проверяет: запрос с critical-детекцией → 403 + заголовки. Затем интеграционный тест: in-memory gin → middleware → mock provider stub → assert status/headers.

## Scope

- `src/internal/api/middleware/shield.go` — ShieldMiddleware
- `src/internal/api/middleware/shield_test.go` — unit-тесты middleware
- `src/internal/api/provider_handler.go` — заглушка proxy handler для `/v1/chat/completions`, `/v1/completions`
- `src/cmd/gateway/main.go` — DI: прокинуть ShieldEngine в Server
- `src/internal/api/server.go` — зарегистрировать proxy route с middleware
- `src/internal/infra/config/config.go` — добавить ShieldConfig (action_on_suspicious)
- Интеграционные тесты в `src/internal/api/` (или `src/internal/api/middleware/middleware_test.go`)
- `specs/active/51-shield-gateway-integration/data-model.md` — stub no-change

**За границами scope:** настоящий прокси-роутинг к провайдерам, стриминг, Post-response scan, UI логов.

## Performance Budget

- P95 latency добавленный middleware < 5ms (без учёта времени ShieldEngine.Scan)
- ShieldEngine.Scan — уже имеет бюджет в spec `50-shield-engine`
- `none` для memory — типовой промпт < 100KB, buf копируется 1 раз

## Implementation Surfaces

| Surface | Тип | Роль |
|---|---|---|
| `api/middleware/shield.go` | new | ShieldMiddleware gin.HandlerFunc |
| `api/middleware/shield_test.go` | new | Unit тесты middleware |
| `api/provider_handler.go` | new | Заглушка proxy (200 OK + stub response) |
| `api/server.go` | modify | RegisterProxyRoute — регистрация с middleware |
| `cmd/gateway/main.go` | modify | DI: ShieldEngine → Server |
| `infra/config/config.go` | modify | ShieldConfig секция |
| `api/middleware/middleware_test.go` | modify/existing | Integration тест с in-memory server |

## Bootstrapping Surfaces

`api/middleware/` уже существует (requestid.go, cors.go, etc.). `api/provider_handler.go` — новая поверхность. Конфиг — новая секция в существующем файле.

## Влияние на архитектуру

- Локальное: новый middleware в Gin chain, применяется только к `/v1/chat/completions` group.
- Интеграция: Server получает зависимость от `*shield.ShieldEngine` через конструктор.
- Миграция: нет — новая функциональность под feature flag не требуется (middleware не затронет существующие `/api/v1/...` маршруты).

## Acceptance Approach

| AC | Подход | Surfaces | Наблюдение |
|---|---|---|---|
| AC-001 | Middleware вызывает ShieldEngine.Scan; если результат blocked — c.AbortWithStatusJSON(403) | shield.go | unit test: assert status code + body |
| AC-002 | Middleware вызывает c.Next() при clean; proxy handler отрабатывает | shield.go, provider_handler.go | integration: mock provider получает запрос |
| AC-003 | Middleware читает X-Shield-Profile-Slug, вызывает ProfileRepository.FindBySlug | shield.go | unit test: spy на ProfileRepository |
| AC-004 | Profile not found/disabled → 404 | shield.go | unit test: mock repo returns nil |
| AC-005 | ShieldEngine error → 502 | shield.go | unit test: mock engine returns error |
| AC-006 | Middleware устанавливает X-Shield-* headers через c.Header() | shield.go | unit test: assert response headers |
| AC-007 | Integration test with in-memory gin + mock engine + mock provider | middleware_test.go | go test passes for blocked+clean |
| AC-008 | Middleware пишет zap log после scan | shield.go | logger spy test: assert log fields |

## Данные и контракты

- DB model: не меняется. ProfileRepository уже имеет FindBySlug.
- Config: добавляется `Shield` секция с `ActionOnSuspicious` (enum: "block" | "pass").
- HTTP: новые response headers `X-Shield-Status`, `X-Shield-Incident-ID`.
- API контракты: новый POST `/v1/chat/completions` — заглушка, возвращает `{"choices":[...]}`.
- См. `data-model.md` — no-change.

## Стратегия реализации

- DEC-001 Middleware как отдельный файл, а не inline в handler
  Why: middleware уже выделены в отдельный пакет; shield middleware логически изолирован; легко тестировать независимо; можно добавить/убрать маршрут без правки handler.
  Tradeoff: gin.HandlerFunc не может влиять на request после Next() кроме заголовков — этого достаточно для pre-request scan.
  Affects: api/middleware/shield.go
  Validation: middleware unit test

- DEC-002 Body buffering: read + restore для c.Next()
  Why: middleware потребляет body при чтении промпта; downstream handler должен получить нетронутый body.
  Tradeoff: один полный copy тела в память (приемлемо для типового промпта <100KB).
  Affects: shield.go
  Validation: proxy handler получает оригинальный body

- DEC-003 ProfileResolver как внутренняя функция middleware, а не отдельный сервис
  Why: резолвинг простой (header → repo.FindBySlug); выделение в отдельный сервис — premature abstraction.
  Tradeoff: если добавится сложная логика fallback + кеш, придётся рефакторить.
  Affects: shield.go
  Validation: AC-003

- DEC-004 Fallback mapping в конфиге, а не в ProfileRepository
  Why: для fallback используется tenant+model → profile_slug маппинг, который оператор задаёт статически; не требуется новая колонка в БД или метод в репозитории.
  Tradeoff: статический конфиг — требует перезагрузки gateway (SIGHUP / restart).
  Affects: config.go
  Validation: отложено (не в MVP AC)

## Incremental Delivery

### MVP (Первая ценность)

- ShieldMiddleware с primary resolution (X-Shield-Profile-Slug)
- Proxy handler заглушка
- AC-001, AC-002, AC-003, AC-006
- Validation: `go test ./src/internal/api/middleware/...`

### Итеративное расширение

- Fallback resolution (X-Tenant-ID + model) → AC-004 (частично)
- ShieldConfig: ActionOnSuspicious
- Логирование → AC-008
- Proxy handler реальный (выходит за scope этого spec; отдельная фича)

## Порядок реализации

1. Config: добавить ShieldConfig (только ActionOnSuspicious в MVP)
2. ShieldMiddleware: read body, resolve profile, scan, abort/continue
3. ShieldMiddleware unit tests
4. Proxy handler stub
5. Server: RegisterProxyRoute с middleware
6. main.go: DI
7. Integration test
8. Logging (AC-008)

Шаги 2-3 параллельны (TDD: тест → имплементация).

## Риски

- **Риск**: ProfileRepository.FindBySlug может отсутствовать в адаптере для PostgreSQL.
  Mitigation: проверить существующий адаптер; если FindBySlug не реализован — добавить в том же impl pass.
- **Риск**: Middleware потребляет body раньше, чем proxy handler.
  Mitigation: use `io.NopCloser(bytes.NewBuffer(bodyBytes))` для восстановления body.
- **Риск**: TenantID не передан — нет default tenant в контексте.
  Mitigation: middleware извлекает X-Tenant-ID; если пусто — `NewTenantID("default")`.

## Rollout и compatibility

- Новый маршрут `/v1/chat/completions` не конфликтует с существующими (`/api/v1/...`).
- Middleware не влияет на существующие эндпоинты.
- Feature flag не требуется.
- После релиза: мониторить `X-Shield-Status: error` для выявления misconfiguration.

## Проверка

- **Unit** (middleware): mock ShieldEngine + mock ProfileRepository → 6 test cases (blocked, clean, profile_not_found, engine_error, empty_messages, invalid_content_type)
- **Integration** (server_test.go): in-memory gin + real middleware + mock provider → AC-007
- **Manual**: curl POST `/v1/chat/completions -H "X-Shield-Profile-Slug: test"` → 403/clean
- AC-001..AC-008 покрыты автоматическими тестами

## Соответствие конституции

нет конфликтов
