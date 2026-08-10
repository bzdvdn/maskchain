# Conversation Logging — План

## Phase Contract

Inputs: `specs/active/conversation-logging/spec.md`, конституция, минимальный repo-контекст (proxy chain, usage pipeline, config-инфраструктура).
Outputs: `plan.md`, `data-model.md`, `contracts/api.md`.
Stop if: scope расплывчатый — нет, спецификация конкретна.

## Цель

Добавить сквозной конвейер захвата диалога «запрос → ответ» для proxy-трафика (non-streaming и streaming/SSE): gateway-конвейер фиксирует исходное тело запроса (до mask) и итоговое тело ответа (после unmask; для streaming — verbatim SSE-лента), шифрует AEAD-ключом из ENV и пишет в PostgreSQL через тот же async-pattern, что usage-аналитика. Admin-бинарник расшифровывает и отдаёт через `/api/v1/conversations` (list — метаданные; detail — base64-обёртка содержимого). Новая страница Conversations в React UI декодирует base64 и показывает диалог. Ошибки захвата не ломают data plane.

## MVP Slice

- Middleware захвата (оригинал + mask-доказательство из shield + финальный ответ; non-streaming JSON и streaming SSE verbatim) → async worker (шифрование) → async-запись в `conversation_logs` (/chat/completions и /messages).
- Admin handler list/detail с base64-обёрткой (в т.ч. `payload.masking`), DTO, роуты под adminSessionMw.
- UI страница Conversations (список + деталь: request/response + как применён mask, decode atob).
- Fail-closed bootstrap: ключ из ENV обязателен при включённой фиче.
- Закрывает: AC-001, AC-004, AC-005, AC-006, AC-008, AC-009, AC-010, AC-011.

## First Validation Path

1. `make build`; собрать gateway+admin. Запустить compose-стек.
2. Включить `conversations.enabled: true`, задать `MASKCHAIN_CONVERSATION_KEY`.
3. `curl` non-streaming `/api/v1/chat/completions` → `psql`: в `conversation_logs` появилась строка; `request`/`response` — не-читаемый ciphertext (проверка AC-004). Повторить с `stream:true` → строка с `streamed=true`, `response` — ciphertext SSE-ленты (AC-003).
4. `curl` с admin Bearer: `GET /api/v1/conversations` (список, в т.ч. поле `streamed`) → `GET /api/v1/conversations/:id` → `payload.request`/`payload.response` — base64; `echo <b64> | base64 -d` отдаёт исходный prompt и ответ (AC-005).
5. UI `/conversations`: список и раскрытие детали (AC-009).

## Scope

- Новая поверхность: middleware `conversation.go` в `src/internal/api/middleware/`, инжектится в `RegisterProxyRoute` (первым в chain, до shield).
- Крипто-пакет `src/internal/infra/crypto/` (README-style): AES-256-GCM, ключ `MASKCHAIN_CONVERSATION_KEY`.
- Доменный слой: `src/internal/domain/conversation/` (entity, port `ConversationStore`).
- Адаптер `src/internal/adapters/repository/conversation/pg_conversation_store.go` + миграция `014_conversation_logs.up/down.sql`.
- Async-конвейер по образцу analytics: `src/internal/app/conversation/` (async worker + cleanup worker) или переиспользование существующего паттерна.
- Admin handler `src/internal/api/handler/conversation/`, DTO, роуты в `admin.go` `RegisterConversationHandler`.
- UI: `ui/src/pages/Conversations.tsx`, `ui/src/api/conversations.ts`, меню в `Layout`, роут в `App.tsx`.
- Config: секция `ConversationsConfig` в `config.go`; bootstrap-проверка ключа в `run.go`.

## Performance Budget

- Hot path (middleware): только чтение тела + буферизация ответа + enqueue — без crypto/DB; median overhead < 0.2 мс.
- Фон: worker-горутина шифрует и батчами пишет; cleanup — отдельная горутина.
- Peak memory: буфер запроса/ответа ограничен 1 МБ (обрезка), очередь канала ограничена буфером воркера.
- Рост БД: ограничен TTL-очисткой; контент обрезается до лимита (Допущение в spec).

## Implementation Surfaces

