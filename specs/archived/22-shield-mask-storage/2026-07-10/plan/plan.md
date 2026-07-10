# Mask Storage — План

## Phase Contract

Inputs: spec 22-shield-mask-storage, inspect pass.
Outputs: plan.md, data-model.md.
Stop if: spec слишком расплывчата для безопасного планирования — spec пройдена inspect с pass.

## Цель

Создать полный pipeline обратимого маскинга: MaskEntry entity, MaskStorage interface, MaskUseCase (MaskText/UnmaskText), Postgres+Valkey репозитории с write-through/read-through, CompositeDetector для объединения детекторов под одним типом, HTTP хендлеры `/mask` и `/unmask`, конфигурацию и DI в main.go. Всё покрыто unit-тестами.

## MVP Slice

MaskUseCase + in-memory storage + CompositeDetector + тесты. Покрывает AC-001–AC-007, AC-011. Проверяет базовую механику: entity, storage, mask/unmask round-trip, overlap filter, conflict, UUIDv7.

## First Validation Path

```bash
go test ./src/internal/domain/shield/mask/... -v
```

Зелёный прогон подтверждает: MaskUseCase работает, round-trip проходит, overlap filter корректен, conflict возвращается.

## Scope

- `src/internal/domain/shield/mask/` — новый пакет: entity, storage, usecase, uuid, errors
- `src/internal/domain/shield/detector/composite.go` — CompositeDetector (новый файл в существующем пакете)
- `src/internal/adapters/repository/mask/` — новый пакет: postgres.go, valkey.go, cached.go
- `src/internal/api/mask_handler.go` — HTTP хендлеры (новый файл в существующем пакете)
- `src/internal/api/server.go` — добавлен метод RegisterMaskHandler
- `src/internal/infra/config/config.go` — расширение Config: DatabaseConfig, ValkeyConfig, MaskConfig
- `src/cmd/gateway/main.go` — DI всех компонентов
- `deployments/migrations/001_mask_entries.sql` — DDL для PG

## Performance Budget

- `none`: mask/unmask — синхронные операции на тексте промпта (типично <4KB). P95 <50ms. Performance-тестирование — PostMVP.

## Implementation Surfaces

| Surface | Статус | Участие |
|---------|--------|---------|
| `src/internal/domain/shield/mask/entity.go` | new | MaskEntry entity |
| `src/internal/domain/shield/mask/storage.go` | new | MaskStorage interface |
| `src/internal/domain/shield/mask/errors.go` | new | Sentinel errors |
| `src/internal/domain/shield/mask/usecase.go` | new | MaskUseCase (MaskText, UnmaskText) |
| `src/internal/domain/shield/mask/uuid.go` | new | UUIDv7 generator |
| `src/internal/domain/shield/detector/composite.go` | new | CompositeDetector |
| `src/internal/adapters/repository/mask/postgres.go` | new | PostgresMaskRepo |
| `src/internal/adapters/repository/mask/valkey.go` | new | ValkeyMaskRepo |
| `src/internal/adapters/repository/mask/cached.go` | new | CachedMaskRepo |
| `src/internal/api/mask_handler.go` | new | HTTP handlers |
| `src/internal/api/server.go` | modified | RegisterMaskHandler |
| `src/internal/infra/config/config.go` | modified | Database/Valkey/Mask config |
| `src/cmd/gateway/main.go` | modified | DI wiring |
| `deployments/migrations/001_mask_entries.sql` | new | DDL |

## Bootstrapping Surfaces

- `src/internal/domain/shield/mask/` (директория)
- `src/internal/adapters/repository/mask/` (директория)

## Влияние на архитектуру

- Новые пакеты: `domain/shield/mask/` (domain), `adapters/repository/mask/` (infra).
- Новый файл `detector/composite.go` в существующем пакете — не breaking change.
- `server.go` расширен методом `RegisterMaskHandler` — существующие health-хендлеры не затронуты.
- `config.go` расширен опциональными секциями — обратная совместимость.
- `main.go` заново написан с DI — поведение не меняется при отсутствии PG/Valkey.

## Acceptance Approach

| AC | Подход | Surfaces | Наблюдение |
|----|--------|----------|------------|
| AC-001 | MaskStorage interface с Save/Get/Delete | storage.go, postgres.go, valkey.go, cached.go | Компиляция |
| AC-002 | MaskText с mock detector | usecase.go, usecase_test.go | Assert masked text и replacements |
| AC-003 | UnmaskText с предзаполненным storage | usecase.go, usecase_test.go | Assert restored text |
| AC-004 | Mask → Unmask round-trip | usecase.go, usecase_test.go | Assert restored == original |
| AC-005 | Overlap filter: longer wins | usecase.go, usecase_test.go | Assert только 1 замена (длинная) |
| AC-006 | Save с дубликатом mask_id | postgres.go, usecase_test.go | errors.Is(ErrMaskIDConflict) |
| AC-007 | UUIDv7 формат | uuid.go | Assert длина=36, version=7, variant=10xx |
| AC-008 | ON CONFLICT DO NOTHING | postgres.go | Код содержит SQL с DO NOTHING и проверку RowsAffected |
| AC-009 | TTL + key prefix mask: | valkey.go | Код использует Ex(r.ttl) и key("mask:"+id) |
| AC-010 | Write-through: Save→PG→Valkey; Read-through: Get→Valkey→PG→refresh | cached.go | Порядок вызовов в коде |
| AC-011 | CompositeDetector.Scan мерджит результаты | composite.go | Assert объединённое количество |
| AC-012 | nil pool/client → no-op/ErrNotFound | postgres.go, valkey.go | nil-guards в коде |

## Данные и контракты

