# Prompt Injection Shield Задачи

## Phase Contract

Inputs: plan (`prompt-injection-shield`), spec, data-model (no-change), repo surfaces (entity/detector_type.go, detector/piidetector.go, cmd/gateway/run.go).
Outputs: исполнимые задачи с фазами, Touches, AC mapping.
Stop if: — (plan чёток, surfaces известны).

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/domain/shield/entity/detector_type.go` | T1.1 |
| `src/internal/domain/shield/detector/promptinjectiondetector.go` | T1.2, T4.1 |
| `src/internal/domain/shield/detector/promptinjectiondetector_test.go` | T2.1, T2.2 |
| `src/internal/domain/shield/detector/registry_test.go` | T2.2 |
| `src/internal/domain/shield/service/service_test.go` | T3.1 |
| `src/cmd/gateway/run.go` | T3.2 |
| `src/cmd/all/gateway.go` | T3.2 |
| `src/internal/domain/shield/detector/dictionary_detector.go` | T4.1 (pattern reference) |
| `src/internal/domain/shield/detector/piidetector.go` | T1.2 (pattern reference) |

## Implementation Context

- Цель MVP: детектор `prompt_injection` зарегистрирован, обнаруживает built-in injection-фразы, не даёт false positives на чистом тексте.
- Инварианты/семантика:
  - PromptInjectionDetector имплементирует `detector.Detector` (один метод `Scan(ctx, text)`)
  - Built-in patterns — строковые литералы в Go-коде (не БД, не конфиг)
  - Tenant-level override: конструктор принимает опциональный `[]entity.Pattern`, мержит с built-in при Scan
  - Матчинг: `strings.Contains` (case-insensitive) для MVP, без regex
  - `Confidence` всегда 1.0 в MVP
  - При совпадении tenant pattern с тем же текстом, что и built-in — tenant wins (dedup по `Pattern.ID()`)
- Ошибки/коды: `Scan()` возвращает `[]DetectorResult, error`. Ошибки только при панике в pattern match (не ожидается в MVP).
- Контракты/протокол: `DetectorType` enum — новая константа `DetectorTypePromptInjection = "prompt_injection"`.
- Границы scope: не делаем config-секцию (PostMVP), не делаем regex patterns (PostMVP), не делаем ML/heuristic, не меняем entity.Detector/Pattern/ScanPipeline.
- Proof signals: `go test ./src/internal/domain/shield/detector/ -run TestPromptInjection -v` проходит, `registry.Types()` содержит `"prompt_injection"`.
- References: `DEC-001`, `DEC-002`, `DEC-003`, `DM: no-change`.

## Фаза 1: Основа

Цель: добавить константу DetectorTypePromptInjection и реализовать PromptInjectionDetector struct с built-in patterns и Scan().

- [x] T1.1 Добавить `DetectorTypePromptInjection DetectorType = "prompt_injection"` в `entity/detector_type.go`.
  Touches: `src/internal/domain/shield/entity/detector_type.go`
  AC: AC-001 (предусловие для регистрации)

- [x] T1.2 Создать `src/internal/domain/shield/detector/promptinjectiondetector.go`:
  - struct `PromptInjectionDetector` с полями `builtin []patternEntry` и `tenantPatterns []entity.Pattern`
  - `patternEntry` — внутренняя структура: `fragment string, description string`
  - `NewPromptInjectionDetector(tenantPatterns ...entity.Pattern) *PromptInjectionDetector` — конструктор, загружает ≥ 20 built-in patterns
  - `BuiltinPatterns() []patternEntry` — экспортируемый доступ для тестов
  - `Scan(ctx, text) ([]DetectorResult, error)` — итерация по builtin + tenant patterns, `strings.Contains(text, fragment)` (case-insensitive)
  - Dedup: если tenant pattern с тем же fragment уже есть в built-in, built-in исключается
  - 20+ built-in patterns, покрывающие OWASP LLM01 категории: direct (ignore previous instructions), DAN, role-playing (you are now), system prompt extraction, payload splitting
  Touches: `src/internal/domain/shield/detector/promptinjectiondetector.go`
  AC: AC-002, AC-003, AC-004
  DEC: DEC-001, DEC-002, DEC-003

## Фаза 2: MVP Slice

Цель: unit-тесты подтверждают регистрацию, детекцию, отсутствие false positives и ≥ 20 built-in patterns.

- [x] T2.1 Создать `src/internal/domain/shield/detector/promptinjectiondetector_test.go`:
  - `TestPromptInjectionDetector_Scan_DetectsKnownInjection` — AC-002: текст с "ignore previous instructions" → результат не пуст, Fragment совпадает
  - `TestPromptInjectionDetector_Scan_CleanText` — AC-003: "what is the weather in London?" → пустой результат
  - `TestPromptInjectionDetector_BuiltinPatterns` — AC-004: `NewPromptInjectionDetector().BuiltinPatterns() >= 20`
  Touches: `src/internal/domain/shield/detector/promptinjectiondetector_test.go`
  AC: AC-002, AC-003, AC-004

- [x] T2.2 Добавить тест-кейс в `registry_test.go`: регистрация `DetectorTypePromptInjection` → `registry.Types()` содержит `"prompt_injection"`.
  Touches: `src/internal/domain/shield/detector/registry_test.go`
  AC: AC-001

## Фаза 3: Интеграция

Цель: PromptInjectionDetector подключён к ScanPipeline и зарегистрирован в DI gateway.

- [x] T3.1 Добавить тест в `service_test.go`:
  - ScanPipeline с PromptInjectionDetector (pattern severity=critical) + injection text → `Status() == ScanStatusBlocked`
  - ScanPipeline с PromptInjectionDetector (pattern severity=medium) + injection text → `Status() == ScanStatusSuspicious`
  Touches: `src/internal/domain/shield/service/service_test.go`
  AC: AC-005

- [x] T3.2 Зарегистрировать PromptInjectionDetector в `cmd/gateway/run.go` (функция `initDetectors`) и `cmd/all/gateway.go`:
  - Вызвать `detector.NewPromptInjectionDetector()`
  - Зарегистрировать в registry: `registry.Register(entity.DetectorTypePromptInjection, detector)`
  - Аналогично существующему PIIDetector — отдельный registry entry (не composite)
  Touches: `src/cmd/gateway/run.go`, `src/cmd/all/gateway.go`
  AC: AC-005 (полный цикл)

## Фаза 4: Tenant override

Цель: tenant-level override patterns работают через существующий механизм entity.Detector.

- [x] T4.1 Добавить поддержку tenant patterns в PromptInjectionDetector:
  - Конструктор принимает `tenantPatterns ...entity.Pattern`
  - Scan() мержит builtin + tenant patterns
  - Tenant pattern с тем же текстом переопределяет built-in
  - Добавить тест: два набора patterns → разные результаты для одного текста
  Touches: `src/internal/domain/shield/detector/promptinjectiondetector.go`, `src/internal/domain/shield/detector/promptinjectiondetector_test.go`
  AC: AC-006
  DEC: DEC-003

## Покрытие критериев приемки

- AC-001 → T1.1, T2.2
- AC-002 → T1.2, T2.1
- AC-003 → T1.2, T2.1
- AC-004 → T1.2, T2.1
- AC-005 → T3.1, T3.2
- AC-006 → T4.1

## Заметки

- T1.1 и T1.2 можно выполнять параллельно (разные файлы, нет зависимости).
- T2.1 и T2.2 — после T1.2 (тесты зависят от реализации).
- T3.1 — после T2.1 (тест ScanPipeline требует существующего детектора).
- T3.2 — после T2.2 (DI wiring требует registry registration).
- T4.1 — после T3.2 (tenant override тест требует полного цикла).
- Шаги 1-2 (T1.1 .. T2.2) можно в одном PR. Шаг 3 (T3.1..T3.2) — второй PR. Шаг 4 (T4.1) — третий PR или вместе с шагом 3.