- `src/internal/api/middleware/conversation.go` (new) — захват тела запроса (GetRawData + restore), чтение mask-mapping из gin context (установлен shield), writer-обёртка финального ответа, статус/masked, enqueue «сырых» данных в канал воркера (БЕЗ crypto/DB на hot path). Почему middleware: ответ перехватывается только на пути proxy (как в UsageMiddleware), а не в хендлере; нужно быть вне/до shield для оригинала.
- `src/internal/api/middleware/shield.go` (изменение) — после построения `dictMaskMapping` (merged dict+PII) кладёт его копию в gin context (`c.Set(conversationMaskKey, mapping)`), чтобы conversation-логгер (стоящий ДО shield) получил mask-доказательство (RQ-007).
- `src/internal/infra/crypto/` (new) — AES-GCM Encryptor/Decryptor; fail-closed на отсутствие ключа. Почему infra: крипто не относится к domain, нет обратной зависимости.
- `src/internal/domain/conversation/` (new) — `ConversationLog` entity, `ConversationStore` port (`Save`, `DeleteOlderThan`, `List`, `Get`). Почему отдельный domain: DDD-слой репозиториев уже разделён по доменам (analytics, session).
- `src/internal/adapters/repository/conversation/pg_conversation_store.go` (new) — pgx-реализация store.
- `src/internal/adapters/repository/postgres/migrations/014_conversation_logs.{up,down}.sql` (new).
- `src/internal/api/handler/conversation/` (new) — `ConversationHandler` (List/Get).
- `src/internal/api/admin.go` — `RegisterConversationHandler` (new method).
- `src/internal/api/dto/` — `ConversationResponse`, `ConversationListResponse` (new).
- `src/internal/api/server.go` — вставка conversation middleware в chain `RegisterProxyRoute` (по образцу `RegisterUsageMiddleware`).
- `src/cmd/gateway/run.go`, `src/cmd/admin/run.go`, `src/cmd/all/*.go` — DI: создание crypto, store, worker, middleware, handler.
- `src/internal/infra/config/config.go` (+`defaults.go`/`validator.go` при необходимости) — `ConversationsConfig`.
- Swagger `src/internal/api/swagger/openapi.yaml` — секция conversations (new).
- `ui/src/pages/Conversations.tsx` (new), `ui/src/api/conversations.ts` (new), `ui/src/App.tsx`, `ui/src/components/Layout.tsx` (меню).
- Тесты: unit crypto (encrypt/decrypt, wrong key), middleware capture, store (pg/in-memory), handler DTO, UI-компонент decode; интеграционный — GET list/detail.

## Bootstrapping Surfaces

- `014_conversation_logs` миграция + `ConversationStore` port + pg-адаптер — до всего UI/API.
- Крипто-пакет + bootstrap-проверка ключа — до включения фичи в prod.

## Влияние на архитектуру

- `RegisterProxyRoute`: chain расширяется опциональным middleware (по паттерну session/usage — `s.conversationMiddleware != nil`).
- Gateway/admin делят общую БД и ENV-ключ — расшифровка только в admin, шифрование только в gateway.
- Новый конвейер в gateway повторяет analytics: AsyncWorker (batch insert) + CleanupWorker (TTL).
- Migrations: новая таблица, без изменения существующих.
- OTel: к спанам захвата добавить трассировочные события не требуется (контент не логируется в OTel — приватность).

## Acceptance Approach

- AC-001 — middleware integration-тест: mock store + non-streaming запрос → `Save` вызван с расшифровываемыми request/response; SQL/`Get` возвращает исходный prompt. Surfaces: middleware, store.
- AC-002 — интеграционный тест со shield-block (403): запись имеет `status=blocked`, `response` пуст. Surfaces: middleware (чтение X-Shield-Status/кода ответа).
- AC-003 — тест middleware: `stream:true` запрос → создаётся запись со `streamed=true`, `response` = verbatim SSE-лента; поток без `[DONE]` → `status=error`. Surfaces: middleware.
- AC-004 — unit crypto: правильный ключ → round-trip; wrong key → auth error; test на ciphertext не содержит plaintext. Surfaces: `infra/crypto`.
- AC-005 — handler-тест: Detail возвращает DTO с `payload.request`/`payload.response` (base64 StdEncoding); decode в тесте даёт оригиналы. Surfaces: handler, dto.
- AC-006 — handler-тест: List возвращает только метаданные (без content/payload) + pagination/tenant-фильтр. Surfaces: handler, dto.
- AC-007 — unit cleanup worker: записи старше TTL удалены, свежие живы (по образцу `cleanup_worker_test.go`). Surfaces: app/conversation.
- AC-008 — unit: конструктор crypto без ключа возвращает ошибку; run.go: enabled без ключа → exit non-zero. Surfaces: crypto, run.go bootstrap.
- AC-009 — UI: компонент Conversations рендерит список и деталь с корректным UTF-8 decode из base64 (тест decode-хелпера) + ручной чек. Surfaces: UI страница/API-клиент.
- AC-010 — integration middleware-тест: encryptor/store с ошибкой → ответ 200 до клиента, обработка не прерывается, ошибка в warn-логе. Surfaces: middleware, crypto, store.
- AC-011 — integration/unit: worker с задержкой → ответ клиенту не ждёт записи; инспекция hot path (нет crypto/DB в middleware). Surfaces: middleware, app/conversation.
- Зависимости: AC-005/AC-006 зависят от data model (таблица) и contracts (DTO); AC-007 зависит от cleanup-паттерна analytics.

