# Remove: удаление audit incident инфраструктуры — Модель данных

## Scope

- Связанные `AC-*`: `AC-001`, `AC-006`, `AC-015`, `AC-016`
- Связанные `DEC-*`: `DEC-001`, `DEC-004`
- Статус: `changed`

## Сущности

### DM-001 Incident (удаляется)

- Назначение: представляло событие срабатывания Content Shield (DLP) — dictionary hit или pattern match.
- Источник истины: таблица `incidents` в PostgreSQL.
- Статус: **deleted**
- Связанные `AC-*`: `AC-001`, `AC-003`, `AC-015`
- Связанные `DEC-*`: `DEC-001`, `DEC-004`
- Поля (удаляемые):
  - `id` - UUID
  - `tenant_id` - UUID
  - `profile_slug` - string, optional (удалён в cleanup-profile-repository)
  - `incident_type` - enum (dictionary_hit, pattern_match, audit)
  - `severity` - enum (low, medium, high, critical)
  - `action` - string (allow, block, mask, redact, alert)
  - `pattern_name` - string, optional
  - `matched_text` - text, optional
  - `request_id` - string
  - `created_at` - timestamp
- Жизненный цикл: создавался в middleware/scan_usecase, читался через API handler, не обновлялся, не архивировался.
- Замечания по консистентности: неактуально — сущность удаляется.

### DM-002 ScanResult (изменяется)

- Назначение: результат проверки контента Content Shield.
- Статус: **changed**
- Связанные `AC-*`: `AC-006`
- Поля (до/после):
  - `status` - ScanStatus — **остаётся**
  - `incidents []Incident` — **удаляется**
  - `Incidents()` accessor — **удаляется**
  - `NewScanResult` — **адаптируется** (без incidents)
- Жизненный цикл: без изменений — создаётся в scan service, передаётся в middleware для принятия решения.

## Связи

- `Incident -> Tenant`: FK `tenant_id` — удаляется вместе с таблицей.
- `Incident -> ScanResult`: embedding `[]Incident` — удаляется.

## Производные правила

- не было — severity задавался явно.

## Переходы состояний

- не было — Incident был append-only.

## Вне scope

- Session-based audit (phase 120+): заменит Incident сущность, но не в scope этой spec.
- Таблицы `profiles`, `dictionary_entries`: уже удалены в cleanup-profile-repository.
