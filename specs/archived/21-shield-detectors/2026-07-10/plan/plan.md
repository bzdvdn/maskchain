# Базовые детекторы Content Shield — План

## Phase Contract

Inputs: spec 21-shield-detectors, inspect pass.
Outputs: plan.md, data-model.md.
Stop if: spec слишком расплывчата для безопасного планирования — spec пройдена inspect с pass.

## Цель

Создать пакет `src/internal/domain/shield/detector/` с Detector interface, DetectorResult, DetectorRegistry и 4 concrete детектора (PII, secrets, financial, PHI). Пакет не затрагивает существующие entity/value — только добавляет слой сканирования поверх них. Все детекторы покрыты unit-тестами.

## MVP Slice

Detector interface + DetectorResult + PII detector + тесты. Покрывает AC-001, AC-002, AC-003, AC-009, AC-010, AC-011, AC-012. Проверяет базовую механику: интерфейс, результат, обнаружение PII, пустой ввод, спецсимволы, confidence, позиции.

## First Validation Path

```bash
go test ./src/internal/domain/shield/detector/... -run PII -v
```

Зелёный прогон PII-тестов подтверждает: интерфейс скомпилирован, registry работает, PII-детектор находит email/phone/SSN/passport, пустой ввод и спецсимволы не паникуют, позиции корректны.

## Scope

- `src/internal/domain/shield/detector/` — новый пакет, все файлы
- entity.DetectorType (detector_type.go) — используется для типизации, не меняется
- Никакие entity/value/profile/incident не затрагиваются

## Performance Budget

- `none`: детекторы stateless, regex-сканирование на типичном промпте (<4KB) — <1ms. Performance-тестирование вынесено в shield-benchmark (PostMVP).

## Implementation Surfaces

| Surface | Статус | Участие |
|---------|--------|---------|
| `src/internal/domain/shield/detector/detector.go` | new | Detector interface, DetectorResult struct |
| `src/internal/domain/shield/detector/registry.go` | new | DetectorRegistry |
| `src/internal/domain/shield/detector/piidetector.go` | new | PII regexes |
| `src/internal/domain/shield/detector/secretsdetector.go` | new | Secrets patterns |
| `src/internal/domain/shield/detector/financialdetector.go` | new | Financial + Luhn |
| `src/internal/domain/shield/detector/phidetector.go` | new | ICD-10 patterns |
| `src/internal/domain/shield/detector/*_test.go` | new | Unit-тесты для каждого детектора |
| `src/internal/domain/shield/entity/detector_type.go` | existing (read) | Используется для типизации, не меняется |

## Bootstrapping Surfaces

- Создать директорию `src/internal/domain/shield/detector/` (единственная bootstrapping операция)

## Влияние на архитектуру

- Локальное: новый пакет внутри domain/shield. Никакие границы системы не пересекаются.
- Детекторы stateless — не влияют на lifecycle приложения.
- Registry — in-memory, инициализируется при старте (руками или через wire-up).

## Acceptance Approach

| AC | Подход | Surfaces | Наблюдение |
|----|--------|----------|------------|
| AC-001 | Определить Detector interface, все детекторы реализуют | detector.go, каждый *detector.go | `var _ Detector = (*PIIDetector)(nil)` compile check |
| AC-002 | DetectorResult struct с публичными полями | detector.go | Assert на каждом поле в тесте |
| AC-003 | PII regexes, тест с email/phone/SSN/passport | piidetector.go, piidetector_test.go | `len(result)==4`, confidence=1.0 |
| AC-004 | Secrets regexes, тест с API key/JWT/PEM | secretsdetector.go, secretsdetector_test.go | `len(result)==3`, ожидаемые типы |
| AC-005 | Financial regexes + Luhn, тест с valid картой/IBAN/SWIFT | financialdetector.go, financialdetector_test.go | `len(result)==3` |
| AC-006 | Luhn-невалидный номер не проходит | financialdetector.go, финансовый тест | result не содержит "credit_card" |
| AC-007 | PHI regex (ICD-10), тест с A00.0/B99.9/J45.0 | phidetector.go, phidetector_test.go | `len(result)==3` |
| AC-008 | Registry: register + get | registry.go | Get известного ≠ nil, Get неизвестного = nil |
| AC-009 | Пустой ввод → `[]DetectorResult{}` | все детекторы | `len(result)==0`, result ≠ nil |
| AC-010 | Спецсимволы без паники | все детекторы | Вызов завершается без panic |
| AC-011 | Точные regex → confidence=1.0 | detector.go | Assert на каждом результате |
| AC-012 | StartPos/EndPos корректны | piidetector.go | `text[StartPos:EndPos] == expected` |

## Данные и контракты

- AC: см. таблицу выше.
- Data model: DetectorResult — новая in-memory структура. Не persisted. Существующие entity не меняются. См. `data-model.md`.
- API/event contracts: не затрагиваются — фича внутри domain слоя.

