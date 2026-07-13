# Profile Cache — Read-through кэш профилей

## Scope Snapshot

- In scope: двухуровневый кэш профилей (Valkey full profile + in-memory LRU metadata), PubSub-инвалидация, graceful degradation, метрики кэша.
- Out of scope: кэширование других сущностей (dictionaries, incidents), hot-reload конфигурации, rate limiting gateway к profile repository.

## Цель

Разработчик gateway получает снижение latency и нагрузки на PG при запросах профилей за счёт того, что полный профиль (включая словарь) читается из Valkey одним round trip. LRU хранит только метаданные профиля (без entries словаря) — защита от burst-нагрузки при сбое Valkey. Успех фичи измеряется 0 обращений к PG при Valkey hit и корректной инвалидацией при обновлении.

## Основной сценарий

1. Gateway стартует с конфигурацией, содержащей адрес Valkey, TTL и размер LRU.
2. **Hot path** (`/mask`): `ProfileCache.FindBySlug` читает Valkey (ключ `profile:<tenant_id>:<slug>:v<version>`). При попадании — возвращается полный профиль с dictionaries, без обращения к PG.
3. **Read-through**: при промахе в Valkey — PG читает профиль + словарь, полный профиль записывается в Valkey, метаданные (без entries словаря) — в LRU.
4. **Admin Save**: после успешного сохранения в PG, профиль пишется в Valkey (write-through), публикуется PubSub `profile.invalidate:<slug>`.
5. **Degraded mode** (Valkey недоступен): чтение идёт через LRU (метаданные) + PG (словарь), запись — только в PG; warning-лог, метрика cache_stale.
6. Все gateway-инстансы получают PubSub-сообщение `profile.invalidate:<slug>` и удаляют запись из LRU (Valkey-ключ истечёт по TTL).

## User Stories

- P1 Story: Разработчик gateway: полный профиль (со словарём) закэширован в Valkey; `/mask` делает 1 round trip, без обращения к PG.
- P2 Story: SRE: при падении Valkey gateway читает из LRU + PG (только словарь); метрики показывают cache_hits/stale.

## MVP Slice

- `ProfileCache` — composite repository над `ProfileRepository`: Valkey (full profile) + LRU (metadata) + PG.
- Valkey: full profile (metadata + dictionary entries), JSON, read/write-through, TTL.
- LRU: только метаданные профиля (slug, version, tenant, name, detectors, preprocessors — без entries словаря), ~300B per entry, bounded 10k.
- PubSub-инвалидация с PSUBSCRIBE.
- Graceful degradation: Valkey down → LRU metadata + PG dict.
- Метрики Prometheus.

## First Deployable Outcome

Интеграционный тест: `ProfileCache` с mock Valkey + mock PG. Сохраняет профиль через cache. Через `FindBySlug`: проверяет Valkey hit (полный профиль, без PG). Симулирует отказ Valkey: проверяет что читает LRU + PG dict. Симулирует update через Save: проверяет version bump в Valkey.

## Scope

- `src/internal/adapters/repository/profile/cached.go` — `ProfileCache` (Valkey primary, LRU fallback, PG secondary)
- `src/internal/adapters/repository/profile/valkey.go` — `ProfileValkeyRepo` (get/set/del профиля)
- `src/internal/adapters/repository/profile/lru.go` — `ProfileMetadata` + `ProfileLRUCache` (метаданные без dict entries)
- `src/internal/adapters/repository/profile/pubsub.go` — подписка/обработка инвалидации
- Конфигурация: новый блок `profile_cache` в Config (Valkey TTL, LRU size).
- Метрики: `maskchain_profile_cache_hits_total`, `maskchain_profile_cache_misses_total`, `maskchain_profile_cache_stale_total`, `maskchain_profile_cache_invalidations_total`.
- `src/internal/adapters/repository/profile/warm.go` — `ProfileCacheWarmer`, фоновая горутина прогрева кэша при старте gateway.
- Интеграционные/unit-тесты для всех сценариев.

## Контекст

