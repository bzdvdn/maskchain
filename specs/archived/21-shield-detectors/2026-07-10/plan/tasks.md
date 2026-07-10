# Базовые детекторы Content Shield — Задачи

## Phase Contract

Inputs: plan.md, data-model.md, spec.md.
Outputs: исполнимые задачи с Touches и покрытием AC.
Stop if: покрытие AC не удаётся сопоставить задачам — все 12 AC покрыты.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/domain/shield/detector/detector.go` | T1.1 |
| `src/internal/domain/shield/detector/piidetector.go` | T2.1 |
| `src/internal/domain/shield/detector/piidetector_test.go` | T2.2 |
| `src/internal/domain/shield/detector/secretsdetector.go` | T3.1 |
| `src/internal/domain/shield/detector/secretsdetector_test.go` | T3.1 |
| `src/internal/domain/shield/detector/financialdetector.go` | T3.2 |
| `src/internal/domain/shield/detector/financialdetector_test.go` | T3.2 |
| `src/internal/domain/shield/detector/phidetector.go` | T3.3 |
| `src/internal/domain/shield/detector/phidetector_test.go` | T3.3 |
| `src/internal/domain/shield/detector/registry.go` | T1.2 |

## Implementation Context

- **Цель MVP:** Detector interface + PII detector + тесты — покрывает 7 из 12 AC (AC-001–003, AC-009–012).
- **Инварианты/семантика:**
  - `Detector.Scan(ctx, text)` возвращает `[]DetectorResult`; пустой слайс (не nil) при отсутствии совпадений.
  - `DetectorResult`: DetectorType (string), Fragment, StartPos, EndPos, Confidence [0.0, 1.0].
  - Regex compilation — в конструкторе (fail-fast). Panic в `Scan()` запрещён.
  - Luhn — package-private функция в financialdetector.go.
  - Registry — sync.Map, методы `Register(DetectorType, Detector)` и `Get(DetectorType) Detector`.
- **Ошибки/коды:** детекторы возвращают error только при отменённом ctx; логические ошибки — пустой результат.
- **Границы scope:** не трогаем entity.Detector, entity.Pattern, entity.Incident и любые persistence-слои.
- **Proof signals:** `go test ./src/internal/domain/shield/detector/... -v` зелёный; compile-time assertion `var _ Detector = (*PIIDetector)(nil)`.

## Фаза 1: Основа

Цель: Detector interface, DetectorResult, Registry. Без этого ни один детектор не работает.

- [x] T1.1 Определить Detector interface и DetectorResult struct.
  - `Detector` с методом `Scan(ctx context.Context, text string) ([]DetectorResult, error)`
  - `DetectorResult` с полями: DetectorType string, Fragment string, StartPos int, EndPos int, Confidence float64
  - Пакет: `package detector`, файл `detector.go`
  - Touches: `src/internal/domain/shield/detector/detector.go`
  - AC: AC-001, AC-002
  - DEC: DEC-001, DEC-002

- [x] T1.2 Реализовать DetectorRegistry.
  - Thread-safe registry на sync.Map
  - Методы: `Register(typ DetectorType, d Detector)`, `Get(typ DetectorType) Detector`, `Types() []DetectorType`
  - Файл: `registry.go` + `registry_test.go`
  - Touches: `src/internal/domain/shield/detector/registry.go`
  - AC: AC-008
  - DEC: DEC-003

## Фаза 2: PII детектор (MVP)

Цель: первый работающий детектор. Подтверждает архитектуру (interface + struct + registry + тесты).

- [x] T2.1 Реализовать PIIDetector.
  - Regex: email, phone (международный), SSN (###-##-####), passport РФ (XX XXXX XXX)
  - Compile в NewPIIDetector(), Scan() использует готовые regexps
  - `var _ Detector = (*PIIDetector)(nil)` compile assertion
  - Файл: `piidetector.go`
  - Touches: `src/internal/domain/shield/detector/piidetector.go`
  - AC: AC-001, AC-003
  - DEC: DEC-004, DEC-006

- [x] T2.2 Добавить unit-тесты PIIDetector.
  - Основной сценарий: email, phone, SSN, passport — 4 результата, корректные фрагменты (AC-003)
  - Пустой ввод → пустой слайс, не nil (AC-009)
  - Спецсимволы без паники (AC-010)
  - Confidence=1.0 для всех точных совпадений (AC-011)
  - StartPos/EndPos → text[StartPos:EndPos] == Fragment (AC-012)
  - Частичные совпадения, пересекающиеся паттерны
  - Файл: `piidetector_test.go`
  - Touches: `src/internal/domain/shield/detector/piidetector_test.go`
  - AC: AC-003, AC-009, AC-010, AC-011, AC-012

## Фаза 3: Secrets, Financial, PHI детекторы

Цель: оставшиеся 3 детектора. Каждый независим, можно параллелить.

- [x] T3.1 Реализовать SecretsDetector + тесты.
  - API-ключи (sk-*, pk-*), Bearer-токены, JWT (3 сегмента base64), PEM-блоки
  - Тесты: AC-004 (3 результата), пустой ввод, спецсимволы, confidence, позиции
  - Файлы: `secretsdetector.go`, `secretsdetector_test.go`
  - Touches: `src/internal/domain/shield/detector/secretsdetector.go`, `src/internal/domain/shield/detector/secretsdetector_test.go`
  - AC: AC-001, AC-004, AC-009, AC-010, AC-011, AC-012

- [x] T3.2 Реализовать FinancialDetector (Luhn) + тесты.
  - Номера карт (13–19 цифр) с Luhn-проверкой, IBAN regex, SWIFT regex
  - Luhn — package-private функция, отдельно протестирована
  - Тесты: AC-005 (valid карта + IBAN + SWIFT), AC-006 (invalid Luhn не проходит)
  - Файлы: `financialdetector.go`, `financialdetector_test.go`
  - Touches: `src/internal/domain/shield/detector/financialdetector.go`, `src/internal/domain/shield/detector/financialdetector_test.go`
  - AC: AC-001, AC-005, AC-006, AC-009, AC-010, AC-011, AC-012
  - DEC: DEC-005

- [x] T3.3 Реализовать PHIDetector + тесты.
  - ICD-10 коды: буква + две цифры, опционально точка + 1–2 цифры
  - Тесты: AC-007 (3 кода), пустой ввод, спецсимволы
  - Файлы: `phidetector.go`, `phidetector_test.go`
  - Touches: `src/internal/domain/shield/detector/phidetector.go`, `src/internal/domain/shield/detector/phidetector_test.go`
  - AC: AC-001, AC-007, AC-009, AC-010, AC-011, AC-012

## Фаза 4: Проверка

Цель: финальный прогон, покрытие, review.

- [x] T4.1 Финальный прогон тестов и проверка покрытия.
  - `go test ./src/internal/domain/shield/detector/... -v -count=1` — зелёный
  - `go test ./src/internal/domain/shield/detector/... -cover` — >80%
  - `go vet ./src/internal/domain/shield/detector/...` — чистый
  - `golangci-lint run ./src/internal/domain/shield/detector/...` — чистый
  - Проверка `@sk-task`/`@sk-test` маркеров над всеми owning declarations
  - Touches: все файлы пакета
  - AC: все

## Покрытие критериев приемки

| AC | Задачи | DEC |
|----|--------|-----|
| AC-001 | T1.1, T2.1, T3.1, T3.2, T3.3 | DEC-001, DEC-002 |
| AC-002 | T1.1 | DEC-002 |
| AC-003 | T2.1, T2.2 | DEC-004, DEC-006 |
| AC-004 | T3.1 | DEC-004, DEC-006 |
| AC-005 | T3.2 | DEC-005 |
| AC-006 | T3.2 | DEC-005 |
| AC-007 | T3.3 | DEC-004, DEC-006 |
| AC-008 | T1.2 | DEC-003 |
| AC-009 | T2.2, T3.1, T3.2, T3.3 | — |
| AC-010 | T2.2, T3.1, T3.2, T3.3 | — |
| AC-011 | T2.2, T3.1, T3.2, T3.3 | — |
| AC-012 | T2.2, T3.1, T3.2, T3.3 | — |

## Заметки

- T1.1 и T1.2 можно выполнять параллельно (независимы)
- T2.1 → T2.2 обязательный порядок (сначала impl, потом тесты)
- T3.1, T3.2, T3.3 независимы — можно параллелить после T1.x
- T4.1 — финальная, после завершения всех остальных
- Trace-маркеры: `@sk-task 21-shield-detectors#T<N>.<M>` над каждой функцией/типом; `@sk-test 21-shield-detectors#T<N>.<M>` над каждым тестом. Запрещены на уровне package/import/file-header.
