# Audit Incidents Модель данных

## Scope

- Связанные `AC-*`: AC-001, AC-002, AC-003, AC-004, AC-006, AC-007
- Связанные `DEC-*`: DEC-003, DEC-004, DEC-005
- Статус: `changed`
- Incident entity расширяется; таблица incidents дополняется

## Сущности

### DM-001 Incident (расширение)

- Назначение: запись о срабатывании Content Shield — кто, когда, какой детектор, какое действие, redacted контекст
- Источник истины: таблица incidents (PostgreSQL)
- Инварианты: request_id + timestamp не могут быть пустыми; severity — одно из low/medium/high/critical
- Связанные `AC-*`: AC-001, AC-002, AC-003, AC-004, AC-006, AC-007
- Связанные `DEC-*`: DEC-003, DEC-004, DEC-005
- Поля (существующие + новые):

| Поле | Тип | Обязательность | Смысл |
|---|---|---|---|
| `slug` (id) | string | required | UUID инцидента |
| `requestID` | string | required | ID запроса, вызвавшего срабатывание |
| `timestamp` | time.Time | required | Время срабатывания |
| `tenant` | string | required | Тенант (денормализован из профиля) |
| `profileSlug` | string | required | Slug профиля, на котором сработало |
| `detectorType` | string | required | regex / dictionary / presidio |
| `entryValue` | *string | optional | Значение, на котором сработал детектор |
| `severity` | value.Severity | required | low/medium/high/critical |
| `action` | string | required | block / redact / alert |
| `promptSnippetRedacted` | *string | optional | Redacted prompt (rename from `rawSnippet`) |
| `responseSnippet` | *string | optional | Response snippet (новое поле) |
| `detectorID` | string | required | ID детектора (scan internal, не в API) |
| `patternID` | value.PatternID | required | ID паттерна (scan internal, не в API) |
| `fragment` | string | required | Фрагмент текста (scan internal, не в API) |
| `position` | int | required | Позиция в тексте (scan internal, не в API) |

- Жизненный цикл:
  - Создание: ShieldEngine → ScanResult → AlertReaction → repo.Save
  - Чтение: GET /api/v1/incidents (list), GET /api/v1/incidents/:id (detail)
  - Удаление: не в scope данной фичи
  - Обновление: не в scope данной фичи
- Замечания по консистентности:
  - `tenant` денормализован — при смене tenant у профиля старые инциденты не обновляются
  - `promptSnippetRedacted` и `responseSnippet` — уже redacted на уровне создания; слой чтения не редэктит

## Связи

- DM-001 → Profile: incident.profileSlug связывает с профилем (не формальный FK, логическая связь)
- DM-001 → Tenant: incident.tenant денормализован для фильтрации без JOIN

## Производные правила

- none — все поля хранимые, не вычисляемые

## Переходы состояний

- Жизненный цикл инцидента — однократное создание; переходов нет (immutable после записи)

## Вне scope

- Поля для acknowledge/resolve/assign — не в scope данной фичи
- Индексы для full-text поиска — не в scope
- Soft-delete, archived_at — не в scope