- Существующий `PostgresProfileRepo` — единственная имплементация `ProfileRepository`; новая имплементация делегирует ему запись/чтение PG.
- Valkey уже используется в проекте (`valkey-go`, `ValkeyConfig`, маски); `ValkeyConfig.Addr` переиспользуется.
- LRU-библиотеки в проекте нет (`hashicorp/golang-lru` не в go.mod) — будет добавлена.
- PubSub в valkey-go — через `D` (Do) для publish и `Receive` для subscribe.
- Gateway — основной потребитель чтения профилей (hot path `/mask`); admin — источник изменений.
- Profile содержит связанные сущности (dictionaries с 1k+ entries), которые кэшируются в Valkey, но НЕ в LRU.

## Зависимости

- `github.com/hashicorp/golang-lru/v2` — in-memory LRU cache (2nd level, metadata only).
- `github.com/valkey-io/valkey-go` — уже в проекте, используется для кэша и PubSub.
- Существующий `PostgresProfileRepo` — primary storage.
- Существующий `ValkeyConfig` (Addr, Password, TTLSec) — конфигурация подключения.

## Требования

- RQ-001 Система ДОЛЖНА поддерживать двухуровневый кэш профилей: Valkey (full profile, TTL) + in-memory LRU (metadata only, bounded).
- RQ-002 Ключи в Valkey ДОЛЖНЫ иметь формат `profile:<tenant_id>:<slug>:v<version>`, где `version` — монотонно возрастающий счётчик из PG, а `tenant_id` исключает коллизии между tenant'ами.
- RQ-003 При чтении профиля (`FindBySlug`/`FindByID`) система ДОЛЖНА сначала читать Valkey; при попадании — возвращать полный профиль без обращения к PG.
- RQ-004 При промахе в Valkey и успешном чтении из PG система ДОЛЖНА записать полный профиль в Valkey и метаданные (без entries словаря) в LRU.
- RQ-005 При создании/обновлении профиля (`Save`) система ДОЛЖНА записать профиль в PG и в Valkey (write-through), затем опубликовать PubSub-сообщение в канал `profile.invalidate:<slug>`. Если Valkey SET упал (единичная ошибка), но PG успешен — система ДОЛЖНА продолжить (Valkey-ключ устареет по TTL); метрика cache_stale инкрементируется.
- RQ-006 Gateway ДОЛЖЕН быть подписан через PSUBSCRIBE на pattem `profile.invalidate:*` и при получении сообщения удалять соответствующий ключ из LRU.
- RQ-007 При недоступности Valkey (ошибка соединения, timeout) система ДОЛЖНА читать метаданные из LRU + entries словаря из PG, писать только в PG; в лог ДОЛЖНО записываться warning-сообщение; метрика `maskchain_profile_cache_stale_total` ДОЛЖНА инкрементироваться. Если LRU не содержит метаданных — полное чтение из PG.
- RQ-008 System ДОЛЖНА экспортировать метрики Prometheus: cache_hits, cache_misses, cache_stale, cache_invalidations с лейблами `{operation="find_by_slug|find_by_id|save"}` и `{level="lru|valkey|pg"}`.
- RQ-009 TTL для Valkey-ключей ДОЛЖЕН быть конфигурируемым через `profile_cache.valkey_ttl_sec` (default 300 = 5 минут).
- RQ-010 Размер in-memory LRU ДОЛЖЕН быть конфигурируемым через `profile_cache.lru_size` (default 10000) и bounded — при превышении вытесняется LRU-политикой.
- RQ-011 При удалении профиля (`Delete`) система ДОЛЖНА удалить ключ из Valkey, удалить из LRU и опубликовать PubSub-сообщение `profile.invalidate:<slug>` для инвалидации на других инстансах.
- RQ-012 Подписка на каналы инвалидации ДОЛЖНА использовать PSUBSCRIBE с pattem `profile.invalidate:*`; при reconnect подписка ДОЛЖНА восстанавливаться автоматически.
- RQ-013 Gateway ДОЛЖЕН при старте в фоновой горутине прогреть кэш: загрузить активные профили из PG, для каждого — проверить Valkey (если есть — заполнить LRU), если нет — полное чтение из PG и запись в Valkey + LRU. Ошибки прогрева НЕ ДОЛЖНЫ блокировать старт gateway.

## Вне scope

