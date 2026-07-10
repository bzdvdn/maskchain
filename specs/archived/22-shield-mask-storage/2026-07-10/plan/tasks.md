# Mask Storage — Задачи

## Phase Contract

Inputs: plan.md, data-model.md, spec.md.
Outputs: исполнимые задачи с Touches и покрытием AC.
Stop if: покрытие AC не удаётся сопоставить задачам — все 12 AC покрыты.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/domain/shield/mask/entity.go` | T1.1 |
| `src/internal/domain/shield/mask/storage.go` | T1.1 |
| `src/internal/domain/shield/mask/errors.go` | T1.1 |
| `src/internal/domain/shield/mask/uuid.go` | T1.1 |
| `src/internal/domain/shield/mask/usecase.go` | T2.1 |
| `src/internal/domain/shield/mask/usecase_test.go` | T2.2 |
| `src/internal/domain/shield/detector/composite.go` | T1.2 |
| `src/internal/adapters/repository/mask/postgres.go` | T3.1 |
| `src/internal/adapters/repository/mask/valkey.go` | T3.2 |
| `src/internal/adapters/repository/mask/cached.go` | T3.3 |
| `src/internal/api/mask_handler.go` | T4.1 |
| `src/internal/api/server.go` | T4.2 |
| `src/internal/infra/config/config.go` | T5.1 |
| `src/cmd/gateway/main.go` | T5.2 |
| `deployments/migrations/001_mask_entries.sql` | T3.1 |

## Implementation Context

- **Цель MVP:** MaskUseCase с MaskText/UnmaskText + in-memory storage + CompositeDetector + тесты — покрывает AC-001–AC-007, AC-011.
- **Инварианты/семантика:**
  - `MaskEntry.Replacements` — `map[string]string` placeholder→original. Не хранит original_text.
  - `MaskUseCase.MaskText` сканирует все зарегистрированные детекторы → сортирует по длине DESC → фильтрует пересечения (longer wins) → заменяет right-to-left.
  - Плейсхолдеры: `{{<mask_id>.<N>}}`.
  - `Save` с существующим mask_id → ErrMaskIDConflict.
  - nil pool/client → no-op/ErrMaskNotFound.
- **Ошибки/коды:** domain errors: `ErrMaskNotFound`, `ErrMaskIDConflict`. Оборачиваются `fmt.Errorf("save mask entry: %w", err)`.
- **Границы scope:** не трогаем entity.Detector, entity.Incident, ScanPipeline, профили, audit.
- **Proof signals:** `go test ./...` зелёный; round-trip тест; mask → unmask = original.

## Фаза 1: Domain core + CompositeDetector

Цель: MaskEntry, MaskStorage, errors, UUIDv7, CompositeDetector. Фундамент для use case.

- [x] T1.1 Создать пакет mask с MaskEntry, MaskStorage, errors, UUIDv7.
  - `MaskEntry`: mask_id string, profile_id *string, Replacements map[string]string, CreatedAt time.Time
  - `MaskStorage`: Save(ctx, *MaskEntry) error, Get(ctx, maskID) (*MaskEntry, error), Delete(ctx, maskID) error
  - `ErrMaskNotFound`, `ErrMaskIDConflict`
  - `NewUUIDv7()` — RFC 9562, crypto/rand, 36 chars, version 7
  - Touches: `src/internal/domain/shield/mask/entity.go`, `storage.go`, `errors.go`, `uuid.go`
  - AC: AC-001, AC-007
  - DEC: DEC-001, DEC-006

- [x] T1.2 Реализовать CompositeDetector.
  - Оборачивает несколько `Detector` в один; Scan() мерджит результаты
  - Touches: `src/internal/domain/shield/detector/composite.go`
  - AC: AC-011
  - DEC: DEC-005

## Фаза 2: MaskUseCase + тесты (MVP)

Цель: ядро маскинга — MaskText сканирует/заменяет/сохраняет, UnmaskText загружает/восстанавливает.

- [x] T2.1 Реализовать MaskUseCase.
  - `MaskText(ctx, text, maskID)` — сканирование через registry → сортировка (длина DESC) → фильтр пересечений (longer wins) → замена right-to-left → save → masked text
  - `UnmaskText(ctx, maskedText, maskIDs)` — load каждого mask_id → merge replacements → strings.ReplaceAll → restored text
  - Touches: `src/internal/domain/shield/mask/usecase.go`
  - AC: AC-002, AC-003, AC-004, AC-005
  - DEC: DEC-001, DEC-002

- [x] T2.2 Добавить unit-тесты MaskUseCase.
  - Empty text → empty replacements (AC-002)
  - Нет детекторов → текст не меняется
  - Одиночная замена → "test@example.com" → "{{abc.1}}" (AC-002)
  - UnmaskText → восстановление (AC-003)
  - Round-trip mask→unmask = original (AC-004)
  - Overlap filter — длинное побеждает короткое (AC-005)
  - Conflict на дубликат mask_id (AC-006)
  - UUIDv7 — длина, версия, variant (AC-007)
  - Множественные mask_ids в unmask
  - Touches: `src/internal/domain/shield/mask/usecase_test.go`
  - AC: AC-002, AC-003, AC-004, AC-005, AC-006, AC-007

## Фаза 3: Репозитории

Цель: PG persistence, Valkey cache, композит с write-through/read-through.

- [x] T3.1 Реализовать PostgresMaskRepo + миграция.
  - INSERT with ON CONFLICT DO NOTHING → RowsAffected → ErrMaskIDConflict
  - SELECT по mask_id → unmarshal JSONB → MaskEntry
  - DELETE по mask_id
  - nil pool → no-op/ErrMaskNotFound
  - DDL: `CREATE TABLE mask_entries (mask_id TEXT PK, profile_id TEXT, replacements JSONB, created_at TIMESTAMPTZ)`
  - Touches: `src/internal/adapters/repository/mask/postgres.go`, `deployments/migrations/001_mask_entries.sql`
  - AC: AC-008, AC-012
  - DEC: DEC-004

- [x] T3.2 Реализовать ValkeyMaskRepo.
  - Save → JSON marshal → SET mask:<id> with EX (TTL)
  - Get → GET mask:<id> → JSON unmarshal; valkey.Nil → ErrMaskNotFound
  - Delete → DEL mask:<id>
  - nil client → no-op/ErrMaskNotFound
  - Touches: `src/internal/adapters/repository/mask/valkey.go`
  - AC: AC-009, AC-012
  - DEC: DEC-004

- [x] T3.3 Реализовать CachedMaskRepo.
  - Save: primary.Save (PG) → secondary.Save (Valkey, best-effort)
  - Get: secondary.Get (Valkey) → hit? return; miss → primary.Get (PG) → secondary.Save (refresh) → return
  - Delete: primary.Delete → secondary.Delete
  - Touches: `src/internal/adapters/repository/mask/cached.go`
  - AC: AC-010
  - DEC: DEC-003

## Фаза 4: HTTP handlers

Цель: `/mask` и `/unmask` эндпоинты с корректной обработкой ошибок.

- [x] T4.1 Реализовать MaskHandler.
  - `HandleMask`: читает mask_id из query (или генерирует UUIDv7), тело запроса → MaskUseCase.MaskText → 200 с masked text + X-Mask-ID; 409 на conflict
  - `HandleUnmask`: читает mask_ids из query (comma-separated), тело запроса → MaskUseCase.UnmaskText → 200 с restored text; 404 на not found
  - Touches: `src/internal/api/mask_handler.go`
  - AC: AC-002, AC-003, AC-006

- [x] T4.2 Добавить RegisterMaskHandler в Server.
  - Метод `RegisterMaskHandler(h *MaskHandler)` на Server
  - Регистрирует POST /api/v1/shield/mask и POST /api/v1/shield/unmask
  - Touches: `src/internal/api/server.go`
  - AC: AC-002, AC-003

## Фаза 5: Интеграция

Цель: конфигурация, DI, сборка.

- [x] T5.1 Расширить конфиг: Database, Valkey, Mask секции.
  - `DatabaseConfig` с DSN
  - `ValkeyConfig` с Addr, Password, TTLSec
  - `MaskConfig` с CacheTTLSec (default 3600)
  - Defaults для всех полей
  - Touches: `src/internal/infra/config/config.go`

- [x] T5.2 Обновить main.go: DI всех компонентов.
  - Инициализация PG pool (pgxpool.New) с nil-обработкой
  - Инициализация Valkey client с nil-обработкой
  - Создание DetectorRegistry с CompositeDetector (PII + secrets + financial)
  - Создание PostgresMaskRepo, ValkeyMaskRepo, CachedMaskRepo
  - Создание MaskUseCase → MaskHandler → регистрация в Server
  - Touches: `src/cmd/gateway/main.go`
  - AC: все

## Покрытие критериев приемки

| AC | Задачи | DEC |
|----|--------|-----|
| AC-001 | T1.1 | DEC-001 |
| AC-002 | T2.1, T2.2, T4.1, T4.2 | DEC-001, DEC-002 |
| AC-003 | T2.1, T2.2, T4.1, T4.2 | DEC-001 |
| AC-004 | T2.1, T2.2 | DEC-001, DEC-002 |
| AC-005 | T2.1, T2.2 | DEC-002 |
| AC-006 | T2.2, T3.1, T4.1 | DEC-001 |
| AC-007 | T1.1, T2.2 | DEC-006 |
| AC-008 | T3.1 | — |
| AC-009 | T3.2 | — |
| AC-010 | T3.3 | DEC-003 |
| AC-011 | T1.2 | DEC-005 |
| AC-012 | T3.1, T3.2 | DEC-004 |

## Заметки

- T1.1 и T1.2 независимы — можно параллелить.
- T2.1 → T2.2 обязательный порядок.
- T3.1, T3.2, T3.3 последовательно (CachedMaskRepo зависит от Postgres + Valkey).
- T4.1 и T4.2 зависят от T2.1.
- T5.1 и T5.2 — финальная интеграция, после всех остальных.
- Все задачи отмечены [x], т.к. реализация уже выполнена.
- Trace-маркеры: `@sk-task 22-shield-mask-storage#T<N>.<M>` над каждым owning типом/функцией; `@sk-test 22-shield-mask-storage#T<N>.<M>` над каждым тестом.
