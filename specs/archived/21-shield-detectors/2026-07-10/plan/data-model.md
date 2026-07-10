# Базовые детекторы Content Shield — Модель данных

## Scope

- Связанные `AC-*`: `AC-002`
- Связанные `DEC-*`: `DEC-002`
- Статус: `changed`
- Изменение: новый in-memory value object DetectorResult. Существующие entity/value objects не затрагиваются.

## Сущности

### DM-001 DetectorResult (value object, in-memory)

- Назначение: результат сканирования одного совпадения детектора. Содержит тип, фрагмент, позиции и confidence.
- Источник истины: создаётся детектором при сканировании. Не сохраняется в БД.
- Инварианты:
  - StartPos >= 0, EndPos > StartPos, EndPos <= len(text)
  - 0.0 <= Confidence <= 1.0
  - Fragment == text[StartPos:EndPos]
  - Fragment не пустой
- Связанные `AC-*`: AC-002, AC-011, AC-012
- Связанные `DEC-*`: DEC-002
- Поля:
  - `DetectorType` — string, required, тип детектора ("pii", "secrets", "financial", "phi") или конкретного паттерна
  - `Fragment` — string, required, совпавший текст
  - `StartPos` — int, required, >= 0, позиция начала в исходном тексте
  - `EndPos` — int, required, > StartPos, позиция конца (эксклюзивно)
  - `Confidence` — float64, required, 0.0–1.0, надёжность совпадения
- Жизненный цикл:
  - Создаётся: внутри `Scan()` каждого детектора
  - Удаляется: сборщиком мусора после вызова
  - Не сохраняется, не мутируется после создания
- Замечания по консистентности: не применимо (in-memory value, создаётся заново при каждом Scan)

### DM-002 Detector (interface, behavioral)

- Назначение: контракт сканирования текста. Каждый concrete detector имплементирует этот интерфейс.
- Связанные `AC-*`: AC-001
- Связанные `DEC-*`: DEC-001
- Методы:
  - `Scan(ctx context.Context, text string) ([]DetectorResult, error)` — обязателен
  - `Type() DetectorType` — возвращает тип детектора (может быть частью registry)

### DM-003 DetectorRegistry (in-memory)

- Назначение: регистрация и получение детекторов по типу. Thread-safe.
- Связанные `AC-*`: AC-008
- Связанные `DEC-*`: DEC-003
- Методы:
  - `Register(typ DetectorType, d Detector)` — регистрация
  - `Get(typ DetectorType) Detector` — получение (nil если не найден)
  - `Types() []DetectorType` — список зарегистрированных типов

## Связи

- `DetectorRegistry -> Detector`: 1:N регистрация по DetectorType
- `Detector -> DetectorResult`: 1:N на результат одного Scan
- `DM-001 (DetectorResult)` концептуально близок к `entity.Incident`, но не заменяет его. Incident — persisted domain event; DetectorResult — transient scan output.

## Производные правила

- `text[StartPos:EndPos] == Fragment` — гарантируется конструктором/тестами
- `Confidence` для regex-based детекторов всегда 1.0 (точное совпадение шаблона)

## Переходы состояний

- `none`: все объекты immutable или stateless.

## Вне scope

- Persisted entity — не применимо (все объекты transient).
- entity.Incident, entity.Pattern, entity.Detector — не меняются.