- Кэширование других сущностей (dictionaries отдельно, incidents, mask entries) — только профили целиком.
- Persistence LRU через restart — LRU in-memory, сброс при перезапуске.
- Hot-reload конфигурации профильного кэша.
- Админский UI для просмотра/очистки кэша.
- Rate limiting запросов gateway к кэшу/PG.
- Кэширование списков (`ListByTenant`) — только одиночные чтения.
- Кэширование dictionary entries отдельным ключом — весь словарь частью full profile в Valkey.

## Критерии приемки

### AC-001 Read-through: промах в Valkey, чтение из PG, запись в кэш

- Почему это важно: гарантирует, что при первом чтении наполняются оба уровня кэша, и последующие запросы берут данные из Valkey.
- **Given** Valkey пуст, LRU пуст, в PG есть профиль `my-profile` с version=1 и словарём из 3 entries
- **When** `ProfileCache.FindBySlug(ctx, tenantID, "my-profile")` вызван
- **Then** возвращается полный профиль (со словарём), полный профиль записан в Valkey (ключ `profile:<tenantID>:my-profile:v1`), метаданные (без entries) записаны в LRU
- Evidence: mock PG — один вызов FindBySlug; mock Valkey — один Set с полным профилем; LRU содержит ProfileMetadata без dictionary entries

### AC-002 Write-through: Save пишет в PG и Valkey

- Почему это важно: гарантирует, что кэш консистентен после записи.
- **Given** профиль `my-profile` (новый, version=1, со словарём)
- **When** `ProfileCache.Save(ctx, profile)` вызван
- **Then** профиль сохранён в PG, полный профиль записан в Valkey (ключ `profile:<tenantID>:my-profile:v1`), опубликовано PubSub-сообщение `profile.invalidate:my-profile`
- Evidence: mock PG — один Exec; mock Valkey — один Set + один Publish

### AC-003 Valkey hit: чтение из Valkey без обращения к PG

- Почему это важно: основная цель — 1 round trip на hot path `/mask`.
- **Given** в Valkey есть ключ `profile:<tenantID>:my-profile:v2` с валидным профилем (включая словарь)
- **When** `ProfileCache.FindBySlug(ctx, tenantID, "my-profile")` вызван
- **Then** полный профиль возвращён из Valkey; PG не опрашивался; профиль содержит словарь
- Evidence: mock PG — 0 вызовов; возвращённый профиль имеет version=2 и непустой словарь

### AC-004 LRU fallback при недоступности Valkey (metadata есть)

- Почему это важно: при отказе Valkey gateway продолжает работать без падения.
- **Given** Valkey недоступен, в LRU есть ProfileMetadata для `my-profile` (version=3), в PG есть полный профиль (включая словарь)
- **When** `ProfileCache.FindBySlug(ctx, tenantID, "my-profile")` вызван
- **Then** метаданные прочитаны из LRU; entries словаря догружены из PG; полный профиль собран и возвращён; warning-лог; метрика cache_stale инкрементирована
- Evidence: mock Valkey возвращает ошибку; LRU содержит metadata; mock PG — один вызов FindBySlug или dictRepo; проверены logger.Warn и метрика

### AC-005 PubSub инвалидация на всех gateway инстансах

- Почему это важно: кэш-когерентность между репликами gateway без polling.
- **Given** gateway-1 подписан через PSUBSCRIBE на pattem `profile.invalidate:*`, в его LRU есть ProfileMetadata для `my-profile`
- **When** admin публикует `profile.invalidate:my-profile` через PUBLISH
- **Then** gateway-1 получает сообщение по PSUBSCRIBE и удаляет `my-profile` из LRU
- Evidence: после получения сообщения, `FindBySlug` для `my-profile` не находит metadata в LRU

### AC-006 Graceful degradation: Valkey недоступен, LRU пуст

- Почему это важно: при холодном старте и отказе Valkey система корректно читает из PG.
- **Given** Valkey недоступен, LRU пуст, профиль есть в PG
- **When** `ProfileCache.FindBySlug(ctx, tenantID, "my-profile")` вызван
- **Then** полный профиль прочитан из PG; метаданные записаны в LRU; warning-лог; метрика cache_stale инкрементирована
- Evidence: mock Valkey возвращает ошибку; mock PG — один FindBySlug; LRU содержит ProfileMetadata после вызова

### AC-007 Инвалидация по version bump при обновлении PG

