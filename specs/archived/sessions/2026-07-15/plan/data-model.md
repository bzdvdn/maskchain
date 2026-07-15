# Sessions Модель данных

## Scope

- Связанные `AC-*`: `AC-001`, `AC-002`, `AC-005`, `AC-006`, `AC-007`
- Связанные `DEC-*`: `DEC-001`, `DEC-002`, `DEC-003`
- Статус: `changed`

## Сущности

### DM-001 Session

- Назначение: трекинг диалога — кто, когда, какой моделью, сколько сообщений, токенов и маскировок каждого типа.
- Источник истины: PostgreSQL (`sessions` table).
- Инварианты:
  - `SessionID` — UUIDv7, уникальный, не меняется после создания.
  - `TenantID` — обязателен, immutable после создания.
  - `Status` ∈ {`active`, `expired`, `closed`}.
  - `ExpiresAt` >= `CreatedAt`; после `expired` или `closed` — update запрещён (кроме DeleteExpired).
  - Все счётчики (`TokenCount`, `MessageCount`, `TotalMasks`, `DictMaskCount`, `PIIMaskCount`, `PreprocessorCount`) >= 0.
- Связанные `AC-*`: `AC-001`, `AC-002`, `AC-005`, `AC-006`, `AC-009`, `AC-010`
- Связанные `DEC-*`: `DEC-001`, `DEC-002`, `DEC-003`
- Поля:
  - `SessionID` — UUIDv7, PK, required. Генерируется на уровне middleware.
  - `TenantID` — TEXT, required. Извлекается из tenant context.
  - `Model` — TEXT, required. Из тела chat-запроса.
  - `TokenCount` — BIGINT, default 0. Сумма токенов за сессию.
  - `MessageCount` — INT, default 0. Количество сообщений.
  - `TotalMasks` — INT, default 0. Всего масок (placeholders).
  - `DictMaskCount` — INT, default 0. Словарных масок.
  - `PIIMaskCount` — INT, default 0. PII-масок.
  - `PreprocessorCount` — INT, default 0. Препроцессорных операций.
  - `Status` — TEXT, default `active`. `active` | `expired` | `closed`.
  - `TTL` — INTERVAL, required. Длительность жизни с момента создания/продления.
  - `CreatedAt` — TIMESTAMPTZ, required. Момент создания.
  - `ExpiresAt` — TIMESTAMPTZ, required. `CreatedAt + TTL`.
- Жизненный цикл:
  - **Создание:** middleware читает `X-Session-ID` (UUIDv7) из запроса → `Save`.
  - **Update:** middleware инкрементирует счётчики на каждый запрос → `IncrementCounts`.
  - **Продление TTL:** `PATCH .../extend` → `ExtendTTL`, сдвигает `ExpiresAt = now() + TTL`.
  - **Закрытие:** `DELETE .../id` → `Close`, статус → `closed`.
  - **Expired:** по `ExpiresAt` → статус `expired` (выставляется при чтении или фоновым процессом).
  - **Удаление:** CleanupWorker → `DeleteExpired`.
- Замечания по консистентности:
  - `IncrementCounts` — атомарный `UPDATE SET col = col + $1` в PG.
  - Race condition на increment: последняя операция побеждает (допустимо для счётчиков).
  - Нельзя закрыть уже `closed` или `expired` сессию — проверка в use case.

### DM-002 SessionID (value object)

- Назначение: типобезопасный UUIDv7 идентификатор сессии.
- Источник истины: генерируется на уровне middleware.
- Инвариант: соответствует RFC 9562 UUIDv7.
- Поля: `Value string`.
- Связанные `AC-*`: `AC-009`.
- Замечания: переиспользовать `mask.NewUUIDv7()` или общий `pkg/uuid` при его появлении.

### DM-003 ReplacementSummary (value object, опционально)

- Назначение: агрегированная статистика маскировок за сессию (не хранится отдельной сущностью — поля в Session).
- Статус: `no-change` — поля уже включены в Session.

## Связи

- `Session -> Tenant`: сессия принадлежит ровно одному тенанту. TenantID — внешний ключ (логический, без FK constraint в PG для гибкости).
- `Session -> MaskEntry`: сессия не ссылается на MaskEntry напрямую. Связь через `X-Session-ID` на уровне middleware.

## Производные правила

- `TotalMasks = DictMaskCount + PIIMaskCount` — не вычисляется, а инкрементируется независимо (защита от рассинхронизации).
- Статус `expired` выставляется если `ExpiresAt < NOW()` и статус `active`.

## Переходы состояний

- `active` → `expired`: по `ExpiresAt` (неявно при чтении или фоновым CleanupWorker).
- `active` → `closed`: вызов `Close` (DELETE endpoint).
- `closed` → (none): финальное состояние.
- `expired` → (none): удаляется CleanupWorker.

## Вне scope

- FK constraint от `sessions.tenant_id` к `tenants.id` — избыточен, tenant_id валидируется через middleware.
- Индексы на счётчиках — не нужны для MVP, добавляются при профилировании.
