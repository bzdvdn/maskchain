# Conversation Logging — Задачи

## Phase Contract

Inputs: plan.md + spec.md + data-model.md + contracts/api.md.
Outputs: упорядоченные исполнительные задачи (фазы), Surface Map, Implementation Context, покрытие AC.
Stop if: задача без конкретных `Touches:` или AC без исполнимой задачи — нет.

## Surface Map

| Surface | Tasks |
|---------|-------|
| src/internal/infra/config/config.go, defaults.go, config_test.go | T1.1 |
| src/internal/infra/crypto/encryptor.go, encryptor_test.go | T1.2, T4.1 |
| src/internal/domain/conversation/conversation_log.go, storage.go, storage_test.go | T1.3, T2.4, T4.1 |
| src/internal/adapters/repository/postgres/migrations/014_conversation_logs.{up,down}.sql | T1.4, T2.4 |
| src/internal/adapters/repository/conversation/pg_conversation_store.go, pg_conversation_store_test.go | T1.5, T2.4, T4.1 |
| src/internal/app/conversation/async_worker.go | T2.1 |
| src/internal/app/conversation/cleanup_worker.go, cleanup_worker_test.go | T2.1, T4.1 |
| src/internal/api/middleware/conversation.go, conversation_test.go | T2.2, T2.5, T4.1 |
| src/internal/api/middleware/shield.go, shield_test.go | T2.2, T4.1 |
| src/internal/api/server.go | T2.2, T2.3 |
| src/cmd/gateway/run.go, src/cmd/all/gateway.go | T2.3 |
| src/internal/api/handler/conversation/conversation_handler.go, conversation_handler_test.go | T3.1, T4.1 |
| src/internal/api/dto/conversation.go | T3.1 |
| src/internal/api/admin.go | T3.1 |
| src/internal/api/swagger/openapi.yaml | T3.1 |
| ui/src/api/conversations.ts | T3.2, T5.1 |
| ui/src/pages/Conversations.tsx, ui/src/App.tsx, ui/src/components/Layout.tsx | T3.2, T5.1 |
| ui/src/pages/Conversations.test.tsx (decode helper), ui/src/utils/base64.ts | T4.2, T5.1 |
| src/internal/api/middleware/conversation.go, conversation_test.go (mask_id + фильтр) | T5.1, T6.1 |

## Implementation Context

- Цель MVP: запрос/ответ (non-streaming JSON и streaming SSE verbatim) + mask-доказательство (как применён mask) фиксируются зашифрованными (AEAD, ключ из ENV) в `conversation_logs`; admin API + UI «Conversations» показывает диалог и применённый mask (base64→atob/TextDecoder); запись полностью асинхронная.
- Инварианты/семантика:
  - `request` = оригинальный `messages`-массив до mask-преобразования shield (capture middleware — ПЕРВЫМ в proxy chain, до shield); `response` = финальное тело после unmask (что увидел клиент); для streaming — verbatim SSE-лента (`data: ...\n\n` + `[DONE]`). (DEC-001, DEC-007, DM-001)
  - Mask-доказательство `masking`: mapping «плейсхолдер → оригинал» (merged dict+PII). Shield после построения `dictMaskMapping` кладёт копию в gin context (`c.Set(conversationMaskKey, ...)`); conversation-логгер (стоит ДО shield) читает его после `c.Next()`. (RQ-007)
  - `masked` = `masking != null` (наличие mapping).
  - `status`: `ok` (shield clean, 2xx) / `blocked` (shield 403, `X-Shield-Status: blocked`) / `error` (shield error или upstream без тела; для streaming — поток без терминатора `[DONE]`).
  - `streamed` = `true` для записей streaming-запросов (`stream: true`); задаётся один раз при создании записи. (RQ-009, DEC-007)
  - Запись не создаётся только при пустом `messages` (W2). Streaming захватывается, НЕ пропускается (AC-003).
  - Контент обрезается до 1 МБ перед шифрованием (для streaming — к накопленной SSE-ленте, при достижении лимита лента обрезается, поток клиенту не прерывается); формат ciphertext = `nonce(12) || ciphertext` (AES-GCM, ключ 32 байта base64 из ENV). (DEC-003, DEC-004)
  - Асинхронность (RQ-008/AC-011): на hot path — только чтение тела + буферизация ответа + enqueue «сырых» данных в канал; **шифрование и DB-write выполняет worker в своей goroutine** (DEC-002). Ошибки worker не влияют на клиентский ответ; warn-лог с trace_id и drop.
  - Хранилище: `conversation_logs` (id uuid, tenant_id, model, status, masked, streamed bool default false, request bytea, response bytea null, masking bytea null, request_len, response_len, created_at; index (tenant_id, created_at)). (DM-001)