- AC: см. таблицу выше.
- Data model: MaskEntry — новая persisted entity с JSONB-полем replacements. См. `data-model.md`.
- API contracts: два новых HTTP endpoinта `/api/v1/shield/mask` и `/api/v1/shield/unmask`.
- DB: новая таблица `mask_entries` (миграция `001_mask_entries.sql`).
- Кэш: Valkey key `mask:<id>` с TTL.

## Стратегия реализации

### DEC-001 MaskEntry хранит только replacements (не original_text)

- **Why:** unmask требует только отображения placeholder→original. Хранение original_text избыточно и увеличивает размер записи. Соответствует поведению RELAY encrypt/decrypt.
- **Tradeoff:** без original_text нельзя восстановить цепочку при потере replacements (коррупция JSONB). Приемлемо: replacements — единственный источник истины для unmask.
- **Affects:** entity.go, postgres.go (JSONB), valkey.go (JSON marshal)
- **Validation:** AC-002, AC-003, AC-004

### DEC-002 Longer match wins при пересечении совпадений

- **Why:** короткое совпадение внутри длинного (напр., "example.com" внутри "john@example.com") не должно заменяться отдельно — иначе unmask сломается. Предпочтение длины даёт наиболее полное замещение.
- **Tradeoff:** если более короткое совпадение семантически важнее (напр., номер карты внутри UUID), оно будет пропущено. В реальности детекторы не возвращают вложенных совпадений одного типа.
- **Affects:** usecase.go (sort + overlap filter)
- **Validation:** AC-005

### DEC-003 CachedMaskRepo — композитный репозиторий

- **Why:** разделение ответственности: PG — source of truth, Valkey — read cache. UseCase не знает о кэшировании.
- **Tradeoff:** дополнительный уровень абстракции. При отказе Valkey чтение идёт напрямую в PG (graceful degradation).
- **Affects:** cached.go, postgres.go, valkey.go
- **Validation:** AC-010

### DEC-004 nil-safe репозитории

- **Why:** приложение должно стартовать без PG/Valkey (dev-режим, тесты). nil pool/client → no-op вместо panic.
- **Tradeoff:** ошибки конфигурации могут быть не замечены до первого запроса. Компенсируется warn-логом при nil DSN/Addr.
- **Affects:** postgres.go, valkey.go
- **Validation:** AC-012

### DEC-005 CompositeDetector для множественных детекторов одного типа

- **Why:** DetectorRegistry keyed by DetectorType (regex/keyword/presidio), но все regex-детекторы одного типа. CompositeDetector объединяет их под одним ключом.
- **Tradeoff:** дополнительный уровень обёртки. Registry по-прежнему хранит один Detector на тип.
- **Affects:** composite.go
- **Validation:** AC-011

### DEC-006 UUIDv7 — на основе crypto/rand

- **Why:** стандарт RFC 9562, time-sortable, глобально уникален. crypto/rand — криптостойкий источник.
- **Tradeoff:** медленнее math/rand. На типичной нагрузке (единицы ID/сек) некритично.
- **Affects:** uuid.go
- **Validation:** AC-007

## Incremental Delivery

### MVP (Первая ценность)

MaskUseCase + in-memory storage + CompositeDetector + тесты. Покрывает AC-001–AC-007, AC-011.
Валидация: `go test ./src/internal/domain/shield/mask/... -v`.

### Итеративное расширение

1. PostgresMaskRepo (AC-008) — после MVP, т.к. требует PG.
2. ValkeyMaskRepo (AC-009) — после MVP, т.к. требует Valkey.
3. CachedMaskRepo (AC-010) — после двух репозиториев.
4. HTTP handlers — после use case.
5. Config + main.go DI — финальная интеграция.
6. Migration SQL — вместе с PostgresMaskRepo.

## Порядок реализации

1. **Domain core**: entity, storage interface, errors, uuid — без этого ничего не работает.
2. **CompositeDetector** — небольшое дополнение к существующему пакету.
3. **MaskUseCase** — ядро логики: MaskText + UnmaskText.
4. **UseCase tests** — round-trip, overlap, conflict, UUIDv7.
5. **PostgresMaskRepo** — PG persistence.
6. **ValkeyMaskRepo** — Valkey cache.
7. **CachedMaskRepo** — write-through/read-through.
8. **HTTP handlers** — MaskHandler.
9. **Server + Config + main.go** — DI и регистрация.
10. **Migration SQL** — DDL для PG.

## Риски

- **pgx/valkey-go могут не собраться в изолированной среде:** внешние зависимости.
  *Mitigation:* GONOSUMCHECK/GONOSUMDB для сборки; go.mod уже содержит все зависимости.
- **Overlap filter может пропустить крайний случай:** сложная логика фильтрации интервалов.
  *Mitigation:* unit-тест покрывает базовый случай перекрытия; при необходимости доработать.
- **Valkey недоступен при старте:** caching layer не должен ломать приложение.
  *Mitigation:* nil-client guard в ValkeyMaskRepo; graceful degradation до PG-only.

## Rollout и compatibility

- Специальных rollout-действий не требуется. Новые эндпоинты; существующие не меняются.
- Mask-эндпоинты не имеют флагов — доступны сразу после деплоя.
- Миграция БД: `001_mask_entries.sql` — CREATE TABLE IF NOT EXISTS.

## Проверка

- Automated: `go test ./...` — покрывает все AC.
- Code review: overlap filter, SQL injection (параметризованные запросы), nil-guards.
- Каждый AC подтверждается assertion в тестах.

## Соответствие конституции

- нет конфликтов. DDD + Clean Architecture соблюдены: domain без external зависимостей, infra-адаптеры для PG/Valkey. Nil-safe дизайн соответствует enterprise-требованиям (outbound proxy, изолированные среды).
