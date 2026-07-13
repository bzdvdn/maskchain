# Profile Cache — Задачи

## Phase Contract

Inputs: plan.md, spec.md, data-model.md.
Outputs: задачи с Touches и AC-покрытием.
Stop if: none.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `adapters/repository/profile/` (new) | T1.1, T2.1, T2.2, T2.4, T3.1, T3.2, T4.1 |
| `infra/config/config.go` | T1.2 |
| `infra/metrics/metrics.go` | T2.3 |
| `cmd/gateway/main.go` | T2.5, T3.3 |
| `cmd/admin/main.go` | T2.5 |
| `go.mod` | T1.1 |

## Implementation Context

- **Цель MVP**: ProfileCache (Valkey full profile + LRU metadata) с read/write-through, graceful degradation, warming на старте и метриками. PubSub — после MVP.
- **Инварианты/семантика:**
  - Read order: Valkey first (1 round trip full profile) → при miss: PG → populate Valkey+LRU
  - Degraded (Valkey down): LRU metadata + PG dict → assemble full profile
  - LRU хранит только ProfileMetadata (~300B, без dict entries)
  - Valkey key: `profile:<tenant_id>:<slug>:v<version>`
  - ProfileCache реализует существующий `shield.ProfileRepository` без изменений интерфейса
- **Ошибки/коды:**
  - Valkey error → fallback to LRU+PG (degraded), log.Warn, cache_stale++
  - Valkey недоступен + LRU miss → полное чтение из PG, log.Warn, cache_stale++
  - Write-through: PG.Save успешен, Valkey.Set упал → continue (eventual consistency по TTL)
  - Warming error → log.Warn, не блокировать startup
- **Контракты/протокол:**
  - Valkey value: JSON-marshalled `entity.Profile` (весь объект включая dictionaries)
  - PubSub канал: `profile.invalidate:<slug>` (pattern `profile.invalidate:*`)
  - PSUBSCRIBE для pattern-matching подписки
- **Границы scope:**
  - ListByTenant не кэшируется
  - Dictionaries не кэшируются отдельным ключом (входят в full profile Valkey)
  - LRU не восстанавливается после restart
- **Proof signals:** mock PG + mock Valkey тесты для каждого AC; GET /metrics содержит ненулевые counters
- **References:** DEC-001 (Valkey-first), DEC-002 (JSON), DEC-003 (PubSub), DEC-004 (write-through eventual), DEC-005 (config), DEC-006 (warming)

## Фаза 1: Основа

Цель: подготовить структуру пакета, конфигурацию и LRU-адаптер.

- [x] T1.1 Создать пакет `adapters/repository/profile/`, добавить `hashicorp/golang-lru/v2` в go.mod.
  Touches: `adapters/repository/profile/`, `go.mod`

- [x] T1.2 Добавить `ProfileCacheConfig` (valkey_ttl_sec, lru_size, warm_on_startup, warm_concurrency) в config.go + DefaultConfig поле + defaults.
  Touches: `infra/config/config.go`

- [x] T1.3 Реализовать `ProfileMetadata` struct и `ProfileLRUCache` (thread-safe обёртка над `lru.Sized[string, *ProfileMetadata]`). Size limit: 10000 (из конфига).
  Touches: `adapters/repository/profile/lru.go`

## Фаза 2: MVP Slice

Цель: ProfileCache core (без PubSub) + метрики + warming + wire.

- [x] T2.1 Реализовать `ProfileValkeyRepo` — JSON marshal/unmarshal полного Profile, key `profile:<tenant_id>:<slug>:v<version>`, TTL из конфига. Методы: Get, Set, Del.
  Touches: `adapters/repository/profile/valkey.go`

- [x] T2.2 Реализовать `ProfileCache` (composite repository, implements `shield.ProfileRepository`):
  - `FindBySlug/FindByID`: Valkey first (full profile) → если miss: PG → populate Valkey+LRU
  - `Save`: PG → Valkey Set (full) → return
  - `Delete`: PG Del → Valkey Del → LRU Remove
  - Degraded (Valkey error): LRU metadata + PG dict → assemble → warn log; если LRU miss → PG full
  - Метрики: cache_hits/misses/stale инкремент в точках вызова
  Touches: `adapters/repository/profile/cached.go`