- Почему это важно: stale-кэш не должен возвращаться после обновления профиля.
- **Given** в Valkey есть ключ `profile:<tenantID>:my-profile:v1`, в LRU — ProfileMetadata для `my-profile` с version=1
- **When** `PostgresProfileRepo.Save` обновляет профиль (version инкрементируется до 2)
- **Then** после Save: Valkey содержит `profile:<tenantID>:my-profile:v2` с новыми данными, старый ключ v1 отсутствует
- Evidence: FindBySlug возвращает version=2; старый ключ v1 не существует в Valkey

### AC-009 Delete инвалидирует кэш на всех инстансах

- Почему это важно: stale-данные не должны возвращаться после удаления профиля.
- **Given** в Valkey есть ключ `profile:<tenantID>:deleted-profile:v1`, в LRU gateway-1 есть ProfileMetadata
- **When** `ProfileCache.Delete(ctx, profileID)` вызван
- **Then** профиль удалён из PG, ключ удалён из Valkey, LRU gateway-1 очищен, опубликовано PubSub-сообщение `profile.invalidate:deleted-profile`
- Evidence: mock PG — один Exec (DELETE); mock Valkey — один Del + один Publish; LRU не содержит ProfileMetadata

### AC-010 Cache warming при старте gateway

- Почему это важно: первый `/mask` после рестарта gateway должен находить профиль в кэше, а не грузить PG.
- **Given** gateway только что запущен; LRU пуст; Valkey пуст; в PG есть 2 active профиля
- **When** фоновая горутина прогрева завершила работу
- **Then** оба профиля присутствуют в Valkey (full profile) и в LRU (metadata). Если один из профилей уже был в Valkey — LRU заполнен metadata, Valkey не перезаписан.
- Evidence: после прогрева `FindBySlug` для обоих профилей возвращает данные без обращения к PG; `cache_misses_total` не инкрементирован (warm не считаем miss)

### AC-008 Метрики кэша экспортируются

- Почему это важно: observability для SRE.
- **Given** `ProfileCache` инициализирован с Prometheus регистрацией
- **When** выполнены операции: hit (Valkey), miss (PG + read-through), stale (Valkey ошибка + LRU fallback), invalidation
- **Then** метрики `maskchain_profile_cache_hits_total`, `maskchain_profile_cache_misses_total`, `maskchain_profile_cache_stale_total`, `maskchain_profile_cache_invalidations_total` присутствуют в /metrics с корректными значениями
- Evidence: HTTP GET /metrics содержит все четыре метрики с ненулевыми значениями

## Допущения

- Версия профиля монотонно возрастает (PG `ON CONFLICT ... version = version + 1`); это гарантирует свежесть ключей.
- PubSub-сообщения могут быть потеряны при краше — последующее чтение подхватит актуальную версию из Valkey (TTL) или PG.
- Все gateway-инстансы подключаются к одному Valkey instance/sentinel (shared-nothing для LRU, shared для Valkey).
- `ListByTenant` не кэшируется, так как профили списком нужны только в admin UI (редкий запрос).
- `Delete` публикует PubSub-сообщение `profile.invalidate:<slug>` (как и Save), что обеспечивает LRU-инвалидацию на всех gateway-инстансах.

## Критерии успеха

- SC-001 Чтение профиля при Valkey hit < 5ms p99; при degraded fallback (LRU + PG dict) < 50ms p99.
- SC-002 При недоступности Valkey latency чтения профиля не превышает PG latency + LRU lookup (< 1ms overhead).

## Краевые случаи

- Transient write-through failure: PG.Save успешен, Valkey.Set/Publish упал — профиль сохранён в PG, кэш устареет по TTL; eventual consistency.
- Valkey недоступен при старте — gateway работает в degraded mode (LRU + PG), метрика cache_stale.
- LRU overflow (burst > 10k уникальных профилей) — вытеснение по LRU-политике.
- Профиль удалён из PG — инвалидация кэша (Valkey DEL + LRU evict + PubSub).
- PubSub reconnect — valkey-go автоматически переподключается, подписка восстанавливается (resubscribe).
- Конкурирующие записи в один профиль — последний Save побеждает (version = version + 1, last write wins).
- Dictionary >1k entries — кэшируется полный JSON в Valkey, LRU не содержит entries.

## Открытые вопросы

- none — все ключевые решения покрыты текущими допущениями.

Готово к: /spk.plan 102-profile-cache
