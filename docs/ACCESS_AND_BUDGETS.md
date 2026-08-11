# Virtual Keys и Budget Enforcement

Контроль доступа к LLM через DB-first виртуальные ключи и финансовое ограничение расхода (budget) по тенанту, ключу или модели.

## Модель

### Virtual Key

Виртуальный ключ — это scoped-креденшал, который **может заменить** или дополнить legacy `tenants.api_keys`. Хранится в Postgres (`virtual_keys`), в Valkey не дублируется.

| Поле | Описание |
|---|---|
| `id` | `vk_<hex>` |
| `tenant_id` | Владелец ключа |
| `label` | Человекочитаемое имя (опционально) |
| `allowed_models` / `blocked_models` | Scoped model access: только эти модели (пусто = все), заблокированные всегда запрещены |
| `budget_cap` | Жёсткий лимит расходов ключа в USD (persisted, не зависит от budget counter) |
| `spent` | Накопленный расход ключа |
| `expires_at` | Срок действия (опционально) |
| `metadata` | Произвольные пары ключ-значение |
| `enabled` | Активен/отключён |

Ключ возвращается **только один раз** при создании в поле `key` ответа. В БД хранится только `key_hash` (SHA-256). Аутентификация по ключу — через `middleware/virtualkey_auth.go`, которое подставляет Virtual Key в контекст.

### Budget

Бюджет — правило лимита расхода по скоупу:

| Поле | Описание |
|---|---|
| `scope` | `tenant` (весь тенант), `key` (конкретный VirtualKey), `model` (конкретная модель в тенанте) |
| `type` | `monthly`, `daily`, `custom` (с `custom_days`) |
| `soft_limit` | Порог алерта (не блокирует) |
| `hard_limit` | Жёсткий лимит — при достижении запросы блокируются (429) |
| `notify_at` | `[]float64` — проценты от hard_limit, при пересечении которых шлётся алерт |
| `currency` | Валюта (по умолчанию `USD`) |
| `enabled` | Активен/отключён |

Счётчик расхода живёт в **Valkey** по ключу `budget:{scope}:{entity}:{period}` (`entity`: tenant_id / key_id / tenant:model). Каждый период (`monthly`/`daily`/`custom`) — новый ключ с TTL до конца периода. Постоянная история — в Postgres (`budgets`, `budget_spend`, `budget_spend_daily`).

## Как работает enforcement

Пайплайн запроса:

```
Client --> Auth (tenant|virtual key) --> RateLimit --> BudgetMiddleware --> Shield --> Routing --> LLM
                                                        |                                  |
                                                        +--> после ответа: cost = Lookup(model).Cost(in,out)
                                                             counter.Increment + RecordSpend + alerts
```

**До запроса** (`BudgetMiddleware`, только `POST`, `src/internal/api/middleware/budget.go`):
1. Из контекста берутся tenantID и keyID, из запроса — `model`.
2. `ListActiveForRequest` находит активные бюджеты по скоупам: tenant → key → model (пересечение).
3. Для каждого проверяется `CounterKey(now)` через `SpendCounter.Current`. Если `HardExceeded` → ответ `429` с `error_code: BUDGET_EXCEEDED`.

**После ответа** (non-streaming, статус 200, в теле есть `usage`):
1. Body буферизуется `usageBodyWriter` и парсится `extractUsage(body, model)`.
2. `cost = CostRateRegistry.Lookup(model).Cost(prompt_tokens, completion_tokens)` (курс за 1k токенов).
3. `counter.Increment` (атомарный `INCRBYFLOAT` + `EXPIRE`).
4. `RecordSpend` пишет историю в Postgres.
5. `CrossedThresholds(prev, new)` находит пересечённые проценты из `notify_at` → `AlertNotifier.NotifyBudgetExceeded` (webhook + лог).
6. Для бюджета скоупа `key` — `accumulateKeySpend` добавляет cost в `VirtualKey.spent`.

**Streaming** ответов не инкрементирует счётчик (по умолчанию). Hard-limit проверка перед запросом работает всегда.

**Агрегация:** `AggregationWorker` (ticker, по умолчанию `5m`, настраивается `budgets.aggregation_interval`) материализует дневной spend через `BudgetRepository.AggregateDaily` в `budget_spend_daily`.

## Alerts (webhook)

При пересечении порога `notify_at` отправляется `POST` на `budgets.alert_webhook_url`:

```json
{
  "event": "budget.notify",
  "budget_id": "budget_...",
  "tenant_id": "...",
  "scope": "tenant",
  "threshold_pct": 80,
  "spent": 96.4,
  "hard_limit": 120,
  "currency": "USD",
  "notified_at": "2026-08-11T10:00:00Z"
}
```