- [x] T2.3 Зарегистрировать counter-векторы `maskchain_profile_cache_hits_total`, `maskchain_profile_cache_misses_total`, `maskchain_profile_cache_stale_total`, `maskchain_profile_cache_invalidations_total` в `metrics.go`, wire в `RegisterMetrics`.
  Touches: `infra/metrics/metrics.go`

- [x] T2.4 Реализовать `ProfileCacheWarmer`: list active profiles из PG → для каждого: GET Valkey (есть → populate LRU metadata, нет → полная PG загрузка → populate Valkey+LRU). Запуск в фоновой горутине, не блокирует startup.
  Touches: `adapters/repository/profile/warm.go`

- [x] T2.5 Wire ProfileCache в gateway/main.go (вместо прямого PostgresProfileRepo) и admin/main.go. В gateway: запустить Warmer (если warm_on_startup) async. Valkey client переиспользовать из существующего `initValkey`. Admin: просто заменить repo, PubSub publisher не нужен (будет в T3.3).
  Touches: `cmd/gateway/main.go`, `cmd/admin/main.go`

- [x] T2.6 Написать unit/integration тесты для Фазы 2: mock PG + mock Valkey. Покрыть AC-001, AC-002, AC-003, AC-004, AC-006, AC-008, AC-009, AC-010.
  Touches: `adapters/repository/profile/cached_test.go`, `adapters/repository/profile/valkey_test.go`, `adapters/repository/profile/lru_test.go`, `adapters/repository/profile/warm_test.go`

## Фаза 3: PubSub-инвалидация

Цель: кэш-когерентность между gateway инстансами.

- [x] T3.1 Добавить PubSub publish в `ProfileCache.Save` и `ProfileCache.Delete`: публикация `profile.invalidate:<slug>` через Valkey `Do(Publish)`. Save → publish после успешного Set, Delete → publish после успешного Del.
  Touches: `adapters/repository/profile/cached.go`

- [x] T3.2 Реализовать PubSub subscriber goroutine: PSUBSCRIBE `profile.invalidate:*`, при получении сообщения — извлечь slug из channel name, удалить ProfileMetadata из LRU. Логирование reconnect.
  Touches: `adapters/repository/profile/pubsub.go`

- [x] T3.3 Wire PubSub subscriber в gateway/main.go (запуск горутины после старта сервера). Wire Publisher — уже в T3.1.
  Touches: `cmd/gateway/main.go`

- [x] T3.4 Написать тесты для PubSub: mock PSUBSCRIBE, симулировать publish, проверить LRU eviction. Покрыть AC-005, AC-007.
  Touches: `adapters/repository/profile/pubsub_test.go`, `adapters/repository/profile/cached_test.go`

## Фаза 4: Проверка

Цель: финальный verify.

- [x] T4.1 Выполнить verify: проверить что все AC покрыты тестами, тесты проходят, lint/typecheck чистые. Обновить repo-map если изменилась структура.
  Touches: `specs/active/102-profile-cache/tasks.md`

## Покрытие критериев приемки

- AC-001 -> T2.2, T2.6
- AC-002 -> T2.2, T2.6
- AC-003 -> T2.2, T2.6
- AC-004 -> T2.2, T2.6
- AC-005 -> T3.1, T3.2, T3.4
- AC-006 -> T2.2, T2.6
- AC-007 -> T3.1, T3.4
- AC-008 -> T2.3, T2.6
- AC-009 -> T2.2, T2.6
- AC-010 -> T2.4, T2.6

## Заметки

- Фаза 1 и Фаза 2 можно выполнять последовательно (T1.x → T2.x)
- T2.3 (metrics) не зависит от T2.2 — можно параллельно после T2.1
- T3.x зависят от T2.2 (нужен ProfileCache core)
- Не начинать Фазу 3 пока Фаза 2 не прошла verify
- Trace-маркеры `@sk-task 102-profile-cache#T{x.x}` ставить над owning type/function/test declaration

Готово к: /spk.implement 102-profile-cache
