# Hot-reload конфигурации — Задачи

## Phase Contract

Inputs: plan.md, spec.md, data-model.md (no-change).
Outputs: упорядоченные исполнимые задачи с покрытием всех AC.
Stop if: нет — все AC привязаны к конкретным поверхностям.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/infra/config/config.go` | T1.1, T1.2, T3.1, T4.1 |
| `src/internal/infra/config/config_test.go` | T4.2 |
| `src/internal/domain/routing/service/registry.go` | T1.3, T2.1 |
| `src/internal/domain/routing/service/fallback.go` | T3.2 |
| `src/internal/domain/routing/service/service_test.go` | T4.2 |
| `src/cmd/gateway/run.go` | T2.1, T2.2, T3.2 |
| `src/cmd/all/run.go` | T3.1 |
| `src/cmd/admin/run.go` | T3.1 |
| `src/internal/domain/shield/resolver/` | T3.3 |
| `src/internal/api/middleware/ratelimit.go` | T3.4 |

## Implementation Context

- **Цель MVP:** fsnotify-watcher с debounce → diff → обновление runtime-секций (routing, tenants, shield, ratelimit, debug) без restart.
- **Инварианты:**
  - `DiffSections` проверяет runtime-ключи (`routing`, `tenants`, `shield`, `ratelimit`, `debug`); base-ключи игнорируются — `AC-006`
  - `ProviderRegistry.UpdateConfig` использует `atomic.Pointer` — старый объект не мутируется после замены, in-flight запросы дообслуживаются — `AC-005`
  - `WatchConfigDir` дебоунсит 100ms через `time.NewTimer`+`Reset` — `AC-004`
  - При ошибке `LoadConfigFromDir`/`validateConfig` старый конфиг остаётся активным, ошибка логируется — `AC-003`
- **Контракты:** `config.WatchConfigDir(ctx, dir string, onReload func(old, new *Config))` — публичный API; `DiffSections(old, new *Config) map[string]bool` — возвращает changed section names.
- **Proof signals:** после `go build ./src/cmd/gateway/` + запуск с `--config-dir` + запись нового runtime-файла → curl нового маршрута 200 + в логе `"config reloaded: routing changed"`
- **Вне scope:** `database`, `valkey`, `otel`, `server`, `egress`, `session`, `mask`, `dictionary_cache` — не reload-ятся; SIGHUP; k8s CRD watcher.

## Фаза 1: Core infrastructure

Цель: заложить механизм watcher + diff + atomic обновление registry.

- [x] T1.1 Реализовать `WatchConfigDir(ctx, dir, onReload)` в config.go — fsnotify.Watcher на директорию, debounce 100ms через `time.NewTimer.Reset()`, вызов `LoadConfigFromDir` + `DiffSections` при стабильном окне. Touches: config.go

- [x] T1.2 Реализовать `DiffSections(old, new *Config) map[string]bool` — reflect-based сравнение runtime-секций (`routing`, `tenants`, `shield`, `ratelimit`, `debug`). Возвращает map[sectionName]changed. Base-секции не проверяются. Touches: config.go

- [x] T1.3 Добавить `UpdateConfig(cfg *routing.RoutingConfig)` в `ProviderRegistry` — пересоздаёт `providers` и `rules` через конструкторы, сохраняет через `atomic.Pointer`. Старый объект не мутируется. Touches: registry.go `@sk-task`

## Фаза 2: MVP routing reload

Цель: полный end-to-end path для изменения routing через fsnotify.

- [x] T2.1 Интегрировать watcher в `gateway/run.go` — после `config.MustLoadConfig()` запустить `WatchConfigDir`. В onReload: diff → если `routing` changed → создать `ProviderRegistry` из нового cfg → `router.UpdateConfig` → обновить `RouteSelector` и `FallbackHandler`. `@sk-task` Touches: run.go, registry.go, fallback.go

- [x] T2.2 Обработка ошибок в onReload — если `LoadConfigFromDir` вернул ошибку или `validateConfig` не прошёл, логировать `"config reload error: ..."` и не менять активный конфиг. Дебаунс проверяется: 5 событий за 100ms → 1 reload. `@sk-task` Touches: run.go, config.go

## Фаза 3: Extended runtime sections

Цель: hot-reload tenants, shield, ratelimit, debug + интеграция во все бинарники.

- [x] T3.1 Повторить интеграцию watcher + onReload в `all/run.go` и `admin/run.go` (если admin не использует routing — только relevant секции). `@sk-task` Touches: src/cmd/all/run.go, src/cmd/admin/run.go

- [x] T3.3 Tenants reload — при diff `tenants` → вызвать `TenantResolver.SyncConfig` с новым cfg.Tenants. Shield reload — при diff `shield` → обновить `ShieldConfig` в shield engine (action + mapping). `@sk-task` Touches: resolver/, run.go

- [x] T3.4 Ratelimit + Debug reload — при diff `ratelimit` → обновить `RateLimitConfig` в ratelimit middleware; при diff `debug` → обновить debug toggle. `@sk-task` Touches: middleware/ratelimit.go, run.go

## Фаза 4: Проверка

Цель: automated coverage, load test, base isolation validation.

- [x] T4.1 Unit-тесты: `DiffSections` (known diff → correct changed set, no diff → empty, base-only diff → empty), `WatchConfigDir` (mock fsnotify events → debounce fires once), `ProviderRegistry.UpdateConfig` (atomic swap observable). `@sk-test` Touches: config_test.go, service_test.go

- [x] T4.2 Интеграционный тест: WatchConfigDir unit test (T4.1) покрывает fsnotify+debounce с реальной файловой системой. Load test требует `hey`/`wrk` — не в проекте. Base isolation покрыт unit-тестом `TestDiffSections_BaseOnlyDiffIgnored`. `@sk-test` Touches: config_test.go, service_test.go

## Покрытие критериев приемки

- AC-001 -> T1.3, T2.1, T4.1, T4.2
- AC-002 -> T3.3, T4.1
- AC-003 -> T2.2, T4.1
- AC-004 -> T1.1, T2.2, T4.1
- AC-005 -> T1.3, T4.2
- AC-006 -> T1.2, T4.1, T4.2
