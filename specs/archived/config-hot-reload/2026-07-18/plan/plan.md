# Hot-reload конфигурации — План

## Phase Contract

Inputs: spec, inspect (pass), repo-context (config, bootstrap, routing, shield, gateway/all run).
Outputs: plan, data-model (no-change).
Stop if: нет.

## Цель

Добавить fsnotify-наблюдение директории конфига (CONFIG_DIR) и diff-merge обновление runtime-секций (routing, tenants, shield, ratelimit, debug) без перезапуска процесса.

## MVP Slice

- fsnotify watcher на `--config-dir` → debounce 100ms → `LoadConfigFromDir` → diff → update только changed секций
- ProviderRegistry.UpdateConfig, FallbackHandler.UpdateClients, RouteSelector (авто через pointer)
- ShieldEngine обновление action + mapping
- RateLimit middleware обновление cfg
- Debug toggle
- Graceful error rollback
- AC-001 (routing), AC-003 (error rollback), AC-004 (debounce) — первый инкремент

## First Validation Path

```bash
go build ./src/cmd/gateway/ && ./gateway --config-dir=/tmp/test-conf &
# меняем routing
echo 'routing: {providers: [{name: test, api_type: openai, base_url: "http://test", api_keys: ["k"], timeout: 1s, priority: 1}], rules: [{tenant: default, routes: [{model: test, providers: [test]}]}]}' > /tmp/test-conf/99-config-runtime.yaml
# ждём ~200ms — проверяем лог "config reloaded" + curl нового маршрута
```

## Scope

- `src/internal/infra/config/config.go` — `WatchConfigDir(ctx, dir, onReload func(old, new *Config))` + `DiffSections(old, new) map[string]bool`
- `src/internal/domain/routing/service/registry.go` — `UpdateConfig(*routing.RoutingConfig)`
- `src/internal/domain/routing/service/fallback.go` — `UpdateClients(map[string]ports.ProviderClient)`
- `src/cmd/gateway/run.go`, `src/cmd/admin/run.go`, `src/cmd/all/run.go` — проброс watcher + обработчики
- `src/internal/api/server.go` — метод `ReloadConfig(cfg)` или канал перезагрузки для middleware
- `src/internal/domain/shield/` — hot-reload shield config (action, mapping)
- `src/internal/domain/budget/ratelimit.go` — hot-reload ratelimit config

## Performance Budget

- reload latency < 50ms (время от fsnotify event до применения нового конфига)
- reload не блокирует активные запросы — atomic pointer swap
- `none` для memory/alloc — reload редкий (часы-дни)

## Implementation Surfaces

| Surface | Тип | Причина |
|---------|-----|---------|
| `src/internal/infra/config/config.go` | existing | +WatchConfigDir, +DiffSections, +diffSections |
| `src/internal/domain/routing/service/registry.go` | existing | +UpdateConfig для провайдеров и правил |
| `src/internal/domain/routing/service/fallback.go` | existing | +UpdateClients для создания/удаления клиентов |
| `src/internal/api/server.go` | existing | +ReloadConfig или канал для обновления middleware |
| `src/internal/domain/shield/resolver/` | existing | обновление shield config через resolver/TenantResolver |
| `src/internal/api/middleware/ratelimit.go` | existing | обновление RateLimitConfig |
| `src/cmd/gateway/run.go`, `admin/run.go`, `all/run.go` | existing | проброс watcher и reload handlers |

## Bootstrapping Surfaces

- `none` — вся структура уже в репозитории

## Влияние на архитектуру

- WatchConfigDir — новый публичный API в config пакете
- ProviderRegistry получает мутабельный метод UpdateConfig (ранее только read-only)
- FallbackHandler получает UpdateClients (пересоздание provider-клиентов при изменении api_keys/base_url)
- Server может получить коллбек для обновления middleware (shield, ratelimit)
- Никаких брейкингов — все изменения аддитивны, существующие вызовы не меняются

## Acceptance Approach

| AC | Подход | Поверхности | Наблюдение |
|----|--------|-------------|------------|
| AC-001 routing | fsnotify → LoadConfigFromDir → diff → ProviderRegistry.UpdateConfig | config.go, registry.go | лог "config reloaded: routing" + curl нового провайдера |
| AC-002 tenants | fsnotify → diff → TenantResolver.SyncConfig | config.go, resolver.go | curl с новым tenant key → 200 |
| AC-003 error | fsnotify → parse error → log + skip | config.go | curl продолжает работать, в логе ошибка |
| AC-004 debounce | fsnotify multi-event → time.AfterFunc(100ms) → single reload | config.go | лог 1 раз, не 5 |
| AC-005 non-blocking | atomic.Pointer на registry/selector | registry.go | load test без 5xx |
| AC-006 base isolation | diffSections проверяет changed keys, base секции игнорируются | config.go | после reload pg_stat_activity то же число соединений |

