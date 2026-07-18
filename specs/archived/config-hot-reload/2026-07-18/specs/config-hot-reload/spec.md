# Hot-reload конфигурации без перезапуска процесса

## Scope Snapshot

- In scope: горячая перезагрузка runtime-секций конфига (routing, tenants, shield, ratelimit, debug) через fsnotify-наблюдение за директорией, без рестарта процесса и без downtime.
- Out of scope: hot-reload секций, требующих реинициализации соединений (database, valkey, otel), динамическое изменение listen-портов.

## Цель

Оператор/администратор меняет ConfigMap или файлы в `/etc/maskchain/conf.d/` и через fsnotify подхватывает изменения без kubectl rollout restart и без прерывания активных запросов. Успех фичи измеряется тем, что изменение routing/tenants/shield применяется без единого dropped-запроса.

## Основной сценарий

1. Процесс запущен с `--config-dir=/etc/maskchain/conf.d/` и штатно обслуживает трафик
2. Оператор изменяет `99-config-runtime.yaml` (добавляет провайдера, меняет правило роутинга)
3. fsnotify детектит запись файла, debounce 100ms, процесс загружает новый конфиг через `LoadConfigFromDir`
4. diff-секции: изменённые runtime-компоненты бесшовно обновляются (provider registry, router, tenant store, shield mapping, ratelimit)
5. Нетронутые зависимости (PG pool, Valkey conn, http-клиенты) продолжают работать с прежними настройками
6. Ошибка в новом конфиге логируется, процесс продолжает работу со старым конфигом, активные запросы не прерываются

## User Stories

- P1 Story: администратор меняет routing-правила — система применяет их без перезапуска, запросы не дропаются
- P2 Story: администратор меняет tenant keys — клиенты продолжают работать, новые ключи начинают действовать без rollout

## MVP Slice

Горячая перезагрузка `routing.providers` + `routing.rules` + `tenants.*` с fsnotify, debounce, graceful error handling (rollback на старый конфиг при ошибке). AC-001, AC-002, AC-003, AC-004.

## First Deployable Outcome

После одного implementation pass можно: запустить бинарь с `--config-dir`, изменить routing-файл, увидеть в логах "config reloaded: routing updated" и проверить curl-ом, что новый провайдер/правило работает, а старые запросы не упали.

## Scope

- fsnotify watcher директории `/etc/maskchain/conf.d/` (или `CONFIG_DIR`)
- diff-merge старого и нового `*Config`: обновление только изменившихся секций
- Обновляемые секции: `routing`, `tenants`, `shield`, `ratelimit`, `debug`
- Graceful rollback при ошибке парсинга/валидации нового конфига
- Observable логирование: `config reloaded: routing changed`, `config reload error: ...`
- Support в `src/cmd/internal/bootstrap/` для проброса fsnotify + канала перезагрузки
- Тесты: unit на diff-merge + log-based integration на fsnotify

## Контекст

- Уже есть `LoadConfigFromDir(dir)` с deep-merge и валидацией
- ConfigMap в Kubernetes синкается ~30-60s (kubelet sync), fsnotify на read-only файл сработает при каждой синке — нужен debounce
- Helm chart уже разделяет `config-base` и `config-runtime`, но fsnotify будет наблюдать всю директорию — изменения base не должны ломать runtime
- `server.port`, `database.dsn`, `valkey.addr` и другие base-секции не reload-ятся — это intentional
- Процесс уже использует `slog` — изменение `log.level` подхватится автоматически (через динамический handler), не требует fsnotify

## Зависимости

- `github.com/fsnotify/fsnotify` — уже есть в go.sum (transitive от других пакетов) или нужно явно добавить
- Нет меж-спековых зависимостей

## Требования

- RQ-001 Система ДОЛЖНА наблюдать директорию конфига через fsnotify и применять изменения без restart процесса
- RQ-002 Система ДОЛЖНА логировать каждый успешный reload с diff-ом изменённых секций
- RQ-003 При ошибке в новом конфиге система ДОЛЖНА продолжить работу со старым конфигом и записать ошибку в лог
- RQ-004 Система ДОЛЖНА debounce-ить множественные события fsnotify в одном временном окне (100ms)
- RQ-005 reload НЕ ДОЛЖЕН прерывать активные HTTP-запросы (graceful swap, не блокирующий мьютекс)
- RQ-006 reload НЕ ДОЛЖЕН затрагивать секции, не входящие в runtime (database, valkey, otel, server, egress)

## Вне scope

