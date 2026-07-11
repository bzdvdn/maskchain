# Shield Engine — Orchestrator сканирования: План

## Phase Contract

Inputs: spec, inspect (pass), минимальный repo-контекст.
Outputs: plan.md, data-model.md.
Stop if: нет.

## Цель

Реализовать оркестратор `ShieldEngine` в `app/usecase/shield/`, который по запросу (текст + profile slug) загружает профиль, строит цепочку препроцессоры → детекторы (включая словари) → PolicyEvaluator → ReactionPipeline, и возвращает результат с обработанным текстом и картой подстановок. Существующие domain-компоненты не меняются — используется только их композиция.

## MVP Slice

- `ScanPipelineFactory` — построение цепочки из конфигурации Profile.
- `ShieldEngine.Scan` — полный цикл: load profile → build pipeline → execute preprocessors → execute dictionary detection → execute regex detectors → evaluate policy → execute reaction → return result.
- Обработка ошибок: profile not found, disabled profile, empty pipeline.
- Покрытие: AC-001, AC-004, AC-005.

## First Validation Path

Один `go test` в `src/internal/app/usecase/shield/shield_engine_test.go`, который:
1. Создаёт in-memory профиль с CSV-препроцессором, словарём, PII-детектором и реакцией redact.
2. Вызывает `ShieldEngine.Scan` с текстом, содержащим CSV-строку и ключевое слово из словаря.
3. Проверяет `ScanResult.Status`, `len(Incidents) >= 2`, финальный текст без sensitive-данных.

## Scope

- `src/internal/app/usecase/shield/` — новые файлы: types, errors, engine, pipeline factory, use cases.
- `src/internal/app/usecase/shield/*_test.go` — unit + integration тесты.
- Существующий `domain/shield/service/ScanPipeline` не используется и не меняется.

## Performance Budget

- SC-001: полный цикл < 100ms для 1KB текста (цель, не жёсткий порог; на plan: target 200ms для 1st pass).
- `none` critical memory constraints — типовой сценарий оперирует строками < 1MB.

## Implementation Surfaces

| Surface | Тип | Зачем |
|---|---|---|
| `app/usecase/shield/types.go` | new | ScanRequest, ScanResponse DTO |
| `app/usecase/shield/errors.go` | new | ErrProfileNotFound, ErrProfileDisabled sentinels |
| `app/usecase/shield/pipeline_factory.go` | new | ScanPipelineFactory — строит пайплайн из Profile |
| `app/usecase/shield/scan_usecase.go` | new | ScanUseCase — оркестрация полного цикла |
| `app/usecase/shield/shield_engine.go` | new | ShieldEngine — public API, фасад над ScanUseCase |
| `app/usecase/shield/apply_policy_usecase.go` | new | ApplyPolicyUseCase — PolicyEvaluator на готовом ScanResult |
| `app/usecase/shield/*_test.go` | new | Интеграционный тест полного пайплайна |

## Bootstrapping Surfaces

- `mkdir -p src/internal/app/usecase/shield/`

## Влияние на архитектуру

- Локальное: новый пакет `app/usecase/shield/` — чистый слой application orchestration.
- Domain не затронут: ни один существующий файл не меняется.
- Существующий `domain/shield/service/ScanPipeline` становится deprecated (но не удаляется — может потребоваться другим потребителям).

## Acceptance Approach

- AC-001: `ShieldEngine.Scan` с профилем (CSV-preprocessor + dictionary + PII-detector + redact). Surfaces: pipeline_factory, scan_usecase, shield_engine. Проверка: интеграционный тест.
- AC-002: Mask-реакция с Replacements. Surfaces: scan_usecase (сохранение карты подстановок). Проверка: unit-тест с in-memory профилем.
- AC-003: `ApplyPolicyUseCase.Execute` на готовом ScanResult. Surfaces: apply_policy_usecase. Проверка: unit-тест с 3 инцидентами разной severity.
- AC-004: Неизвестный slug → `ErrProfileNotFound`. Surfaces: scan_usecase (проверка после ProfileRepository.FindBySlug). Проверка: unit-тест с `errors.Is`.
- AC-005: Пустой пайплайн → status=clean. Surfaces: pipeline_factory (возвращает пустой pipeline), scan_usecase (short-circuit). Проверка: unit-тест.
- AC-006: Все 4 формата placeholders в одном результате. Surfaces: scan_usecase (консолидация replacements со всех этапов). Проверка: интеграционный тест с CSV + JSON + detector + dictionary.

## Данные и контракты

- Domain data model не меняется. `entity.ScanResult` остаётся без изменений.
- Создаются app-level DTO:
  - `ScanRequest{Text string, ProfileSlug string}`
  - `ScanResponse{*entity.ScanResult, ProcessedText string, Replacements map[string]string}`
- API-контракты: ShieldEngine.Scan — внутренний Go API, не HTTP/gRPC-контракт. Rest API изменится только в следующей фазе (gateway handler).
- `data-model.md`: no-change stub (см. отдельный файл).

## Стратегия реализации

### DEC-001: App-level ScanResponse вместо расширения domain ScanResult

Why: domain `entity.ScanResult` чистый — только статус, инциденты, время. Добавление ProcessedText/Replacements в domain нарушило бы его ответственность. App-level wrapper позволяет расширять ответ без касания domain.
Tradeoff: клиент получает два уровня — ScanResult (domain) + метаданные обработки (app).
Affects: `app/usecase/shield/types.go`, `app/usecase/shield/scan_usecase.go`
Validation: тест AC-002 проверяет наличие Replacements в ScanResponse.

### DEC-002: ScanRequest как app-level DTO