Если `alert_webhook_url` не задан — алерты только в лог.

## Конфигурация

```yaml
# deployments/docker-compose/config-runtime.yaml
budgets:
  alert_webhook_url: "${BUDGET_WEBHOOK_URL}"   # опционально
  aggregation_interval: 5m                      # опционально, по умолчанию 5m
```

## Admin API

### Virtual Keys — `/api/v1/keys` (admin session)

| Метод | Путь | Описание |
|---|---|---|
| `POST` | `/api/v1/keys` | Создать (в ответе поле `key` — показывается один раз) |
| `GET` | `/api/v1/keys` | Список всех |
| `GET` | `/api/v1/keys/tenants/:slug` | Ключи тенанта |
| `GET` | `/api/v1/keys/:id` | Один ключ |
| `PATCH` | `/api/v1/keys/:id` | Обновить (label, модели, budget_cap, expires_at, enabled, metadata) |
| `DELETE` | `/api/v1/keys/:id` | Отозвать |

Пример создания:

```bash
curl -s -X POST http://localhost:9090/api/v1/keys \
  -H "Authorization: Bearer <admin-session>" \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "demo",
    "label": "production-integration",
    "allowed_models": ["gpt-4o"],
    "blocked_models": [],
    "budget_cap": 100,
    "expires_at": "2027-01-01T00:00:00Z",
    "metadata": {"team": "platform"}
  }'
```

Ответ `201`: `{"id":"vk_...","key":"sk-mc_...","tenant_id":"demo",...}`.

Запрос к gateway с этим ключом:

```bash
curl -s -X POST http://localhost:8080/api/v1/chat/completions \
  -H "Authorization: Bearer sk-mc_..." \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"Hi"}]}'
```

### Budgets — `/api/v1/budgets` (admin session)

| Метод | Путь | Описание |
|---|---|---|
| `POST` | `/api/v1/budgets` | Создать бюджет |
| `GET` | `/api/v1/budgets` | Список (в ответе поле `spent` — текущий расход периода) |
| `GET` | `/api/v1/budgets/tenants/:slug` | Бюджеты тенанта |
| `GET` | `/api/v1/budgets/:id` | Один бюджет |
| `PATCH` | `/api/v1/budgets/:id` | Обновить |
| `DELETE` | `/api/v1/budgets/:id` | Удалить |
| `GET` | `/api/v1/budgets/:id/history` | История spend (`?limit=&offset=`) |

Пример создания:

```bash
curl -s -X POST http://localhost:9090/api/v1/budgets \
  -H "Authorization: Bearer <admin-session>" \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "demo",
    "scope": "tenant",
    "type": "monthly",
    "soft_limit": 80,
    "hard_limit": 100,
    "currency": "USD",
    "notify_at": [50, 80, 100]
  }'
```

Ключевые комбинации scope/type:

- `scope: key`, `virtual_key_id: "vk_..."` — бюджет на конкретный ключ.
- `scope: model`, `model: "gpt-4o"` — бюджет на модель в тенанте.
- `type: custom`, `custom_days: 30` — кастомное окно.

## Администрирование в UI

В admin UI (порт `9090`) доступны:
- **Keys** (`/keys`) — создание (с показом ключа один раз), enable/disable, revoke, scoped модели.
- **Budgets** (`/budgets`) — дашборд с прогресс-барами `spent/hard_limit` (оранжевый ≥ 90%), create/edit modal (scope, period, soft/hard limits, notify_at, currency), enable/disable, delete, история spend.

Все изменения ключей и бюджетов пишутся в audit log (`create_key`, `update_key`, `revoke_key`, `create_budget`, `update_budget`, `delete_budget`, ...).

## Совместимость и замечания

- **Legacy fallback сохраняется**: `tenants.api_keys` и `middleware/auth.go` продолжают работать. Виртуальные ключи — приоритетный путь; удаление legacy — отдельный рефакторинг.
- **`budgetrepo/valkey_ratelimit.go` и `valkey_tokenbudget.go`** — токен-бюджеты и rate limit, НЕ денежный бюджет. Денежный счётчик — `internal/adapters/repository/budget/valkey_spendcounter.go`.
- **Курсы моделей** берутся из `CostRateRegistry` (таблица cost rates, редактируется в admin: `/api/v1/analytics/cost-rates`). Для моделей без курса расход не считается (`cost=0`).
- Стоимость считается из `usage.prompt_tokens`/`usage.completion_tokens` в теле ответа. Для streaming-провайдеров body не буферизуется — расход по ним в текущей версии не начисляется.