- Ошибки/коды:
  - Bootstrap: `conversations.enabled=true` + отсутствие `MASKCHAIN_CONVERSATION_KEY` → startup error (AC-008), exit non-zero.
  - Runtime: encryptor/INSERT fail → запись дропается с warn-логом (`trace_id`), ответ клиенту не меняется (AC-010). Никогда не паниковать от конвейера логов.
  - Admin API: 404 для несуществующего id; 500 при ошибке расшифровки/ciphertext наружу не выходит (AC-005).
- Контракты/протокол:
  - `GET /api/v1/conversations` → envelope ApiResponse, `data.items` (id, tenant_id, model, status, masked, streamed, created_at), `pagination` в корне; без содержимого. (AC-006)
  - `GET /api/v1/conversations/:id` → `data.payload.request`/`data.payload.response`/`data.payload.masking` = base64 StdEncoding; `response`/`masking` могут быть null; для stream-записей `payload.response` — base64 SSE-ленты. (AC-005)
  - Ключ: ENV `MASKCHAIN_CONVERSATION_KEY` (32 байта → base64); читается `os.Getenv`, НЕ через `CONFIG_*`/yaml.
- Proof signals: gated тесты (middleware capture non-streaming/streaming/blocked/fail-open, crypto round-trip + wrong-key, store, handler, cleanup) + curl-флоw из plan «First Validation Path» (ciphertext в psql; base64 в API).
- Вне scope: парсинг SSE-чанков в «чистый» текст (UI показывает raw SSE-транскрипт), key rotation/KMS, editing/export, per-tenant toggle.
- References: DEC-001..DEC-007, DM-001, RQ-001..RQ-009.

## Фаза 1: Основа

Цель: конфиг, крипто, домен, миграция и store — фундамент, без которого pipeline не собрать.

- [x] T1.1 Добавить `ConversationsConfig` (Enabled bool, RetentionDays int) в конфиг: `Config.Conversations`, поле в default (`enabled: false`, `retention_days`) и unit-тест чтения через `CONFIG_CONVERSATIONS_*`. Touches: src/internal/infra/config/config.go, src/internal/infra/config/defaults.go, src/internal/infra/config/config_test.go
- [x] T1.2 Добавить крипто-пакет `infra/crypto`: `Encryptor`/`Decryptor` на AES-256-GCM; `New(keyB64)` с fail-closed (пустота/невалидный ключ → error); константа `KeyEnvVar = "MASKCHAIN_CONVERSATION_KEY"`; формат `nonce(12)||ct`; unit-тесты (round-trip правильным ключом, wrong-key error, ciphertext не содержит plaintext). Touches: src/internal/infra/crypto/encryptor.go, src/internal/infra/crypto/encryptor_test.go
- [x] T1.3 Добавить domain: entity `ConversationLog` (ID, TenantID, Model, Status, Masked, Request/Response []byte, Masking []MaskingEntry, RequestLen/ResponseLen, CreatedAt), `MaskingEntry{Placeholder, Original}` (mapping «плейсхолдер → оригинал») + port `ConversationStore` (SaveBatch, DeleteOlderThan, List, Get) + in-memory тест-дубль. Touches: src/internal/domain/conversation/conversation_log.go, src/internal/domain/conversation/storage.go, src/internal/domain/conversation/storage_test.go
- [x] T1.4 Добавить миграцию `014_conversation_logs.up.sql`/`.down.sql` по DM-001 (поля, включая `masked` и `masking` bytea null, index (tenant_id, created_at)); миграции автозапускаются через embed. Touches: src/internal/adapters/repository/postgres/migrations/014_conversation_logs.up.sql, src/internal/adapters/repository/postgres/migrations/014_conversation_logs.down.sql
- [x] T1.5 Добавить `PgConversationStore` (по паттерну `PgUsageStore`): SaveBatch (pgx.Batch, ON CONFLICT DO NOTHING), DeleteOlderThan, List (пагинация + фильтр tenant), Get (single row). Тесты с pg (или mock). Touches: src/internal/adapters/repository/conversation/pg_conversation_store.go, src/internal/adapters/repository/conversation/pg_conversation_store_test.go

## Фаза 2: MVP Slice

Цель: gateway-конвейер захвата работает end-to-end (capture → encrypt → async-write), data plane не ломается.

