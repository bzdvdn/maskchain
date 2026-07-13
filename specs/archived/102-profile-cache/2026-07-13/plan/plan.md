# Profile Cache — План

## Phase Contract

Inputs: spec, inspect (pass), repo context (gateway/admin main.go, config, metrics, mask repo).
Outputs: plan, data-model.md.
Stop if: none.

## Цель

Добавить `ProfileCache` — composite repository, реализующий `shield.ProfileRepository`: Valkey (full profile с dictionary entries) как primary cache, in-memory LRU (metadata без entries) как fallback при сбое Valkey. Read-through, write-through + PubSub-инвалидация. Graceful degradation. Метрики.

## MVP Slice

- `ProfileCache` с read/write-through, LRU + Valkey, PubSub-инвалидацией
- Конфигурация (`profile_cache.*`)
- Graceful degradation
- Метрики Prometheus
- AC-001, AC-002, AC-003, AC-004, AC-006, AC-009, AC-008 — базовый контракт кэша (без PubSub)

## First Validation Path

Интеграционный тест: mock PG + mock Valkey. Save → FindBySlug (Valkey hit, полный профиль, без PG) → FindBySlug при Valkey down (LRU metadata + PG dict) → Delete (Valkey DEL + LRU evict). Полный цикл покрывает AC-001, AC-002, AC-003, AC-004, AC-009, AC-006.

## Scope

- `src/internal/adapters/repository/profile/cached.go` — `ProfileCache`, composite repo
- `src/internal/adapters/repository/profile/valkey.go` — `ProfileValkeyRepo` (аналог `ValkeyMaskRepo`)
- `src/internal/adapters/repository/profile/lru.go` — обёртка над `hashicorp/golang-lru/v2`
- `src/internal/adapters/repository/profile/pubsub.go` — подписка/обработка инвалидации
- `src/internal/adapters/repository/profile/warm.go` — `ProfileCacheWarmer`, startup warming
- `src/internal/infra/config/config.go` — `ProfileCacheConfig` (warm_on_startup, warm_concurrency)
- `src/internal/infra/metrics/metrics.go` — регистрация cache-метрик
- `src/cmd/gateway/main.go` — wire ProfileCache, старт PubSub subscriber
- `src/cmd/admin/main.go` — wire ProfileCache (write-through)

## Performance Budget

- LRU hit: < 1ms p99
- Valkey hit: < 5ms p99
- PG miss: < 50ms p99 (значение из SC-001)
- LRU: ~10k entries × ~300B (metadata, без dict entries) = ~3 MB RSS

## Implementation Surfaces

| Surface | Статус | Изменение |
|---|---|---|
| `adapters/repository/profile/` | new | Новый пакет: cached.go, valkey.go, lru.go (ProfileMetadata), pubsub.go, warm.go |
| `infra/config/config.go` | existing | +ProfileCacheConfig, +defaults, +DefaultConfig поле |
| `infra/metrics/metrics.go` | existing | +ProfileCacheHits/Misses/Stale/Invalidations counter-векторы |
| `cmd/gateway/main.go` | existing | Wire ProfileCache, запуск PubSub subscriber горутины |
| `cmd/admin/main.go` | existing | Wire ProfileCache вместо PostgresProfileRepo напрямую |
| `api/handler/profile/handler.go` | existing | Не меняется — принимает shield.ProfileRepository интерфейс |
| `domain/shield/repository.go` | existing | Не меняется — ProfileCache реализует существующий интерфейс |

## Bootstrapping Surfaces

1. `adapters/repository/profile/` — создать директорию и пустой пакет
2. `go.mod` — добавить `hashicorp/golang-lru/v2`
3. `infra/config/config.go` — добавить `ProfileCacheConfig`

## Влияние на архитектуру

- Локальное: новый пакет адаптера, новая конфигурация, новые метрики.
- Интеграции: gateway и admin получают ProfileCache вместо PostgresProfileRepo напрямую.
- Data model: не меняется (нет новых таблиц/колонок).
- Rollout: feature flag не нужен — кэш прозрачен для handler'ов.

## Acceptance Approach

