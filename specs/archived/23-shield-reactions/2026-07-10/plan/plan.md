# Shield Reactions План

## Phase Contract

Inputs: spec, inspect (pass), repo context (shield domain structure).
Outputs: plan.md, data-model.md (no-change stub).
Stop if: spec permits safe planning — нет блокеров.

## Цель

Построить isolirоvannый пакет `reaction` в domain shield, реализовать 4 стратегии реакции на sensitive data (Block, Redact, Mask, Alert) и интерфейс ReactionPipeline для выбора executor'а на основе entity.Reaction от PolicyEvaluator. Заодно рефакторинг MaskUseCase: MaskText → MaskFromResults + миграция API handler.

## MVP Slice

BlockReaction + RedactReaction + AlertReaction + ReactionPipeline (AC-001, AC-002, AC-004, AC-005). MaskReaction (AC-003) — P3, после MVP.

## First Validation Path

Unit-тест ReactionPipeline с mock executor'ами: передать ReactionBlock → убедиться, что BlockReaction.Execute вызван и вернул ошибку 403. Весь цикл без внешних зависимостей.

## Scope

- Новый пакет `src/internal/domain/shield/reaction/`
- ReactionExecutor interface + BlockReaction, RedactReaction, AlertReaction, MaskReaction
- ReactionPipeline interface + default implementation (маппинг Reaction → executor)
- Refactoring: MaskUseCase.MaskFromResults + миграция mask_handler.go с MaskText
- Sentinel error `ErrBlockedByPolicy` в shield/errors
- Unit-тесты на каждый executor и pipeline
- MaskReaction — P3 (отдельная задача после MVP)

## Performance Budget

- `none` — реакции stateless (кроме MaskReaction с I/O в MaskStorage). Для SC-001/SC-002 (<100ms на тест) достаточно in-memory storage/mocks.

## Implementation Surfaces

| Surface | Change | Reason |
|---------|--------|--------|
| `src/internal/domain/shield/reaction/` | **NEW** | Новый пакет: executor interface + реализации + pipeline interface |
| `src/internal/domain/shield/mask/usecase.go` | Modify | Добавить `MaskFromResults`, удалить `MaskText` |
| `src/internal/domain/shield/mask/uuid.go` | No change | `NewUUIDv7` уже существует, MaskReaction использует его |
| `src/internal/domain/shield/errors/errors.go` | Modify | Добавить `ErrBlockedByPolicy` |
| `src/internal/api/mask_handler.go` | Modify | Мигрировать с `MaskText` на registry.ScanAll + `MaskFromResults` |
| `src/internal/domain/shield/entity/reaction.go` | No change | Существующий enum не меняется |
| `src/internal/domain/shield/service/evaluate.go` | No change | PolicyEvaluator не меняется |

## Bootstrapping Surfaces

- `src/internal/domain/shield/reaction/` — создать директорию и package, затем наполнять executor'ами

## Влияние на архитектуру

- Локальное: новый пакет `reaction` в domain shield, не затрагивает инфраструктуру
- MaskUseCase теряет `MaskText` (public API change), handler мигрируется в этом же PR
- PolicyEvaluator и entity.Reaction остаются нетронутыми

## Acceptance Approach

### AC-001 BlockReaction 403

- Подход: unit-тест BlockReaction.Execute с произвольным ScanResult
- Surfaces: `reaction/block.go`, `errors/errors.go`
- Наблюдение: `errors.Is(err, ErrBlockedByPolicy)`, в сообщении severity/тип детектора

### AC-002 RedactReaction

- Подход: unit-тест с текстом `"email: user@example.com"` и incident с фрагментом
- Surfaces: `reaction/redact.go`
- Наблюдение: результат `"email: ****************"` (16 звездочек)

### AC-003 MaskReaction (P3)

- Подход: unit-тест с in-memory MaskStorage, проверка placeholder'ов
- Surfaces: `reaction/mask.go`, `mask/usecase.go` (MaskFromResults)
- Наблюдение: текст содержит `{{ <UUIDv7> }}`, MaskStorage.Get возвращает оригинал

### AC-004 AlertReaction

- Подход: unit-тест с mock IncidentRepository
- Surfaces: `reaction/alert.go`
- Наблюдение: `IncidentRepository.Save` вызван для каждого incident-а, текст не изменён

### AC-005 ReactionPipeline routing

- Подход: unit-тест с mock executor'ами, проверка вызова по Reaction
- Surfaces: `reaction/pipeline.go`
- Наблюдение: для ReactionBlock → ошибка; ReactionLog → изменённый текст; ReactionReview → лог + оригинал

## Данные и контракты

- Data model: не меняется (сущности MaskEntry, Incident, ScanResult, Reaction остаются)
- Sentinel error: `ErrBlockedByPolicy` добавляется в shield/errors
- API handler: сигнатура `HandleMask` меняется (использует registry.ScanAll + MaskFromResults)
- MaskUseCase: `MaskText` удаляется, `MaskFromResults` добавляется (breaking change для вызывающих, но только handler использует)

