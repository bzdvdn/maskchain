# Shield Engine — Orchestrator сканирования: Задачи

## Phase Contract

Inputs: plan, spec, data-model (no-change stub).
Outputs: упорядоченные задачи с покрытием AC.
Stop if: нет.

## Surface Map

| Surface | Tasks |
|---|---|
| `app/usecase/shield/types.go` | T1.1 |
| `app/usecase/shield/errors.go` | T1.1 |
| `app/usecase/shield/pipeline_factory.go` | T2.1 |
| `app/usecase/shield/scan_usecase.go` | T2.1, T3.1 |
| `app/usecase/shield/shield_engine.go` | T2.1 |
| `app/usecase/shield/apply_policy_usecase.go` | T4.1 |
| `app/usecase/shield/shield_engine_test.go` | T2.2, T3.2 |
| `app/usecase/shield/apply_policy_usecase_test.go` | T4.2 |

## Implementation Context

- Цель MVP: `ShieldEngine.Scan(ctx, ScanRequest{Text, ProfileSlug})` выполняет полный цикл и возвращает `ScanResponse`, покрывая AC-001, AC-004, AC-005.
- Инварианты/семантика:
  - Порядок пайплайна: препроцессоры → словари (как `DictionaryDetector`) → regex-детекторы → `PolicyEvaluator` → `ReactionPipeline`
  - `ScanRequest{Text string, ProfileSlug string}` — app DTO, не domain.
  - `ScanResponse{*entity.ScanResult, ProcessedText string, Replacements map[string]string}` — app wrapper.
  - Ошибки: `ErrProfileNotFound` + `ErrProfileDisabled` — sentinel errors.
  - Пустой пайплайн (нет активных detectors + dictionaries) → short-circuit: `ScanStatusClean`.
  - Для mask-реакции (AC-002/006) вызывать `MaskReaction` напрямую вне `DefaultReactionPipeline`, так как в domain нет `ReactionMask` и маппинга в pipeline.
  - `DefaultReactionPipeline` маппинг: `allow→passthrough`, `block→Block`, `log→Redact`, `review→Alert`.
- Ошибки/коды:
  - `ErrProfileNotFound` — slug не найден в репозитории.
  - `ErrProfileDisabled` — профиль существует, но enabled=false.
- Контракты/протокол:
  - `ScanPipelineFactory.Build(ctx, *entity.Profile) -> ([]preprocessor.Processor, []detector.Detector, error)` — строит список процессоров и детекторов.
  - `ScanUseCase` принимает `ProfileRepository`, `PolicyEvaluator`, `ReactionPipeline`, опционально `MaskReaction`.
  - Для словарей: `detector.NewDictionaryDetector(dict)` оборачивает `*dictionary.Dictionary` в `detector.Detector`.
- Границы scope:
  - Не менять domain-интерфейсы (`Detector`, `Processor`, `ReactionExecutor`, entity).
  - Не трогать `domain/shield/service/ScanPipeline`.
  - Нет кэширования пайплайна (DEC-005).
- Proof signals:
  - go build && go test ./src/internal/app/usecase/shield/... проходит.
  - Интеграционный тест полной цепочки (AC-001) — красный до T2.1, зелёный после T2.2.
  - `errors.Is(err, ErrProfileNotFound)` — AC-004.
  - `errors.Is(err, ErrProfileDisabled)` — отдельный тест (suggestion из inspect).
- References: DEC-001, DEC-002, DEC-003, DEC-004, DM (no-change).

## Фаза 1: Bootstrapping

Цель: подготовить типы и ошибки, от которых зависят все последующие фазы.

- [x] T1.1 Создать `types.go` (ScanRequest, ScanResponse) и `errors.go` (ErrProfileNotFound, ErrProfileDisabled). ScanRequest содержит `Text` + `ProfileSlug`; ScanResponse оборачивает `*entity.ScanResult` + `ProcessedText` + `Replacements`. Touches: `app/usecase/shield/types.go`, `app/usecase/shield/errors.go`