- Перезагрузка `base`-секций (database, valkey, otel, server, egress, session, mask, dictionary_cache)
- reload `log.level` через fsnotify (уже работает через динамический slog handler)
- reload OTEL endpoint (требует переинициализации tracer provider)
- Reconnect PostgreSQL/Valkey при смене dsn/addr
- Dynamic listen port (server.port, admin_port)
- reload через SIGHUP (альтернативный механизм — отложен)
- Оператор Kubernetes (watcher на CRD-ресурсы)

## Критерии приемки

### AC-001 Изменение routing применяется без перезапуска

- Почему это важно: оператор добавляет провайдера или меняет правила — запросы должны сразу пойти по новому маршруту, без даунтайма
- **Given** процесс запущен с `--config-dir` и обслуживает трафик
- **When** оператор изменяет `routing` секцию в runtime-файле
- **Then** в логе появляется `"config reloaded"` + `"routing"` в diff, а следующий curl-запрос идёт по новому маршруту
- Evidence: grep лога по `config reloaded` + curl, подтверждающий новый маршрут

### AC-002 Изменение tenants применяется без перезапуска

- Почему это важно: добавление/отзыв API-ключа тенанта не должен требовать переката подов в кластере
- **Given** процесс запущен с `--config-dir` и обслуживает трафик
- **When** оператор изменяет `tenants` секцию (добавляет ключ)
- **Then** запрос с новым ключом авторизуется, старый ключ продолжает работать
- Evidence: curl с новым ключом → 200, curl со старым ключом → 200

### AC-003 Ошибка в новом конфиге не роняет процесс

- Почему это важно: битый config (синтаксическая ошибка, невалидное поле) не должен убивать процесс и вызывать outage
- **Given** процесс запущен со штатным конфигом
- **When** оператор записывает синтаксически невалидный YAML или конфиг с отсутствующим required-полем
- **Then** процесс продолжает работу со старым конфигом, в логе ошибка парсинга
- Evidence: curl-запросы продолжают работать, в stderr/log записана ошибка

### AC-004 debounce-событий

- Почему это важно: kubelet триггерит несколько inotify-событий при синке ConfigMap; без debounce каждый вызовет reload
- **Given** fsnotify-наблюдение активно
- **When** в течение 100ms происходит 5+ файловых событий (write + chmod + attrib)
- **Then** reload запускается ровно один раз
- Evidence: лог "config reloaded" выведен 1 раз (не 5), тайминг между событиями и reload-ом < 200ms с момента последнего события

### AC-005 reload не блокирует активные запросы

- Почему это важно: middleware не должен подвисать на время lock-а; старый router должен дообслуживать in-flight запросы
- **Given** процесс обрабатывает конкурентные запросы (10 RPS+)
- **When** происходит reload конфига
- **Then** ни один запрос не получает 5xx, вызванных reload (graceful pointer swap)
- Evidence: под нагрузкой (`hey` / `wrk`) во время reload — нет 5xx, спровоцированных reload

### AC-006 reload не затрагивает base-секции

- Почему это важно: замена runtime YAML не должна переинициализировать пул соединений или сбрасывать egress-клиенты
- **Given** процесс использует `database.dsn`, `valkey.addr`, `egress` из base
- **When** происходит reload runtime-конфига
- **Then** database/pool/valkey/HTTP-клиенты не пересоздаются, статистика пула сохраняется
- Evidence: после reload pg_stat_activity показывает те же соединения, egress-клиенты не пересоздаются (observable через metrics)

## Допущения

- fsnotify срабатывает на read-only mounts от ConfigMap (kubelet делает chmod после записи)
- sync ConfigMap → pod занимает до 30-60s; fsnotify видит изменения сразу после того, как volume обновлён
- Процесс уже стартовал с валидным конфигом; нет сценария "первый запуск без конфига"
- slog-level меняется в runtime автоматически (через динамический handler) — это не фича данного spec
- Все runtime-секции (routing, tenants, shield, ratelimit, debug) не блокируют main goroutine при инициализации

## Критерии успеха

- SC-001 reload < 50ms (без латентности для active запросов)
- SC-002 Конкурентный reload (2+ событий подряд) не вызывает race condition или deadlock

## Краевые случаи

- Пустая директория конфига — процесс на старте падает с ошибкой (как сейчас)
- reload при пустом файле (0 байт) — не роняет процесс, логирует ошибку
- reload при удалении runtime-файла — процесс продолжает со старым конфигом
- reload при изменении base-файла — base-секции игнорируются, в логе warning
- 10 reload-ов подряд — все успешны, без утечки goroutines/handlers

## Открытые вопросы

- `none`