## Данные и контракты

- Data model не меняется — ни одна сущность не получает новых полей
- `data-model.md` — no-change stub (см. ниже)
- API контракты не меняются
- Новый публичный API: `config.WatchConfigDir(ctx, dir, onReload)`, `DiffSections(old, new *Config) map[string]bool`
- Новый API домена: `ProviderRegistry.UpdateConfig(cfg)`, `FallbackHandler.UpdateClients(clients)`

## Стратегия реализации

### DEC-001 Atomic pointer swap для router

- Why: ProviderRegistry создаётся один раз при старте, передаётся по указателю в RouteSelector, HealthChecker, FallbackHandler. Замена указателя atomic.Pointer — не блокирует читателей.
- Tradeoff: старый registry живёт пока есть in-flight запросы (но GC соберёт после). Для полной гарантии — sync.RWMutex, но atomic достаточно.
- Affects: domain/routing/service/registry.go, selector.go
- Validation: race detector + load test (AC-005)

### DEC-002 Пересоздание provider-клиентов при изменении api_keys/base_url

- Why: FallbackHandler держит map[string]ports.ProviderClient (http.Client с auth). При изменении API-ключа или base_url старые клиенты не обновляются — нужен Replace. Создаём новые, заменяем map атомарно, старые дообслуживают in-flight.
- Tradeoff: дороже, чем просто обновить registry (новые http.Client), но неизбежно при смене auth.
- Affects: domain/routing/service/fallback.go, gateway/run.go (buildProviderClients)
- Validation: curl-запрос после смены api_key — успех

### DEC-003 WatchConfigDir — отдельная goroutine с debounce

- Why: не блокировать main goroutine. Debounce через `time.AfterFunc` или `time.NewTimer`, reset при каждом событии.
- Tradeoff: 100ms задержки между записью файла и reload — приемлемо для конфига (не real-time).
- Affects: infra/config/config.go
- Validation: AC-004

### DEC-004 diffSections — mapstructure-based diff

- Why: сравнивать два `*Config` целиком через reflect итеративно по известным runtime-ключам. Если секция не изменилась — не обновляем.
- Tradeoff: reflect на каждый reload — микросекунды. Альтернатива: md5sum файлов. Выбран reflect — не нужно хранить хеши.
- Affects: infra/config/config.go
- Validation: unit test с известным diff

## Incremental Delivery

### MVP (routing + error rollback + debounce)

- fsnotify watcher + debounce
- LoadConfigFromDir при событии
- diffSections (только routing)
- ProviderRegistry.UpdateConfig
- Log + curl validation
- AC-001, AC-003, AC-004

### Итеративное расширение

1. **Tenants**: diff tenants → TenantResolver.SyncConfig — AC-002
2. **Shield + RateLimit + Debug**: diff → обновление middleware config — AC-006 часть
3. **FallbackHandler.UpdateClients**: пересоздание provider-клиентов при изменении api_keys/base_url
4. **Base isolation + load test**: AC-005, AC-006 (полный)

## Порядок реализации

1. `WatchConfigDir` + `DiffSections` + debounce в config.go (ядро)
2. `ProviderRegistry.UpdateConfig` в registry.go
3. Проброс watcher в gateway/run.go — reload routing (MVP)
4. Та же интеграция в all/run.go и admin/run.go
5. Tenants → SyncConfig при diff
6. Shield/RateLimit/Debug → обновление middleware
7. FallbackHandler.UpdateClients
8. Load test + metrics validation
9. Base isolation unit test

## Риски

| Риск | Mitigation |
|------|------------|
| fsnotify на read-only mount (ConfigMap) не срабатывает | kubelet делает chmod после записи → inotify. Добавить fallback poll interval (таймер 60s как резерв) |
| Race между reload и запросом во время atomic swap | atomic.Pointer safe; старый объект не мутируется после замены |
| ProviderClient создание дорогое | Создаём новые клиенты до замены; старые дообслуживают in-flight |
| Изменение shield или ratelimit может потребовать реинициализации middleware | Middleware принимает config по указателю; атомарная замена cfg-pointer. Gin handler перечитывает на каждый запрос |

## Rollout и compatibility

- Все изменения аддитивны — старый `MustLoadConfig()` без `--config-dir` продолжает работать
- `WatchConfigDir` запускается только если указан `--config-dir` или `CONFIG_DIR`
- Специальных rollout-действий не требуется

## Проверка

- Unit: `WatchConfigDir` (mock fsnotify events), `DiffSections`, `ProviderRegistry.UpdateConfig`
- Integration: запуск бинаря с `--config-dir`, запись файла, curl проверка
- Load test: AC-005 под `hey` / `wrk`
- Helm: template проверка, что `args: [--config-dir=...]` не сломался

## Соответствие конституции

- нет конфликтов
