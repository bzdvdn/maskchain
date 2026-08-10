# Conversation Logging — Модель данных

## Scope

- Связанные `AC-*`: `AC-001`, `AC-002`, `AC-003`, `AC-004`, `AC-005`, `AC-006`, `AC-007`, `AC-009`, `AC-010`, `AC-011`
- Связанные `DEC-*`: `DEC-003`, `DEC-004`
- Статус: `changed`
- Новая persisted entity `conversation_logs`; существующие таблицы не меняются.

## Сущности

### DM-001 ConversationLog

- Назначение: фиксация диалога «исходный запрос пользователя → итоговый ответ LLM» для non-streaming (JSON) и streaming (SSE verbatim) proxy-запросов, с возможностью отображения в admin UI.
- Источник истины: PostgreSQL, таблица `conversation_logs`; пишет gateway, читает admin.
- Инварианты:
  - `request`/`response` хранятся только как AEAD-ciphertext (nonce||tag||ct), никогда plaintext.
  - `tenant_id`, `model`, `created_at`, `masked` обязательны.
  - Для записи «запрос зафиксирован» допустимо `response = NULL` (blocked/error-путь), но `request` не может быть `NULL` для создаваемой записи.
- Связанные `AC-*`: `AC-001`, `AC-002`, `AC-003`, `AC-004`, `AC-005`, `AC-006`
- Связанные `DEC-*`: `DEC-003`, `DEC-004`
- Поля:
  - `id` - UUID (uuidv7-подобный, как в mask), PK, required
  - `tenant_id` - VARCHAR(255), required
  - `model` - VARCHAR(255), required
  - `status` - VARCHAR(32), required; значения: `ok`, `blocked`, `error` (derived из исхода shield/upstream)
  - `masked` - BOOLEAN, required, default false; `true` если присутствует mask-доказательство (`masking`), т.е. отправленное в LLM отличалось от сохранённого оригинала
  - `streamed` - BOOLEAN, required, default false; `true` для streaming-запросов (`stream: true`), где `response` — verbatim SSE-лента
  - `masking` - BYTEA, nullable; зашифрованный JSON-массив `[{"placeholder":"[MASK_...]", "original":"..."}]` (merged dict+PII от shield); `NULL` когда mask не применялся
  - `mask_id` - VARCHAR(64), nullable; id dict-mask из `X-Shield-Dict-Mask-ID` (совпадает с `[MASK_<id>.<n>]`-плейсхолдерами), связывает лог с маской в заголовках ответа; `NULL` когда dict-mask не применялся (например, PII-only или blocked)
  - `request` - BYTEA[], required (encrypted request payload)
  - `response` - BYTEA, nullable (encrypted response payload; `NULL` для blocked/error без тела; для streaming — ciphertext SSE-ленты)
  - `request_len` / `response_len` - INTEGER, опционально для метаданных и оценки роста
  - `created_at` - TIMESTAMPTZ NOT NULL; индекс вместе с `tenant_id`
- Жизненный цикл:
  - Создание: gateway после завершения запроса (после обработки ответа/блокировки) — one record per request; для streaming — после завершения потока (terminator `[DONE]`/обрыв).
  - Фильтр создания (пост-verify): запись создаётся только если применена маскировка (`masking` непуст) ИЛИ запрос заблокирован (`status=blocked`); чистые unmasked-запросы не логируются.
  - Обновление: не обновляется.
  - Удаление: TTL-очисткой `DELETE WHERE created_at < now() - ttl` через CleanupWorker; ручного удаления в scope нет.
- Замечания по консистентности:
  - Дубликатов/частичных строк избегать: запись вставляется одним INSERT после полного формирования ciphertext.
  - Если шифрование или INSERT упал — запись дропается с warn-логом, но поток прокси не прерывается (AC-010 / RQ-006).

## Связи

- Нет значимых межсущностных связей: `conversation_logs` не ссылается FK на tenants (тенанты в этой БД не имеют отдельных сущностей) и не связан с usage/масками.
- Опционально: `session_id` для связи с сессией при наличии `X-Session-ID` — помечено как кандидат на итеративное расширение, не блокирует MVP.

## Производные правила

- `status`: `ok` если shield clean и upstream 2xx с ответом; `blocked` если shield отклонил (403/X-Shield-Status: blocked); `error` если shield error или upstream ошибка без тела; для streaming без терминатора `[DONE]` (обрыв соединения/ошибка) — `error`.
- `streamed`: `true` если запрос был `stream: true`; задаётся один раз при создании записи.
- Длина контента: request/response обрезаются до лимита (≈1 МБ) перед шифрованием (Assumption spec); для streaming лимит применяется к накопленной SSE-ленте.

## Переходы состояний

- Жизненный цикл достаточно прост (create → ttl-delete): отдельный список переходов не требуется.

## Вне scope

- Индексирование/поиск по содержимому (ciphertext не позволяет), полнотекстовый поиск, key-rotation, история версий записей.

## No-Change Stub

- (не используется — модель меняется; см. DM-001)