## Данные и контракты

- `data-model.md` — обязателен: новая сущность `conversation_logs` (`changed`, + поля `masked`/`masking`).
- `contracts/api.md` — новые эндпоинты `/api/v1/conversations` (GET list, GET `/:id`), payload base64 (request/response/masking).
- Query-интерфейс только через `ConversationStore`; без изменения существующих контрактов (usage/session/incident не трогаются).
- Compatibility: admin-бинарник и gateway-бинарник версии с этим конвейером могут работать с БД без новой таблицы? Нет — нужен запуск миграции. Ролout см. «Rollout и compatibility».

## Стратегия реализации

- DEC-001 Ставить conversation middleware первым в proxy chain (до shield)
  Why: shield читает оригинал тела и замещает `c.Request.Body` замаскированной версией; захват «исходного сообщения» обязан быть до shield. Ответ: writer-обёртка conversation — самая внешняя, поэтому статус unmask от shield (dictUnmaskWriter.flush) и тело от провайдера проходят через неё целиком, и в буфер попадает финальный ответ, который увидел клиент.
  Tradeoff: двойное чтение тела (захват + shield) — восстанавливаем `c.Request.Body` после `GetRawData`; накладные расходы ограничены 1 МБ.
  Affects: middleware/conversation.go, server.go.
  Validation: AC-001 (origins: расшифрованный request содержит исходный prompt, response = клиентский).

- DEC-002 «Сырые» данные в middleware, шифрование + запись — в фоновых goroutines (полностью async)
  Why: по требованию RQ-008/AC-011 ни крипта, ни БД не должны быть на горячем пути ответа. Middleware только читает тело, буферизует ответ в writer-обёртке и enqueue'ит нешифрованные {request, response, masking, meta} в канал; dedicated worker (паттерн AsyncWorker из analytics) в своей goroutine шифрует все поля и батчами вставляет ciphertext. Usage-пиплайн шифрует/до этого не шифровал — здесь обе фазы вынесены с hot path.
  Tradeoff: потеря последних невылетевших записей при падении процесса (буферный канал) — приемлемо для логов; потребление памяти ограничено ёмкостью канала и обрезкой до 1 МБ.
  Affects: middleware/conversation.go (enqueue), app/conversation (worker: encrypt+write), crypto.
  Validation: AC-004, AC-011, SC-001.

- DEC-007 Стриминг захватывается как verbatim SSE-лента с флагом `streamed`
  Why: streaming — значимая доля трафика; для инцидент-аудита нужен фактический ответ, который клиент увидел. Conversation writer-обёртка буферизует SSE-ленту (все `data: ...\n\n` + `[DONE]`), не задерживая чанки — поток идёт клиенту в реальном времени; после завершения потока или обрыва запись создаётся с `streamed=true` и `response` = накопленная лента (после unmask).
  Tradeoff: память буфера ленты ограничена лимитом 1 МБ (при достижении лента обрезается, поток клиенту не прерывается); поток без терминатора `[DONE]` (обрыв) помечается `status=error`. Парсинг deltas в «чистый» текст — вне scope (Открытый вопрос spec; MVP: UI показывает raw SSE-транскрипт).
  Affects: middleware/conversation.go (bufferedWriter с cap лимита, статус по наличию `[DONE]`), domain (поле `Streamed`), миграция 014, pg store, DTO/UI.
  Validation: AC-003, RQ-009.

- DEC-003 Отдельный пакет `infra/crypto` с fail-closed конструктором
  Why: единое место шифрования/расшифровки (gateway пишет, admin читает); отсутствие ключа при enabled → ошибка инициализации, а не silent можно.
  Tradeoff: если нужен будет KMS/rotation — потребуется рефакторинг интерфейса, заложено: Encryptor interface.
  Affects: infra/crypto, run.go bootstrap.
  Validation: AC-008.

- DEC-004 Таблица `conversation_logs` с bytea request/response/masking (nonce+ct) и TTL
  Why: хранение зашифрованных блобов (включая mask-доказательство), а не JSON с пометками; индексы по (tenant_id, created_at) для пагинации и cleanup.
  Tradeoff: нельзя полнотекстово фильтровать по содержимому — не требуется.
  Affects: миграция 014, pg store.
  Validation: AC-001, AC-004, AC-007.

