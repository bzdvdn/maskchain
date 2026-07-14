# Critical Test Coverage — Задачи

## Phase Contract

Inputs: plan.md, data-model.md, spec.md (для уточнения AC), constitution summary, минимальный repo-контекст.
Outputs: tasks.md с фазами, Surface Map, покрытие AC.
Stop if: нет — plan конкретен, все AC привязываются к задачам.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/api/server_test.go` | T2.1, T3.5 |
| `src/internal/api/mask_handler_test.go` | T2.2, T2.3, T3.3 |
| `src/internal/api/middleware/shield_test.go` | T3.1 |
| `src/internal/api/provider_handler_test.go` | T3.2, T3.5 |
| `src/internal/api/integration_test.go` (новый) | T3.4 |

## Implementation Context

- **Цель MVP:** graceful shutdown (AC-001), mask→unmask цикл (AC-002), HandleUnmask три состояния (AC-007)
- **Инварианты/семантика:**
  - Все тесты in-memory с mock-зависимостями (inline mocks, существующий паттерн)
  - Graceful shutdown: программная отмена контекста, без сигналов ОС
  - Mask storage mock: in-memory map для сохранения состояния между mask/unmask
  - Production-код не меняется (кроме минимального экспорта для тестируемости)
- **Ошибки/коды:** 200 = успех, 403 = blocked, 500 = storage error, 502 = shield error, 400 = bad request/missing tenant, 415 = non-JSON content type, 503 = all unhealthy
- **Контракты/протокол:**
  - Mask: `POST /api/v1/shield/mask` → `{mask_id, masked_text}`; Unmask: `POST /api/v1/shield/unmask` → `{original_text}`
  - Shield headers: `X-Shield-Status` (clean/blocked/error), `X-Shield-Incident-ID` (UUID)
  - Auth: `X-API-Key` header, config `Auth{Enabled: false}` для отключения в integration-тесте
  - Routing: `POST /v1/chat/completions`, `POST /v1/completions`
- **Границы scope:** не трогаем production-код (кроме экспорта поля/метода); не добавляем gomock; не пишем E2E/load/UI-тесты
- **Proof signals:** `go test ./src/internal/api/... ./src/internal/api/middleware/...` проходит; каждый AC имеет observable assertion
- **References:** DEC-001 (integration без build tag), DEC-002 (inline mocks), DEC-003 (graceful shutdown через httptest), DM (no-change)

## Фаза 1: Базовый каркас

Пропущена — bootstrapping surfaces отсутствуют (plan: `none`).

## Фаза 2: MVP Slice (AC-001, AC-002, AC-007)

Цель: поставить минимальную независимо демонстрируемую ценность — graceful shutdown, mask→unmask цикл, HandleUnmask три состояния.

- [x] T2.1 Добавить TestGracefulShutdown — проверяет, что Server.Shutdown() корректно завершает активные запросы в течение таймаута через httptest.Server + программный cancel контекста (DEC-003). Touches: `src/internal/api/server_test.go`, `src/internal/api/server.go` (возможен minimal export поля http)
- [x] T2.2 Добавить TestHandleUnmask — проверяет HandleUnmask с существующим mask_id (200 + тело), несуществующим mask_id (200 + пустой результат), ошибкой storage.Get (500). Touches: `src/internal/api/mask_handler_test.go`
- [x] T2.3 Добавить TestMaskUnmaskCycle — полный цикл POST /mask → POST /unmask через HTTP handler, mock storage с in-memory map. Touches: `src/internal/api/mask_handler_test.go`

## Фаза 3: Основная реализация (AC-003, AC-004, AC-006, AC-008, AC-005 + edge cases)

Цель: расширить покрытие на graceful degradation shield, fallback routing, ProxyCompletionHandler, integration-тест и краевые случаи.

- [x] T3.1 Добавить тесты graceful degradation shield middleware: default_action=allow (200 + X-Shield-Status: error + X-Shield-Incident-ID), default_action=block (403), disabled shield (пропускает), cancel контекста, ScanResult=(nil,nil). Touches: `src/internal/api/middleware/shield_test.go`
- [x] T3.2 Добавить TestProxyCompletionHandler — POST /v1/completions с healthy провайдером, ожидает 200. Touches: `src/internal/api/provider_handler_test.go`
- [x] T3.3 Добавить TestHandleMaskStorageError — HandleMask при ошибке storage.Save() возвращает 500. Touches: `src/internal/api/mask_handler_test.go`
- [x] T3.4 Добавить integration-тест полного цикла (AC-005) — in-process gateway с auth (valid key) → shield (clean) → routing (healthy provider) → mock egress, ожидает 200 + X-Shield-Status: clean + X-Request-ID (DEC-001: отдельный файл без build tag). Touches: `src/internal/api/integration_test.go`
- [x] T3.5 Добавить edge case тесты: nil routingHandler в RegisterProxyRoute (legacy path без паники), X-Request-ID propagation через proxy, NotFound (404), /metrics route registration. Touches: `src/internal/api/provider_handler_test.go`, `src/internal/api/server_test.go`

## Фаза 4: Проверка

Цель: доказать, что все тесты проходят, и оставить пакет в reviewable состоянии.

- [x] T4.1 Запустить `go test ./src/internal/api/... ./src/internal/api/middleware/...` — убедиться, что все новые и существующие тесты проходят без ошибок.
- [x] T4.2 Запустить `go test ./src/internal/...` — полный прогон без regression.

## Покрытие критериев приемки

- AC-001 → T2.1
- AC-002 → T2.3
- AC-003 → T3.1
- AC-004 → (покрыт существующим TestRoutingHandlerFallbackIntegration; verify через T4.1)
- AC-005 → T3.4
- AC-006 → T3.2
- AC-007 → T2.2
- AC-008 → T3.3

Готово к: /spk.implement 117-critical-test-coverage