## Стратегия реализации

### DEC-001 ReactionExecutor как interface с единой сигнатурой

  Why: все executor'ы делают одно и то же (получают ScanResult + текст, возвращают изменённый текст + ошибку). Единый контракт позволяет Pipeline работать с любым executor'ом без type switch.
  Tradeoff: если в будущем понадобятся executor'ы с разными входами — придётся расширять контракт.
  Affects: `reaction/executor.go`
  Validation: все 4 реализации компилируются под один интерфейс.

### DEC-002 MaskFromResults принимает []detector.DetectorResult вместо самостоятельного сканирования

  Why: MaskReaction уже имеет результаты сканирования (из ScanResult), повторный запуск детекторов — лишняя работа и потенциально разные результаты.
  Tradeoff: вызывающий (API handler) должен сам запустить детекторы до вызова.
  Affects: `mask/usecase.go`, `api/mask_handler.go`
  Validation: handler вызывает registry.ScanAll → MaskFromResults; MaskReaction конвертирует Incident → DetectorResult → MaskFromResults.

### DEC-003 BlockReaction возвращает domain error, не HTTP-ответ

  Why: reaction — domain-слой, не должен знать об HTTP. Ошибка с ErrBlockedByPolicy и причиной всплывает до handler, который решает, как её маппить (403).
  Tradeoff: handler должен проверять `errors.Is(err, ErrBlockedByPolicy)` для 403.
  Affects: `reaction/block.go`, `errors/errors.go`
  Validation: unit-тест проверяет `errors.Is`, handler integration test проверяет 403.

### DEC-004 ReactionPipeline interface + одна default реализация

  Why: по spec (RQ-008). Позволяет adapter'ам оборачивать pipeline (логирование, метрики) без изменения domain кода.
  Tradeoff: дополнительная косвенность, хотя сейчас только одна реализация.
  Affects: `reaction/pipeline.go`
  Validation: тест с mock pipeline (подмена через interface).

## Incremental Delivery

### MVP (Block + Redact + Alert + Pipeline)

1. Refactoring: `MaskFromResults` + миграция handler'а + sentinel error
2. `reaction/` package: executor interface + BlockReaction
3. RedactReaction
4. AlertReaction
5. ReactionPipeline + AC-005 тест

Критерий: unit-тесты AC-001, AC-002, AC-004, AC-005 проходят.

### Итеративное расширение (P3)

6. MaskReaction + AC-003 тест

## Порядок реализации

1. **Step 0 (bootstrapping)**: `ErrBlockedByPolicy` в shield/errors + создать `reaction/` package
2. **Step 1**: Refactoring — `MaskFromResults`, handler migration, удаление `MaskText`
3. **Step 2-6 in parallel**: Block, Redact, Alert, + Pipeline (независимые)
4. **Step 7 (P3)**: MaskReaction

Steps 2-6 можно параллелить, т.к. executor'ы не зависят друг от друга. Step 1 — gate для всей фичи (иначе handler сломан).

## Риски

- **MaskFromResults дублирует логику MaskText**: низкий — вынос replace + save из MaskText в MaskFromResults, MaskText становится обёрткой scan → MaskFromResults, затем удаляется. Mitigation: рефакторинг за 1 коммит.
- **Incident не содержит позицию фрагмента (position всегда 0 в ScanPipeline)**: средний — ScanPipeline.Execute не заполняет position. Если Incident.position == 0, RedactReaction не сможет заменить по позиции. Mitigation: либо RedactReaction ищет фрагмент в тексте (strings.Index), либо SC-001 acceptance дополняется. В spec Допущения: "Фрагменты не перекрываются" — можно искать по `strings.Index`.
- **MaskFromResults меняет формат placeholder'а**: MaskText использует `{{maskID.counter}}`, spec хочет `{{ UUIDv7 }}`. MaskFromResults может использовать старый или новый формат. Mitigation: MaskFromResults использует `{{maskID.counter}}` (как MaskText), MaskReaction через него будет получать тот же формат. Если нужен `{{ UUIDv7 }}` — формат обсуждается отдельно.

## Rollout и compatibility

- `MaskText` удаляется — breaking change для кого-то, кто вызывает его напрямую (только handler, мигрируется в том же PR)
- Новый sentinel error `ErrBlockedByPolicy` — additive change, не ломает существующий код
- `reaction/` — новый пакет, не влияет на существующий

## Проверка

- Unit-тесты: каждый executor, pipeline routing (AC-001 — AC-005)
- MaskFromResults: unit-тест с in-memory storage (заимствовать из usecase_test.go)
- API handler: integration test (запрос POST /api/v1/shield/mask → 200 с маскированным текстом)
- Lint + build: `make lint && go build ./...`

## Соответствие конституции

- нет конфликтов