## Стратегия реализации

### DEC-001 Detector.Scan принимает context.Context

- **Why:** единообразие с downstream (mask pipeline будет требовать ctx для timeout/cancellation). C Go-конвенцией.
- **Tradeoff:** небольшой overhead передачи ctx, даже когда cancellation не используется. Оправдан совместимостью.
- **Affects:** detector.go, все детекторы, тесты.
- **Validation:** `var _ Detector = (*PIIDetector)(nil)` в каждом файле.

### DEC-002 DetectorResult — struct, не interface

- **Why:** фиксированный набор полей (DetectorType, Fragment, StartPos, EndPos, Confidence). Никакой полиморфной вариации. Struct дешевле и проще тестировать (value equality).
- **Tradeoff:** расширение полей — breaking change. На данном этапе поля стабильны.
- **Affects:** detector.go.
- **Validation:** тесты AC-002 проверяют все поля.

### DEC-003 Registry — sync.Map

- **Why:** потокобезопасность без external lock. Registry будет вызываться из middleware (concurrent requests).
- **Tradeoff:** нет типизации ключей (interface{}). Компенсируется обёрткой с типизированным `Get(DetectorType)`.
- **Affects:** registry.go.
- **Validation:** AC-008.

### DEC-004 Каждый детектор — отдельная struct, имплементирующая Detector

- **Why:** изоляция regex compilation, понятные границы тестов, простота добавления новых типов.
- **Tradeoff:** больше файлов. Оправдано читаемостью.
- **Affects:** piidetector.go, secretsdetector.go, financialdetector.go, phidetector.go.
- **Validation:** compile-time assertion + тест каждого.

### DEC-005 Luhn — package-private функция в financialdetector.go

- **Why:** единственный потребитель Luhn — FinancialDetector. Не нужен отдельный shared utility пока.
- **Tradeoff:** если Luhn понадобится в других детекторах/сервисах — рефакторинг в pkg/.
- **Affects:** financialdetector.go, financialdetector_test.go.
- **Validation:** AC-005, AC-006.

### DEC-006 Regex compilation в New*() конструкторах

- **Why:** compile once at init, reuse on every Scan(). Минимизация latency на горячем пути.
- **Tradeoff:** ошибка в regex → ошибка при создании детектора (fail-fast).
- **Affects:** piidetector.go, secretsdetector.go, financialdetector.go, phidetector.go.
- **Validation:** тест конструктора с валидными regex.

## Incremental Delivery

### MVP (Первая ценность)

Detector interface + DetectorResult + PII detector + тесты. Покрывает AC-001, AC-002, AC-003, AC-009, AC-010, AC-011, AC-012.
Валидация: `go test ./src/internal/domain/shield/detector/... -run PII -v`.

### Итеративное расширение

1. Secrets detector (AC-004) — после PII, т.к. архитектура та же.
2. Financial detector + Luhn (AC-005, AC-006) — новая логика Luhn peer-review.
3. PHI detector (AC-007) — самый простой, только ICD-10 regex.
4. Registry (AC-008) — финальный штрих, может быть и раньше (зависит от PII detector на MVP).

## Порядок реализации

1. **Core**: `detector.go` (interface + DetectorResult) — без этого ничего не работает.
2. **PII detector** — самый понятный, задаёт шаблон для остальных.
3. **Secrets detector** — аналогичная структура.
4. **Financial detector** + Luhn — требует отдельного внимания из-за Luhn-алгоритма.
5. **PHI detector** — простой regex.
6. **Registry** — может быть реализован в любой момент после core.

Параллельно: PII, Secrets, Financial, PHI можно делать независимо после core.
Registry не требует ни одного конкретного детектора — можно реализовать параллельно с core.

## Риски

- **Complex regex → false positives**: email/phone/SSN regex могут захватывать лишнее.
  *Mitigation:* конкретные AC-тесты с граничными случаями (AC-010), code review паттернов.
- **Luhn-реализация может быть ошибочной**: алгоритм чувствителен к перестановкам.
  *Mitigation:* отдельный unit-тест на Luhn + AC-006 (невалидный номер) и AC-005 (валидный).
- **JWT-детекция может быть шумной**: любой base64.payload.signature может быть ошибочно принят за JWT.
  *Mitigation:* AC-004 с конкретным форматом, confidence=1.0 только для чёткого regex.

## Rollout и compatibility

- Специальных rollout-действий не требуется. Новый пакет; существующий код не меняется.
- Все детекторы покрыты unit-тестами — регрессия исключена.

## Проверка

- Automated: `go test ./src/internal/domain/shield/detector/...` — покрывает все AC.
- Code review: regex паттерны, Luhn-алгоритм, корректность позиций.
- Каждый AC подтверждается assertion в тестах.

## Соответствие конституции

- нет конфликтов. Detector interface и registry следуют DDD (domain logic без external зависимостей). PII regexes — временная мера до Presidio, зафиксировано в spec.