- [x] T2.1 Добавить фоновый worker + cleanup по паттерну analytics: `AsyncWorker` (буферный канал; в своей goroutine принимает «сырые» {request, response, masking, meta}, обрезает до 1 МБ, **шифрует все поля через crypto**, batch-INSERT ciphertext) и `CleanupWorker` (DELETE старше RetentionDays). Unit-тесты обоих (по образцу `analytics/async_worker_test.go`, `cleanup_worker_test.go`), включая что шифрование происходит в воркере, а не на hot path. Также устранено несоответствие spec: `ConversationLog.Masking` хранит ciphertext (bytea) вместо plaintext-entries; добавлен `ParseMaskingEntriesJSON`; единый `Sender`-интерфейс для middleware. Touches: src/internal/app/conversation/async_worker.go, src/internal/app/conversation/cleanup_worker.go, src/internal/app/conversation/cleanup_worker_test.go
- [x] T2.2 Реализовать `ConversationMiddleware` (первым в proxy chain, до shield): читает `c.GetRawData()` (оригинал) и восстанавливает Body; пропускает пустые messages; c.Writer-обёртка буферизует финальный ответ; после `c.Next()` читает mask-доказательство из gin context (установлен shield), статус (`ok`/`blocked`/`error` по X-Shield-Status + код), enqueue «сырых» данных в канал воркера — НИКАКОЙ крипты/БД на hot path (AC-011); на ошибках capture/context — warn-лог с trace_id, обработка не прерывается (AC-010). Дополнительно: в `shield.go` после построения `dictMaskMapping` класть его копию в gin context (`c.Set(conversationMaskKey, mapping)`); `masked` = наличие mapping. Unit-тесты (capture/empty-skip/blocked/mask-mapping/fail-open). Внедрить в `RegisterProxyRoute` через регистратор по образцу `session`/`usage` middleware. Touches: src/internal/api/middleware/conversation.go, src/internal/api/middleware/shield.go, src/internal/api/middleware/conversation_test.go, src/internal/api/server.go
- [x] T2.3 Bootstrap: DI в gateway/all: при `cfg.Conversations.Enabled` — создать crypto (`os.Getenv(crypto.KeyEnvVar)`), store, worker (+cleanup) и middleware; при включённой фиче без ключа — startup error (AC-008). При выключенной фиче middleware не регистрируется. Touches: src/cmd/gateway/run.go, src/cmd/all/gateway.go
- [x] T2.4 Добавить поле `streamed bool` (default false) в domain `ConversationLog`, миграцию `014` (ALTER/up/down: колонка `streamed`), `PgConversationStore` (SaveBatch/List/Get — чтение/запись `streamed`) и in-memory тест-дубль; `RawRecord` получает `Streamed bool`; worker сохраняет его без изменений. Unit-тесты store round-trip streamed. Touches: src/internal/domain/conversation/conversation_log.go, src/internal/domain/conversation/storage.go, src/internal/domain/conversation/storage_test.go, src/internal/adapters/repository/postgres/migrations/014_conversation_logs.up.sql, src/internal/adapters/repository/postgres/migrations/014_conversation_logs.down.sql, src/internal/adapters/repository/conversation/pg_conversation_store.go, src/internal/adapters/repository/conversation/pg_conversation_store_test.go, src/internal/app/conversation/async_worker.go
- [x] T2.5 Streaming capture в `ConversationMiddleware`: для `stream:true` НЕ пропускать; `bufferedWriter` получает cap 1 МБ (накопление SSE-ленты; при превышении — обрезать, но поток клиенту продолжает идти); после `c.Next()` — `Streamed=true`, статус: лента с `[DONE]` → `ok`, без `[DONE]` (обрыв/ошибка) → `error`; enqueue в worker. Также исправлен латентный баг T2.2: статус читается из заголовка **ответа** (`c.Writer.Header()`), а не запроса. Unit-тесты: stream capture (лента = response), stream без [DONE] → error, cap обрезка не рвёт поток клиенту, blocked stream. Touches: src/internal/api/middleware/conversation.go, src/internal/api/middleware/conversation_test.go

## Фаза 3: Основная реализация

Цель: admin read-API и UI страница Conversations.