| AC | Подход | Surfaces | Валидация |
|---|---|---|---|
| AC-001 | Read-through: Valkey miss → PG → full profile в Valkey, metadata в LRU | cached.go, valkey.go, lru.go | mock PG + mock Valkey; assert Set(full) + LRU(metadata, без dict entries) |
| AC-002 | Write-through: Save → PG + Valkey full + PubSub | cached.go, valkey.go | mock PG + mock Valkey, assert Set(full)+Publish |
| AC-003 | Valkey hit: 1 round trip, full profile с dict | cached.go, valkey.go | mock Valkey returns full profile; assert 0 PG calls; profile.Dictionaries() непуст |
| AC-004 | LRU fallback: Valkey down → LRU metadata + PG dict | cached.go, lru.go, profile.go | mock Valkey err; LRU has metadata; assert PG dict loaded; warn log |
| AC-005 | PubSub invalidation | pubsub.go, cached.go | mock PSUBSCRIBE, publish, assert LRU eviction |
| AC-006 | Degradation cold: Valkey down + LRU empty → PG full | cached.go | mock Valkey err; assert PG full read; LRU populated with metadata |
| AC-007 | Version bump: Save → new key, old key DEL | cached.go, valkey.go | assert key `v2` exists, `v1` DEL |
| AC-008 | Metrics export | metrics.go, cached.go | GET /metrics, assert 4 non-zero counters |
| AC-009 | Delete: PG DEL + Valkey DEL + PubSub | cached.go, valkey.go | mock PG + mock Valkey, assert Del+Publish |
| AC-010 | Cache warming: startup → list PG → populate LRU+Valkey | warm.go, cached.go | mock PG returns profiles; after warm assert LRU+Valkey populated; assert no startup block |

## Данные и контракты

- Data model: не меняется (см. `data-model.md`)
- API/event контракты: новый PubSub-канал `profile.invalidate:<slug>` (pattern `profile.invalidate:*`)
- `shield.ProfileRepository` — интерфейс не меняется
- Конфигурация: новый блок `profile_cache`

## Стратегия реализации

### DEC-001 Valkey-first, LRU metadata-only

- **Why**: Hot path `/mask` требует полный профиль со словарём. Valkey — 1 round trip, что оптимально. LRU хранит только ProfileMetadata (~300B, без словаря) — fallback при недоступности Valkey. Альтернатива (LRU-first) заставила бы либо дублировать словарь в LRU (память), либо делать 2 round trip на hot path.
- **Tradeoff**: LRU не ускоряет hot path при здоровом Valkey (но Valkey и так < 5ms). При degraded mode нужно 2 шага (LRU metadata + PG dict).
- **Affects**: cached.go, lru.go (ProfileMetadata), valkey.go, pubsub.go
- **Validation**: AC-001, AC-003, AC-004, AC-006

### DEC-002 JSON-сериализация профиля в Valkey

- **Why**: Простейший формат, уже используется в `ValkeyMaskRepo`. Profile — структура без циклических ссылок. JSON позволяет легко дебажить.
- **Tradeoff**: Не fastest (vs protobuf/gob), но latency некритична (< 1ms на json.Marshal). Миграция формата в будущем — новый ключ (version в имени).
- **Affects**: valkey.go
- **Validation**: AC-001, AC-003

### DEC-003 PubSub на одном канале `profile.invalidate:<slug>` с PSUBSCRIBE `profile.invalidate:*`

- **Why**: Канал на slug позволяет подписаться на все invalidate-события через pattern. PSUBSCRIBE — стандартный valkey/redis механизм. Альтернатива (один канал `profile.invalidate` с slug в payload) требует фильтрации на клиенте.
- **Tradeoff**: PSUBSCRIBE — broadcast на все matching каналы; при 10k профилей нагрузка на PubSub незначительна (только при Save/Delete).
- **Affects**: pubsub.go, cached.go
- **Validation**: AC-005, AC-009, RQ-012

### DEC-004 Write-through без distributed transaction

- **Why**: PG — единственный source of truth. Если Valkey SET/Publish упал после успешного PG — данные консистентны в PG, кэш устареет по TTL. Outbox/Kafka — overkill для этой задачи.
- **Tradeoff**: Окно несогласованности до TTL (по умолчанию 5 мин). Для профилей (редко меняются) приемлемо.
- **Affects**: cached.go
- **Validation**: W-02 из inspect (transient failure — eventual consistency)

### DEC-006 Async cache warming при старте gateway

