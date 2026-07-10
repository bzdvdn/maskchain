# Shield Reactions Задачи

## Phase Contract

Inputs: plan.md, data-model.md, spec.md.
Outputs: исполнимые задачи с покрытием AC.
Stop if: все AC привязаны к задачам — блокеров нет.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/domain/shield/errors/errors.go` | T1.1 |
| `src/internal/domain/shield/reaction/` (NEW) | T1.1, T2.1, T2.2, T2.3, T2.4, T2.5 |
| `src/internal/domain/shield/mask/usecase.go` | T1.2 |
| `src/internal/domain/shield/mask/errors.go` | T1.2 (если нужно добавить sentinel) |
| `src/internal/api/mask_handler.go` | T1.2 |
| `src/internal/domain/shield/errors/errors_test.go` | T4.1 |
| `src/internal/domain/shield/reaction/*_test.go` | T2.2, T2.3, T2.4, T2.5, T3.1 |

## Implementation Context

- **Цель MVP:** BlockReaction (403), RedactReaction (`*`-маска), AlertReaction (лог), ReactionPipeline (routing) на существующих entity.Reaction от PolicyEvaluator.
- **Инварианты/семантика:**
  - ReactionExecutor.Execute(ctx, scanResult, text) → (modifiedText, error)
  - Позиция фрагмента в Incident не гарантирована (ScanPipeline не заполняет position) — RedactReaction ищет по `strings.Index`
  - BlockReaction возвращает domain error с `errors.Is(err, ErrBlockedByPolicy)` — HTTP 403 обрабатывает handler
  - MaskReaction конвертирует Incident → `detector.DetectorResult` и делегирует `MaskFromResults`
- **Ошибки/коды:**
  - `ErrBlockedByPolicy` — новый sentinel в shield/errors
- **Контракты/протокол:**
  - ReactionPipeline interface: `Execute(ctx, reaction, scanResult, text) → (string, error)`
  - Default mapping: ReactionBlock→Block, ReactionLog→Redact, ReactionReview→Alert, ReactionAllow→noop
- **Границы scope:**
  - PolicyEvaluator и entity.Reaction не меняются
  - API endpoints для shield pipeline (использующие реакции) — отдельная фича
- **Proof signals:**
  - Каждый executor проходит unit-тест с доказательством трансформации
  - Pipeline routing тест с mock executor'ами
  - `make lint && go build ./...` проходит
- **References:** DEC-001 (executor interface), DEC-002 (MaskFromResults), DEC-003 (domain error), DEC-004 (pipeline interface). Data model: no-change.

## Фаза 1: Foundation

Цель: sentinel error + reaction package scaffold + рефакторинг MaskUseCase.

- [x] T1.1 Добавить `ErrBlockedByPolicy` sentinel error в shield/errors и создать `src/internal/domain/shield/reaction/` package (пустой, только `doc.go` или `package declaration`). Touches: `src/internal/domain/shield/errors/errors.go`, `src/internal/domain/shield/reaction/` (NEW)

- [x] T1.2 Рефакторинг MaskUseCase: выделить `MaskFromResults(ctx, text, maskID, []detector.DetectorResult)` из `MaskText`. `MaskText` становится обёрткой (scan → MaskFromResults). Мигрировать `mask_handler.go` на `MaskFromResults`. После миграции — удалить `MaskText`. Touches: `src/internal/domain/shield/mask/usecase.go`, `src/internal/api/mask_handler.go`

## Фаза 2: MVP — Executors + Pipeline

Цель: ReactionExecutor interface, Block/Redact/Alert реализации, ReactionPipeline.

- [x] T2.1 Определить `ReactionExecutor` interface в `reaction/executor.go` с методом `Execute(ctx, *entity.ScanResult, string) (string, error)`. Touches: `src/internal/domain/shield/reaction/executor.go`

- [x] T2.2 Реализовать `BlockReaction`: возвращает (текст без изменений), ошибка с `ErrBlockedByPolicy`. В сообщении ошибки — severity и тип детектора. + unit-тест (AC-001). Touches: `src/internal/domain/shield/reaction/block.go`, `src/internal/domain/shield/reaction/block_test.go`

- [x] T2.3 Реализовать `RedactReaction`: заменяет каждый фрагмент из `ScanResult.Incidents()` на `strings.Repeat("*", len(fragment))`. Фрагменты ищет через `strings.Index` (позиция не гарантирована). + unit-тест (AC-002). Touches: `src/internal/domain/shield/reaction/redact.go`, `src/internal/domain/shield/reaction/redact_test.go`

- [x] T2.4 Реализовать `AlertReaction`: для каждого incident из ScanResult вызывает `IncidentRepository.Save`, возвращает текст без изменений. + unit-тест с mock IncidentRepository (AC-004). Touches: `src/internal/domain/shield/reaction/alert.go`, `src/internal/domain/shield/reaction/alert_test.go`

- [x] T2.5 Реализовать `ReactionPipeline` interface + default реализацию с маппингом Reaction → executor. + unit-тест с mock executor'ами для всех 4 Reaction (AC-005). Touches: `src/internal/domain/shield/reaction/pipeline.go`, `src/internal/domain/shield/reaction/pipeline_test.go`

## Фаза 3: P3 — MaskReaction

Цель: обратимое маскирование через MaskFromResults.

- [x] T3.1 Реализовать `MaskReaction`: конвертирует `Incident` → `detector.DetectorResult`, вызывает `MaskUseCase.MaskFromResults`. + unit-тест с in-memory MaskStorage (AC-003). Touches: `src/internal/domain/shield/reaction/mask.go`, `src/internal/domain/shield/reaction/mask_test.go`

- [x] T4.1 Обновить `errors_test.go` — добавить `ErrBlockedByPolicy` в тест на уникальность sentinel errors. Touches: `src/internal/domain/shield/errors/errors_test.go`

- [x] T4.2 `make lint && go build ./... && go test ./...` — код компилируется, линтер чист, все тесты проходят. Touches: `(workspace)`

## Покрытие критериев приемки

- AC-001 → T2.2
- AC-002 → T2.3
- AC-003 → T3.1
- AC-004 → T2.4
- AC-005 → T2.5

## Заметки

- T1.1 и T1.2 — gate для всей фичи (handler был бы сломан без миграции)
- T2.2, T2.3, T2.4, T2.5 — независимы, можно параллелить
- T3.1 — P3, зависит от T1.2 и T2.1
- T4.1 — можно делать сразу после T1.1
- Фаза 4 (T4.2) — финальная, после всех остальных