- [x] T3.1 Добавить `ConversationHandler` (List — метаданные+пагинация+фильтр tenant; Get — расшифровка ключом request/response/**masking**, DTO с `payload.request/response/masking` base64 StdEncoding, 404/500) + DTO в `dto.conversation` + `AdminServer.RegisterConversationHandler` под `adminSessionMw` + Swagger-секцию в `openapi.yaml`. Unit-тесты handler (list без контента, detail base64 round-trip включая masking, 404, decrypt-ошибка → 500). Touches: src/internal/api/handler/conversation/conversation_handler.go, src/internal/api/handler/conversation/conversation_handler_test.go, src/internal/api/dto/conversation.go, src/internal/api/admin.go, src/internal/api/swagger/openapi.yaml
- [x] T3.2 Добавить UI: API-клиент `ui/src/api/conversations.ts`, страница `Conversations.tsx` (список; деталь через `atob`+`TextDecoder` показывает request/response и **как применён mask**: mapping «original → placeholder», сгруппированный из `payload.masking`), роут `/conversations` в `App.tsx`, пункт меню в `Layout.tsx` (секция Management). Touches: ui/src/api/conversations.ts, ui/src/pages/Conversations.tsx, ui/src/App.tsx, ui/src/components/Layout.tsx

## Фаза 4: Проверка

Цель: доказать фичу и оставить пакет reviewable.

- [x] T4.1 Backend-валидация: интеграционный тест middleware со shield+handler (AC-001 capture, AC-002 blocked, AC-003 stream-capture, AC-010 fail-open), store-тест, handler-тест, crypto (AC-004), cleanup (AC-007); прогнать `make test` и `make lint`. Touches: тесты из карты выше + прогон Makefile-целей
- [x] T4.2 UI-валидация: decode-хелпер `ui/src/utils/base64.ts` (atob → UTF-8/TextDecoder) + компонентный тест `Conversations.test.tsx` (список рендер, деталь декодирует); ручной curl-чек по «First Validation Path» из plan (psql: ciphertext; `/api/v1/conversations/:id` → base64 → `base64 -d` сопадает с исходным prompt). Touches: ui/src/utils/base64.ts, ui/src/pages/Conversations.test.tsx

## Фаза 5: Пост-verify уточнения (mask_id + фильтр)

Цель: пост-verify правки от владельца — связать лог с mask-id shield и логировать только диалоги с маскировкой (не засорять таблицу «чистым» трафиком).

- [x] T5.1 Добавить `mask_id` в лог: shield публикует `dictMaskID` в gin context (`c.Set(conversationMaskIDKey, ...)`, рядом с `X-Shield-Dict-Mask-ID`); `RawRecord` + `ConversationLog` получают `MaskID`; миграция `014` — колонка `mask_id VARCHAR(64)` (up/down); store (SaveBatch/List/Get) читает/пишет `mask_id`; DTO list/detail и UI-карточка отображают mask-id. Unit-тесты: middleware MaskID propagation, store round-trip, handler detail/list, UI detail. Touches: src/internal/api/middleware/shield.go, src/internal/api/middleware/conversation.go, conversation_test.go, src/internal/app/conversation/async_worker.go, async_worker_test.go, src/internal/domain/conversation/conversation_log.go, src/internal/adapters/repository/postgres/migrations/014_conversation_logs.{up,down}.sql, src/internal/adapters/repository/conversation/pg_conversation_store.go, pg_conversation_store_test.go, src/internal/api/dto/conversation.go, src/internal/api/handler/conversation/conversation_handler.go, conversation_handler_test.go, src/internal/api/swagger/openapi.yaml, ui/src/api/conversations.ts, ui/src/pages/Conversations.tsx, Conversations.test.tsx
- [x] T6.1 Логировать только запросы, прошедшие маскировку (или blocked): в `ConversationMiddleware` после `c.Next()` — если `masking` пуст И статус не `blocked` → не enqueue. Пользовательский выбор: «маскированные + заблокированные». Unit-тесты: clean unmasked → 0 записей; blocked без маски → 1 запись (blocked); masked stream без [DONE] → error; unmasked fail-open → 0 записей. Touches: src/internal/api/middleware/conversation.go, src/internal/api/middleware/conversation_test.go

## Покрытие критериев приемки

- AC-001 -> T2.2, T1.5, T4.1
- AC-002 -> T2.2, T4.1
- AC-003 -> T2.2, T2.5, T4.1
- AC-004 -> T1.2, T1.5, T4.1
- AC-005 -> T3.1, T4.1
- AC-006 -> T3.1, T4.1
- AC-007 -> T2.1, T4.1
- AC-008 -> T1.1, T1.2, T2.3, T4.1
- AC-009 -> T3.2, T4.2
- AC-010 -> T2.2, T2.1, T4.1, T6.1
- AC-011 -> T2.1, T2.2, T4.1

## Заметки

- Порядок задач согласован с plan «Порядок реализации»: фундамент (Ф1) → capture (Ф2) → admin/UI (Ф3) → валидация (Ф4).
- T2.2/T2.3 нельзя параллелить с Ф3 (handler зависит от читаемого store).
- Фаза «Основа» не пишет в hot path — её результат только структуры и БД.
- `masked`/`masking` формируются по mapping из gin context (выставляется shield), отвечая на вопрос «как применён mask» без дублирования контента (RQ-007, refinement).