Why: `ScanRequest` не содержит инвариантов и бизнес-правил. Класть его в domain означало бы раздувать entity слоя ради pure transport struct.
Tradeoff: нет.
Affects: `app/usecase/shield/types.go`
Validation: компиляция.

### DEC-003: ErrProfileDisabled отдельно от ErrProfileNotFound

Why: два разных состояния ошибки — "нет такого профиля" (постоянная ошибка конфигурации) и "профиль отключён" (временное состояние). Смешивать их мешает корректной обработке на стороне клиента.
Tradeoff: клиент должен обрабатывать две ошибки вместо одной.
Affects: `app/usecase/shield/errors.go`, `app/usecase/shield/scan_usecase.go`
Validation: AC-004 проверяет ErrProfileNotFound; отдельный тест проверяет ErrProfileDisabled.

### DEC-004: Новый app-level пайплайн, не переиспользовать domain `service.ScanPipeline`

Why: существующий domain `ScanPipeline` использует `matchContent` (string contains), не поддерживает regex-детекторы, словари и препроцессоры. Его адаптация сложнее написания новой композиции на app-уровне.
Tradeoff: небольшое дублирование логики прохода по детекторам. НО новый код вызывает `Detector.Scan(ctx, text)` (интерфейс domain), а не копирует логику.
Affects: `app/usecase/shield/pipeline_factory.go`, `app/usecase/shield/scan_usecase.go`
Validation: AC-001 проверяет, что PII-детектор (regex) и словарь (dictionary) оба находят инциденты — это невозможно через старый domain ScanPipeline.

### DEC-005: Нет кэширования пайплайна в MVP

Why: каждое сканирование строит пайплайн заново из профиля. Кэш добавит сложности с инвалидацией при обновлении профиля. Если профили загружаются часто, ProfileRepository сам может кэшировать.
Tradeoff: extra allocs на каждый Scan. Компенсация: построение пайплайна — дешёвая операция (создание срезов детекторов).
Affects: нет (решение отложить).
Validation: SC-001 (если не укладывается в 200ms, пересмотреть).

## Incremental Delivery

### MVP (Первая ценность)

- Bootstrapping: `types.go`, `errors.go`, `pipeline_factory.go`
- `ShieldEngine.Scan` с полным пайплайном — препроцессоры → словари → детекторы → PolicyEvaluator → ReactionPipeline
- Поддержка реакций: block, redact, mask (Replacements в MVP не обязателен)
- Обработка ошибок: profile not found, disabled profile, empty pipeline
- Покрывает: AC-001 (redact без проверки placeholders), AC-004, AC-005
- Валидация: интеграционный тест shield_engine_test.go

### Итеративное расширение

1. **Placeholder-based masking** — консолидация карты подстановок со всех этапов, все 4 формата (`{{csv.*}}`, `{{json.*}}`, `{{p.*}}`, `{{dict.*}}`).
   - Покрывает: AC-002, AC-006
   - Валидация: тест проверяет Replacements и форматы placeholders
2. **ApplyPolicyUseCase** — отдельный use case для переоценки политики.
   - Покрывает: AC-003
   - Валидация: unit-тест с 3 инцидентами разной severity

## Порядок реализации

1. `types.go` + `errors.go` — база (нет зависимостей)
2. `pipeline_factory.go` — строит пайплайн (зависит от domain/interfaces)
3. `scan_usecase.go` — оркестрация (зависит от pipeline_factory + ProfileRepository + domain components)
4. `shield_engine.go` — public API фасад (зависит от scan_usecase)
5. `shield_engine_test.go` — интеграционный тест (зависит от shield_engine)
6. Placeholder-расширение scan_usecase (добавление Replacements)
7. `apply_policy_usecase.go` + тест
8. AC-006 тест (все 4 формата)

Параллельно: ничего. Все шаги последовательны.

## Риски

- Риск: существующий `reaction.DefaultReactionPipeline` не имеет маппинга для `entity.Reaction` → `ReactionExecutor` для mask-реакции (есть только block, redact, alert).  
  Mitigation: MaskReactionExecutor уже реализован в `reaction/mask.go`. В `DefaultReactionPipeline` может не быть его маппинга — тогда нужно либо расширить DefaultReactionPipeline, либо вызывать MaskReaction напрямую. Решение: на implement-фазе — проверить и при необходимости передать maskExecutor в DefaultReactionPipeline или обернуть вызов.

- Риск: `entity.ScanResult` не содержит информации о найденных фрагментах для построения placeholders (только `Incident.Fragment`).  
  Mitigation: достаточно — `Incident.Fragment` содержит совпавший текст. Детекторы возвращают `DetectorResult.Fragment` и позиции. ScanUseCase аккумулирует их.

## Rollout и compatibility

- Специальных rollout-действий не требуется: `ShieldEngine` — новый код, не меняет существующее поведение.
- Существующий `domain/shield/service/ScanPipeline` остаётся на месте без изменений для обратной совместимости.

## Проверка

| Что | Тип | AC/DEC |
|---|---|---|
| `shield_engine_test.go` — полный пайплайн | integration | AC-001, AC-005 |
| `shield_engine_test.go` — profile not found | unit | AC-004 |
| `shield_engine_test.go` — placeholder masking + replacements | unit | AC-002, AC-006 |
| `apply_policy_usecase_test.go` — 3 инцидента разной severity | unit | AC-003 |
| `shield_engine_test.go` — disabled profile error | unit | AC-004 variant |
| `pipeline_factory_test.go` — построение из пустого профиля | unit | AC-005 |

## Соответствие конституции

- нет конфликтов.
