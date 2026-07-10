# Mask Storage — Модель данных

## Scope

- Связанные `AC-*`: `AC-001`, `AC-002`, `AC-008`, `AC-009`
- Связанные `DEC-*`: `DEC-001`, `DEC-003`, `DEC-004`
- Статус: `changed`
- Изменение: новая persisted entity MaskEntry, new value object (UUIDv7), новая таблица `mask_entries` в PG, новый ключ `mask:<id>` в Valkey.

## Сущности

### DM-001 MaskEntry (entity, persisted)

- Назначение: цепочка обратимого маскинга — отображение placeholder → original для одного mask_id.
- Источник истины: PG (`mask_entries`). Кэш в Valkey (`mask:<id>`).
- Инварианты:
  - MaskID не пустой, уникален глобально
  - Replacements не nil (может быть пустым)
  - CreatedAt не zero
- Связанные `AC-*`: AC-001, AC-002, AC-008
- Связанные `DEC-*`: DEC-001
- Поля:
  - `MaskID` — string, required, UUIDv7, уникальный ключ
  - `ProfileID` — *string, optional, nullable, привязка к профилю
  - `Replacements` — map[string]string, required, not nil, placeholder→original (напр. `{"{{abc.1}}": "john@example.com"}`)
  - `CreatedAt` — time.Time, required, not zero, время создания
- Жизненный цикл:
  - Создаётся: в MaskUseCase.MaskText после детекции и замены
  - Читается: в MaskUseCase.UnmaskText для восстановления
  - Удаляется: через MaskStorage.Delete (не используется в MVP)
- PG схема:
  ```sql
  CREATE TABLE mask_entries (
      mask_id      TEXT        PRIMARY KEY,
      profile_id   TEXT,
      replacements JSONB       NOT NULL DEFAULT '{}',
      created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
  );
  CREATE INDEX idx_mask_entries_created_at ON mask_entries (created_at DESC);
  ```
- Valkey схема:
  - Key: `mask:<mask_id>`
  - Value: JSON-сериализованный MaskEntry (все поля)
  - TTL: конфигурируемый (default 3600s)
  - Стратегия: write-through (PG→Valkey), read-through (Valkey→PG fallback→refresh)

### DM-002 Placeholder (value object, convention)

- Назначение: строковый плейсхолдер в замаскированном тексте.
- Связанные `AC-*`: AC-002, AC-003
- Формат: `{{<mask_id>.<counter>}}`, где counter — uint, нумерация с 1
- Пример: `{{abc123.1}}`, `{{abc123.2}}`
- Инварианты:
  - mask_id в плейсхолдере соответствует MaskEntry.MaskID
  - counter уникален в рамках одного MaskEntry
- Производные правила:
  - `strings.ReplaceAll(text, placeholder, original)` — безопасно, т.к. placeholder уникален в тексте

### DM-003 UUIDv7 (value object)

- Назначение: глобально уникальный, time-sortable идентификатор для MaskID.
- Связанные `AC-*`: AC-007
- Связанные `DEC-*`: DEC-006
- Формат: 36 символов, `xxxxxxxx-xxxx-7xxx-{8|9|a|b}xxx-xxxxxxxxxxxx`
- Алгоритм:
  - 48 bits: timestamp в миллисекундах с Unix epoch
  - 4 bits: version (7)
  - 2 bits: variant (10xx)
  - 62 bits: случайные (crypto/rand)
- Пример: `018f3a6e-1b7c-7a23-8b45-6c7d8e9f0a1b`

## Связи

- `MaskUseCase → MaskStorage → MaskEntry`: use case использует storage для CRUD entry.
- `MaskEntry → Placeholder`: 1:N — одна запись содержит много placeholder→original пар.
- `CachedMaskRepo → PostgresMaskRepo + ValkeyMaskRepo`: композитный репозиторий делегирует PG и Valkey.
- `MaskUseCase → DetectorRegistry → CompositeDetector → Detector`: use case получает результаты детекции для генерации замен.
- `MaskEntry` не связан с `entity.Incident` или `entity.Profile` напрямую (ProfileID — опциональный FK-like поле без constraints).

## Производные правила

- `len(entry.Replacements) == 0` → маскинг не выполнялся (чистый текст)
- Плейсхолдер уникален в пределах MaskEntry: `{{<id>.N}}` для N от 1 до len(replacements)
- unmask итерирует `merged` map и выполняет `strings.ReplaceAll` — порядок не важен, т.к. плейсхолдеры не пересекаются

## Переходы состояний

- `none`: MaskEntry создаётся один раз и не мутируется. Delete — терминальное состояние.

## Вне scope

- Profile entity — не создаётся, ProfileID опционален.
- Audit log — не сохраняется (будущий spec).
- Incidents — не связаны с MaskEntry.
- Scheduled cleanup (TTL-based или batch delete).
