# Shield Gateway Integration Задачи

## Phase Contract

Inputs: plan, spec, data-model (no-change).
Outputs: исполнимые задачи с покрытием всех AC.
Stop if: нет.

## Surface Map

| Surface | Tasks |
|---|---|
| `src/internal/infra/config/config.go` | T1.1 |
| `src/internal/api/provider_handler.go` | T1.2 |
| `src/internal/api/middleware/shield.go` | T2.1 |
| `src/internal/api/middleware/shield_test.go` | T2.2, T4.2 |
| `src/internal/api/server.go` | T3.1 |
| `src/cmd/gateway/main.go` | T3.1 |
| `src/internal/api/middleware/middleware_test.go` | T3.2 |

## Implementation Context

- **Цель MVP:** ShieldMiddleware + proxy handler stub + primary profile resolution (X-Shield-Profile-Slug) + block/allow + headers. AC-001, AC-002, AC-003, AC-006.
- **Инварианты/семантика:**
  - Маршрут `/v1/chat/completions` — новый, не затрагивает существующие `/api/v1/*`
  - Middleware не изменяет request body (buffered + restored для c.Next())
  - X-Shield-Profile-Slug имеет приоритет над X-Tenant-ID + model (fallback не в MVP)
  - TenantID де-факто "default" при отсутствии X-Tenant-ID
- **Ошибки/коды:**
  - blocked → HTTP 403 `{"shield_status":"blocked","incident_id":"<uuid>"}`
  - profile not found/disabled → HTTP 404 `X-Shield-Status: error`
  - engine error → HTTP 502 `X-Shield-Status: error`
  - не JSON body → HTTP 415
  - body > 1MB → HTTP 413
- **Контракты/протокол:**
  - POST `/v1/chat/completions` — новый entrypoint
  - Response headers: `X-Shield-Status`, `X-Shield-Incident-ID`
  - Proxy handler stub возвращает `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`
- **Границы scope:**
  - Не делаем настоящий LLM-прокси (роутинг, стриминг)
  - Не делаем post-response scan
  - Не делаем fallback resolution (X-Tenant-ID + model) — только primary в MVP
- **Proof signals:**
  - `go test ./src/internal/api/middleware/...` проходит
  - `go test ./src/internal/api/...` проходит
  - curl POST `/v1/chat/completions -H "X-Shield-Profile-Slug: test"` → blocked/clean в зависимости от промпта
- **References:** DEC-001 (middleware isolation), DEC-002 (body buffering), DEC-003 (inline resolver), DM (no-change)

## Фаза 1: Foundation

Цель: подготовить config, proxy handler stub и проверить наличие ProfileRepository адаптера.

- [x] T1.1 Добавить ShieldConfig секцию в `src/internal/infra/config/config.go`: `ActionOnSuspicious` (string, "block" | "pass"), `TenantModelMapping` (map[string]map[string]string — отложено, но структура нужна). Обновить DefaultConfig. Touches: `src/internal/infra/config/config.go`
- [x] T1.2 Создать `src/internal/api/provider_handler.go` с proxy handler stub для `/v1/chat/completions` и `/v1/completions` — возвращает 200 + mock JSON `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`. Touches: `src/internal/api/provider_handler.go`

## Фаза 2: MVP Middleware

Цель: реализовать ShieldMiddleware с primary profile resolution, body buffering, scan, block/allow, headers.

- [x] T2.1 Реализовать `ShieldMiddleware(engine *shield.ShieldEngine, profileRepo shield.ProfileRepository, cfg *config.ShieldConfig, log *zap.Logger)` gin.HandlerFunc:
  - Буферизация body (io.ReadAll → io.NopCloser(bytes.NewBuffer))
  - Извлечение X-Shield-Profile-Slug из заголовка → `ProfileRepository.FindBySlug(tenantID, slug)`
  - Парсинг JSON body → извлечение messages[].content
  - Вызов `engine.Scan(ctx, ScanRequest{Text: content, ProfileSlug: slug})`
  - При blocked: c.AbortWithStatusJSON(403, blockResponse)
  - При clean: c.Header("X-Shield-Status", "clean"), c.Header("X-Shield-Incident-ID", uuid), c.Next()
  - При error/not found: abort с соответствующим статусом
  - Обработка edge cases: пустой messages (clean), не JSON (415), body > 1MB (413)
  - Touches: `src/internal/api/middleware/shield.go`
- [x] T2.2 Добавить unit тесты в `src/internal/api/middleware/shield_test.go`:
  - blocked (AC-001)
  - clean pass (AC-002)
  - profile resolution via X-Shield-Profile-Slug (AC-003)
  - profile not found → 404 (AC-004)
  - engine error → 502 (AC-005)
  - headers present (AC-006)
  - Edge: empty messages, non-JSON content-type
  - Touches: `src/internal/api/middleware/shield_test.go`

## Фаза 3: Интеграция

Цель: встроить middleware в Server, DI в main.go, интеграционный тест.

- [x] T3.1 Добавить `RegisterProxyRoute(shieldMiddleware gin.HandlerFunc)` в Server, регистрирующую POST `/v1/chat/completions` и `/v1/completions` с применением middleware + provider_handler. В `main.go` создать ShieldEngine, обернуть в ShieldMiddleware, передать в Server. Touches: `src/internal/api/server.go`, `src/cmd/gateway/main.go`
- [x] T3.2 Добавить integration test в `src/internal/api/middleware/middleware_test.go`: in-memory gin server + mock ShieldEngine + mock provider handler → полный цикл blocked и clean (AC-007). Touches: `src/internal/api/middleware/middleware_test.go`

## Фаза 4: Логирование

Цель: добавить structured logging результатов сканирования.

- [x] T4.1 Добавить логирование в ShieldMiddleware (zap.Logger): поля shield_status, profile_slug, model, latency_ms, incident_id. Логировать после завершения scan (до abort/c.Next). Touches: `src/internal/api/middleware/shield.go`
- [x] T4.2 Добавить logger spy unit test: проверить наличие полей в логе (AC-008). Touches: `src/internal/api/middleware/shield_test.go`

## Покрытие критериев приемки

- AC-001 (blocked) → T2.1, T2.2
- AC-002 (clean pass) → T2.1, T2.2
- AC-003 (profile resolution) → T2.1, T2.2
- AC-004 (profile not found) → T2.1, T2.2
- AC-005 (engine error) → T2.1, T2.2
- AC-006 (headers) → T2.1, T2.2
- AC-007 (integration test) → T3.2
- AC-008 (logging) → T4.1, T4.2