- **Why**: После рестарта gateway LRU пуст. Первый запрос `/mask` попадёт в Valkey (хорошо), но при холодном Valkey — в PG. Warming загружает active профили в фоне, не блокируя старт. Профилей обычно десятки — warming занимает < 1s.
- **Tradeoff**: Дополнительная PG нагрузка при старте. Опционально (warm_on_startup config), concurrency ограничен. Ошибки warming не фатальны — read-through работает.
- **Affects**: warm.go, cached.go, gateway/main.go, config.go
- **Validation**: AC-010

### DEC-005 Конфигурация как отдельный блок `profile_cache`

- **Why**: Чистое разделение с mask cache (разные TTL, разные LRU size). Не переиспользуем `MaskConfig.CacheTTLSec`.
- **Tradeoff**: Дублирование структуры конфига. Некритично.
- **Affects**: config.go
- **Validation**: RQ-009, RQ-010

## Incremental Delivery

### MVP (Первая ценность)

1. Config: `ProfileCacheConfig` (valkey_ttl_sec, lru_size) + DefaultConfig
2. `lru.go`: `ProfileMetadata` struct + `ProfileLRUCache` обёртка над `lru.Sized[string, *ProfileMetadata]`
3. `valkey.go`: `ProfileValkeyRepo` — JSON set/get/del полного профиля с TTL
4. `cached.go`: `ProfileCache` — read-through (Valkey-first → PG → populate Valkey+LRU), write-through (PG → Valkey), delete (PG + Valkey), degraded (LRU metadata + PG dict)
5. `warm.go` — `ProfileCacheWarmer`: list active profiles → for each: try Valkey (populate LRU) or full PG → populate both
6. Wire в gateway + admin
7. Метрики: counter-векторы cache_hits/misses/stale
8. Тесты: AC-001, AC-002, AC-003, AC-004, AC-006, AC-009, AC-008, AC-010

### Итеративное расширение (после MVP)

1. PubSub-инвалидация: `pubsub.go` + PSUBSCRIBE горутина в gateway
2. Wire PubSub subscriber в gateway main.go
3. Wire PubSub publish в cached.go (Save, Delete)
4. Тесты: AC-005, AC-007

## Порядок реализации

1. **Config** — ProfileCacheConfig + defaults
2. **ProfileMetadata** + **LRU адаптер** — можно тестировать изолированно
3. **Valkey адаптер** (full profile) — можно тестировать изолированно
4. **ProfileCache core** — Valkey-first read-through + degraded (LRU + PG dict) + write-through
5. **Метрики** — регистрация + инкремент в cached.go
6. **Warm** — `warm.go`, фоновая горутина, wire в gateway main.go
7. **Wire в main.go** — gateway + admin, интеграционные тесты (AC-001-004, 006, 009, 008, 010)
8. **PubSub** — publish + subscriber, финальные тесты (AC-005, AC-007)

## Риски

- **Риск 1**: Valkey PubSub reconnect — подписка теряется при обрыве. **Mitigation**: valkey-go автоматически переподключается; после reconnect subscriber goroutine переподписывается (штатная логика valkey-go DedicatedClient). Добавить логгирование reconnect.
- **Риск 2**: JSON-сериализация Profile с dictionary 1k+ entries — большой Valkey payload. **Mitigation**: Valkey TTL ограничивает lifetime; LRU не содержит entries — memory не затрагивается. При необходимости — сжатие (gzip) или структурные изменения словаря на плане.
- **Риск 3**: Конкурентный доступ к LRU (sync.RWMutex или lru.Sized thread-safe). **Mitigation**: `hashicorp/golang-lru/v2` — thread-safe; дополнительная синхронизация не нужна.

## Rollout и compatibility

- Специальных rollout-действий не требуется. `ProfileCache` реализует существующий `shield.ProfileRepository` — handler'ы не меняются.
- После деплоя: мониторить `maskchain_profile_cache_hits_total` (ожидаемый рост) и `maskchain_profile_cache_stale_total` (ожидаемый 0 в штатном режиме).

## Проверка

- **Unit-тесты**: valkey.go, lru.go, cached.go (с mock PG + mock Valkey)
- **Интеграционные тесты**: ProfileCache с реальным Valkey (через docker-compose) + testcontainers PG
- **AC coverage**: каждый AC → минимум 1 тест
- **DEC coverage**: каждый DEC подтверждается тестом или код-ревью

## Соответствие конституции

нет конфликтов: PostgreSQL остаётся source of truth, Go + Valkey в стеке, интерфейс ProfileRepository не меняется.
