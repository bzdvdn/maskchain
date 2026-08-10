---
report_type: verify
slug: conversation-logging
status: pass
docs_language: ru
generated_at: 2026-08-10
---

# Verify Report: conversation-logging

## Scope

- snapshot: Проверка шифрованного лога запросов/ответов LLM (non-streaming + streaming SSE verbatim), mask-доказательства, async-записи, admin API list/detail (base64), UI Conversations, fail-closed без ключа, пост-verify правок (mask_id + фильтр «маскированные+blocked»).
- verification_mode: default
- artifacts:
  - CONSTITUTION.md
  - specs/active/conversation-logging/tasks.md
- inspected_surfaces:
  - src/internal/api/middleware/conversation.go, conversation_test.go
  - src/internal/api/middleware/shield.go (mask_id в gin context)
  - src/internal/app/conversation/async_worker.go, async_worker_test.go, cleanup_worker.go
  - src/internal/infra/crypto/encryptor.go, encryptor_test.go
  - src/internal/domain/conversation/conversation_log.go, storage.go
  - src/internal/adapters/repository/conversation/pg_conversation_store.go, pg_conversation_store_test.go
  - src/internal/adapters/repository/postgres/migrations/014_conversation_logs.{up,down}.sql
  - src/internal/api/handler/conversation/conversation_handler.go, conversation_handler_test.go
  - src/internal/api/dto/conversation.go
  - src/internal/api/swagger/openapi.yaml
  - src/internal/infra/config/config.go, defaults.go, config_test.go
  - src/cmd/gateway/run.go, src/cmd/all/gateway.go (bootstrap fail-closed)
  - ui/src/api/conversations.ts, ui/src/pages/Conversations.tsx, Conversations.test.tsx, utils/base64.ts

## Verdict

- status: pass
- archive_readiness: safe
- summary: Все 16 задач `[x]` имеют observable proof (unit/integration тесты + live-DB store round-trip + live admin 401/маршрут); в ходе verify найден и исправлен реальный баг (NULL scan `mask_id` в List/Get) + латентный баг теста (`len(masking)!=11`); spec AC-001 согласован с пост-verify фильтром T6.1.

## Checks

- task_state: completed=16, open=0; спорных нет (T1.1..T4.2, T5.1, T6.1)
- acceptance_evidence:
  - AC-001 -> T2.2, T1.5: TestConversationMiddlewareShieldIntegrationCapture: pass; TestPgConversationStoreSaveBatchAndGet (live DB): pass
  - AC-002 -> T2.2, T6.1: TestConversationMiddlewareBlockedStatus, TestConversationMiddlewareShieldIntegrationBlocked, TestConversationMiddlewareStreamBlocked: pass
  - AC-003 -> T2.5: TestConversationMiddlewareStreamCaptureAndEmptySkip, ShieldIntegrationStream, StreamWithoutDoneIsError, StreamBufferCap: pass
  - AC-004 -> T1.2: TestEncryptor_RoundTrip / WrongKey / NoPlaintextInCiphertext / DecryptShortInput: pass
  - AC-005 -> T3.1, T5.1: TestConversationHandlerDetailBase64RoundTrip (включая MaskID): pass; live GET /api/v1/conversations -> 401 UNAUTHORIZED (маршрут за admin-сессией)
  - AC-006 -> T3.1: TestConversationHandlerListMetadataOnly, TestPgConversationStoreListAndFilter (live DB): pass
  - AC-007 -> T2.1: TestCleanupWorkerDeletesOldRecords, TestPgConversationStoreDeleteOlderThan (live DB): pass
  - AC-008 -> T1.1, T1.2, T2.3: TestEncryptor_NewFailClosed, TestConversationsConfig_EnvOverride/Defaults: pass; bootstrap fail-closed в run.go/gateway.go (`failing closed` + error)
  - AC-009 -> T3.2, T4.2: ui vitest Conversations.test.tsx (list render, detail decode, empty state): 10/10 pass
  - AC-010 -> T2.2, T6.1: TestConversationMiddlewareShieldIntegrationFailOpen (0 записей), SkipsUnmaskedClean: pass
  - AC-011 -> T2.1: TestAsyncWorkerBatchInsert/GracefulShutdown/BufferOverflow: pass; hot path не содержит crypto/DB (инспекция conversation.go)
- implementation_alignment:
  - middleware по-прежнему первый в proxy chain (buf.written + c.Writer.Header()), маска из `c.Set(conversationMaskKey)` читается после `c.Next()`; фильтр: `if len(masking)==0 && status != blocked { return }`
  - `mask_id` пробрасывается: shield `c.Set(conversationMaskIDKey, dictMaskID)` -> middleware `conversationMaskIDFromContext` -> RawRecord -> worker `WithMaskID` -> store (SaveBatch/List/Get) -> DTO list/detail -> UI «Mask ID:»
  - ciphertext формат nonce||ct (AES-GCM), ключ ENV `MASKCHAIN_CONVERSATION_KEY`, fail-closed в `crypto.New`
  - store NULL-безопасен для mask_id (исправлен scan на `*string`)

## Errors

- (исправлены в ходе verify)
  - pg_conversation_store.go List/Get: `cannot scan NULL into *string` для `mask_id` — исправлено сканированием в `*string` (List: строка scan маски, Get: локальный `maskID *string` + подстановка)
  - pg_conversation_store_test.go SaveBatchAndGet: `len(got.Masking) != 11` при фактической длине 18 — исправлен ассерт на `len("masking-ciphertext")`
  - Live-DB: колонка `mask_id` отсутствовала (контейнер запущен со старым бинарником до добавления ALTER в 014) — применена вручную `ALTER TABLE ... ADD COLUMN IF NOT EXISTS mask_id`; на свежей БД 014 up создаёт колонку автоматически

## Warnings

- `check-ready.sh verify` выдаёт WARN: пути в `Touches:` вида `conversation_test.go`, `async_worker_test.go`, `migrations/014_...{up,down}.sql` не существуют как одиночные пути — это дешёвый false-positive формата (задачи указывают файлы без полного префикса). Не влияет на статус.
- store-интеграционные тесты скипаются без `TEST_DATABASE_URL`; прогнаны против live Postgres (localhost:5433) — все pass.
- `speckeep doctor` показывает orphaned trace-метки для всех задач (tasks.md живёт в `specs/active/<slug>/`, doctor ищет в корне `specs/active/`) — пре-существующее ограничение, не связано с фичей.

## Questions

- none

## Not Verified

- AC-005/AC-006 полный live-цикл с авторизацией (detail base64 после входа): не подтверждён end-to-end на live-стеке (контейнер со старым бинарником без mask_id; маршрут подтверждён 401, round-trip — unit-тестами). Покрыто тестами.
- SC-001/SC-002 (латенция/error rate) — не замерялись в этом verify (вне AC-матрицы).
- UI-страница вживую (браузер) — не проверялась; покрыто vitest.

## Next Step

- safe to archive
