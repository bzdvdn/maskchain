# Admin UI Design Модель данных

## Scope

- Связанные `AC-*`: `AC-001` (admin sessions), `AC-005` (audit log)
- Связанные `DEC-*`: `DEC-001` (admin sessions отдельная таблица), `DEC-002` (audit log async)
- Статус: `changed`
- Добавляются две новые сущности: admin session, audit log entry

## Сущности

### DM-001 AdminSession

- Назначение: хранение сессий admin-пользователя для UI auth
- Источник истины: PostgreSQL (`admin_sessions` таблица)
- Инварианты: session_id уникален, expires_at всегда > created_at
- Связанные `AC-*`: `AC-001`, `AC-004`
- Связанные `DEC-*`: `DEC-001`, `DEC-003`
- Поля:
  - `session_id` — UUID (PK), генерируется на login
  - `username` — string (NOT NULL), из env `ADMIN_USERNAME`
  - `token_hash` — string (NOT NULL), bcrypt/SHA256 хеш токена
  - `created_at` — timestamptz (NOT NULL)
  - `expires_at` — timestamptz (NOT NULL), default now() + `ADMIN_SESSION_TTL`
- Жизненный цикл:
  - создаётся `POST /api/v1/admin/login` при успешной аутентификации
  - автоматически истекает по `expires_at` (middleware проверяет)
  - удаляется `POST /api/v1/admin/logout` (DELETE по session_id)
  - expired записи чистятся cleanup worker (по аналогии с tenant session cleanup)
- Замечания по консистентности:
  - `token_hash` должен быть константного времени сравнения (constant time compare)
  - middleware НЕ делает `UPDATE expires_at` на каждый запрос — только проверяет `expires_at > now()`

### DM-002 AuditLogEntry

- Назначение: аудит изменений конфигурации (tenant create/update/delete, dictionary update, routing)
- Источник истины: PostgreSQL (`audit_log` таблица)
- Инварианты: каждое событие имеет admin_id, action, target
- Связанные `AC-*`: `AC-005`
- Связанные `DEC-*`: `DEC-002`
- Поля:
  - `id` — BIGSERIAL (PK)
  - `admin_username` — string (NOT NULL), кто сделал
  - `action` — string (NOT NULL), тип: `tenant.create`, `tenant.update`, `tenant.delete`, `dictionary.update`, `routing.update`
  - `target` — string (NOT NULL), что изменено (slug/имя)
  - `details` — jsonb (nullable), diff или дополнительные данные
  - `created_at` — timestamptz (NOT NULL)
- Жизненный цикл:
  - создаётся async через buffered channel при каждом значимом действии admin
  - не обновляется и не удаляется (append-only)
  - retention: `AUDIT_LOG_RETENTION_DAYS` (default 90, cleanup worker)
- Замечания по консистентности:
  - async запись → допустима задержка <1s до появления в БД
  - при graceful shutdown worker flush-ит оставшиеся события

## Связи

- `AdminSession` не связан с `AuditLogEntry` — только `admin_username` как строковый идентификатор
- `AuditLogEntry.target` ссылается на `Tenant.slug` по конвенции, без foreign key (audit log не должен блокировать удаление tenant)

## Производные правила

- `POST /api/v1/admin/login`: если `ADMIN_USERNAME` не задан в env → 503 Service Unavailable
- Токен admin сессии: UUID v4, хешируется SHA256 перед сохранением в БД
- Сравнение токена: constant-time сравнение хеша

## Переходы состояний

- AdminSession: active → expired (автоматически по времени) → deleted (logout)
- AdminSession: невалидный логин → не создаётся запись, возвращается 401

## Вне scope

- Tenant sessions не меняются (tenant session model — отдельный домен `domain/session`)
- PIIConfig, Dictionary, Preprocessor — без изменений