## Фаза 2: MVP — полный пайплайн

Цель: реализовать ShieldEngine.Scan с полным циклом, обработкой ошибок и пустым пайплайном. Покрывает AC-001, AC-004, AC-005.

- [x] T2.1 Реализовать `ScanPipelineFactory` (Build — строит `[]preprocessor.Processor` + `[]detector.Detector` из Profile), `ScanUseCase` (оркестрация: load profile → pipeline → execute → evaluate → react → return), `ShieldEngine` (public API фасад с `Scan(ctx, ScanRequest) ScanResponse`). Обработка: profile not found → ErrProfileNotFound, profile disabled → ErrProfileDisabled, пустой pipeline → ScanStatusClean. Touches: `app/usecase/shield/pipeline_factory.go`, `app/usecase/shield/scan_usecase.go`, `app/usecase/shield/shield_engine.go`
- [x] T2.2 Написать интеграционный тест `shield_engine_test.go`: in-memory профиль с CSV-препроцессором, словарём (exact match), PII-детектором и реакцией redact → `ShieldEngine.Scan` → проверка статуса `suspicious`, `len(Incidents) >= 2`, финальный текст без sensitive-данных. Добавить тесты на ErrProfileNotFound и ErrProfileDisabled. Touches: `app/usecase/shield/shield_engine_test.go`

## Фаза 3: Placeholder-based masking

Цель: расширить ScanUseCase для консолидации карты подстановок со всех этапов с единой схемой `{{csv.*}}`, `{{json.*}}`, `{{p.*}}`, `{{dict.*}}`. Покрывает AC-002, AC-006.

- [x] T3.1 Реализовать консолидацию Replacements в ScanUseCase: аккумулировать замены от препроцессоров (уже в `ProcessResult.Replacements`), словарей и детекторов, формируя единую карту `map[string]string` с ключами по схеме `{{<type>.<ns>.<N>}}`. Для mask-реакции вызвать `MaskReaction` напрямую и дополнить Replacements. Touches: `app/usecase/shield/scan_usecase.go`
- [x] T3.2 Добавить тесты placeholder masking: AC-002 (Replacements содержит `{{csv.default.0}}` и восстановление текста), AC-006 (все четыре формата placeholders в одном сценарии: CSV + JSON + PII-detector + dictionary). Touches: `app/usecase/shield/shield_engine_test.go`

## Фаза 4: ApplyPolicyUseCase

Цель: реализовать отдельный use case для переоценки политики на готовом ScanResult. Покрывает AC-003.

- [x] T4.1 Реализовать `ApplyPolicyUseCase` с методом `Execute(ctx, *entity.ScanResult) entity.Reaction` — делегирует `PolicyEvaluator.Evaluate`. Touches: `app/usecase/shield/apply_policy_usecase.go`
- [x] T4.2 Написать unit-тест `apply_policy_usecase_test.go`: создать ScanResult с тремя инцидентами разной severity, проверить, что реакция соответствует highest severity. Touches: `app/usecase/shield/apply_policy_usecase_test.go`

## Покрытие критериев приемки

- AC-001 -> T2.1, T2.2
- AC-002 -> T3.1, T3.2
- AC-003 -> T4.1, T4.2
- AC-004 -> T2.1, T2.2
- AC-005 -> T2.1, T2.2
- AC-006 -> T3.1, T3.2

## Заметки

- Порядок строго последовательный: каждая фаза зависит от предыдущей.
- AC-002 и AC-006 объединены в одну фазу, так как обе работают с placeholder-механизмом.
- Domain-код не изменяется ни в одной фазе — все изменения в `app/usecase/shield/`.
- Риск с mask-реакцией (нет маппинга в DefaultReactionPipeline) решается вызовом MaskReaction напрямую из ScanUseCase при необходимости.