- DEC-005 Расшифровка только на чтение (admin), base64 в DTO (`payload.request`/`payload.response`, StdEncoding)
  Why: соответствует требованию «на фронт расшифрованный, но в base64»; при decrypt-ошибке — 500, ciphertext наружу не уходит.
  Tradeoff: содержимое расшифровывается на каждый запрос detail (незначительный CPU).
  Affects: handler/conversation, dto.
  Validation: AC-005, AC-006.

- DEC-006 UI: TextDecoder для корректного UTF-8 после atob из base64
  Why: `atob` возвращает binary string; для кириллицы/эмодзи нужно безошибочное преобразование в UTF-8.
  Tradeoff: чуть больше кода, чем `atob` напрямую; покрыто тестом.
  Affects: ui/src/pages/Conversations.tsx.
  Validation: AC-009.

## Incremental Delivery

### MVP (Первая ценность)

- Middleware + crypto + store + миграция + async/cleanup worker (gateway) + admin list/detail + bootstrap fail-closed.
- Критерий: AC-001, AC-004, AC-005, AC-006, AC-008 закрыты; ручной path из «First Validation Path» работает (п.3–4).

### Итеративное расширение

- UI страница Conversations (AC-009) — независимый инкремент поверх API.
- Захват blocked-запросов (AC-002) и стриминга (AC-003, SSE verbatim capture) — уточнение middleware.
- TTL cleanup (AC-007) — фоновый воркер (можно вместе с MVP).
- Swagger-секция.

## Порядок реализации

1. Миграция + crypto + domain port + pg store + unit-тесты — фундамент, без него остальное не на чем.
2. Conversation middleware + внедрение в chain (`RegisterProxyRoute`) + async-writer — захват (AC-001/003/002/010).
3. Bootstrap-проверка ключа в run.go (AC-008).
4. Admin handler + DTO + роуты + Swagger (AC-005/006).
5. UI страница (AC-009).
6. Cleanup worker (AC-007).
Параллельно нельзя: middleware до store; handler до middleware. UI можно параллелить с backend при готовом contracts.

## Риски

- Риск 1: двойное чтение тела Request приводит к конфликту с shield-мидлварой (сломанное тело).
  Mitigation: захват через `GetRawData` + полное восстановление `c.Request.Body`; интеграционный тест с реальной цепочкой (shield + conversation).
- Риск 2: захват «ответа» перехватит блокирующие/error-ответы shield не так, как ожидается (пустой response у blocked).
  Mitigation: определение статуса по `X-Shield-Status`/коду ответа; AC-002 тест.
- Риск 3: offline-UI атоб не переживает UTF-8 (кириллица).
  Mitigation: TextDecoder (DEC-006), юнит-тест decode.
- Риск 4: рост БД/производительность.
  Mitigation: лимит контента 1 МБ, TTL-очистка, async-запись, индексы.
- Риск 5: ключ в не том бинарнике/окружении — admin не расшифрует.
  Mitigation: один env-кролж, doc в конфиге/README; fail-closed при старте обоих.

## Rollout и compatibility

- Новая миграция `014` — обязательный шаг деплоя до включения фичи; подключается автоматически (паттерн миграций).
- Фича вкл/выкл флагом `conversations.enabled`; по умолчанию **выключено** — до включения поведения production не меняется.
- При первом включении: задать `MASKCHAIN_CONVERSATION_KEY` в env gateway и admin; проверить логи старта.
- Monitoring: метрика захвата (количество сохранённых/ошибочных), счетчик CleanupWorker; log warn при drop.
- Специального backfill нет (данные начинают фиксироваться после включения).

## Проверка

- Automated: crypto (round-trip, wrong key, no-plaintext), middleware (capture non-streaming / capture streaming SSE verbatim / blocked / stream-error без [DONE]), pg store (save/get/list/delete-ttl, в т.ч. поле streamed), handler (list metadata / detail base64 / 404), cleanup worker (ttl), UI decode (utf-8 base64), config/bootstrap (fail-closed).
- Manual/operational: `curl`-поток First Validation Path; просмотр ciphertext в `psql`; UI `/conversations`.
- Подтверждает AC-001…AC-011 и DEC-001…DEC-006.

## Соответствие конституции

- нет конфликтов: фича вписывается в domain/port/adapters слои, PostgreSQL как хранилище тенантов/логов (конституция: «React UI — только для управления тенантами и логов»).
- Приватность согласуется с ней: контент в plaintext только в UI, в БД — AEAD-шифрованный.