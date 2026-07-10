# Shield Preprocessors: CSV/JSON Masking Pipeline — Задачи

## Phase Contract

Inputs: plan, data model, spec, repo surfaces (Shield Engine pipeline, Profile entity, Profile Repository).
Outputs: упорядоченные исполнимые задачи с покрытием AC.
Stop if: AC или DEC не покрываются исполнимыми задачами.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/domain/shield/preprocessor/processor.go` | T1.1 |
| `src/internal/domain/shield/preprocessor/factory.go` | T1.1, T2.5 |
| `src/internal/domain/shield/preprocessor/csv.go` | T2.1 |
| `src/internal/domain/shield/preprocessor/json.go` | T2.2 |
| `src/internal/domain/shield/preprocessor/jsonpath.go` | T2.2 |
| `src/internal/domain/shield/preprocessor/csv_test.go` | T2.3 |
| `src/internal/domain/shield/preprocessor/json_test.go` | T2.4 |
| `src/internal/domain/shield/preprocessor/factory_test.go` | T2.5 |
| `src/internal/domain/shield/entity/profile.go` | T1.2 |
| `src/internal/adapters/repository/profile/postgres.go` | T3.1 |
| `src/internal/api/mask_handler.go` | T3.2 |
| `src/internal/api/mask_handler_test.go` | T4.1 |

## Implementation Context

- **Цель MVP:** CSVProcessor (full + surname), JSONProcessor (full + JSONPath с `[*]`), фабрика, интеграция в MaskHandler до детекторов.
- **Инварианты/семантика:**
  - Processor.Process возвращает новую строку, не мутирует исходную
  - CSV-блок: >= 2 строк с одинаковым числом запятых (заголовок + данные)
  - JSONPath split по `.` без escaping; `[*]` — единственный wildcard
  - Fail-open: ошибка препроцессора логируется, pipeline продолжается с оригинальным текстом
  - Пустой список препроцессоров = no-op
- **Ошибки/коды:** `NewPreprocessor` — `ErrUnknownPreprocessorType` при неизвестном `type`; Process возвращает ошибку через `*ProcessResult` (не блокирует pipeline)
- **Контракты/протокол:**
  - `PreprocessorDef` сериализуется в JSONB в поле `preprocessors` профиля
  - `ProcessResult.ModifiedText` передаётся в детекторы вместо оригинального body
  - Плейсхолдеры: `{{csv.<ns>.<N>}}`, `{{json.<ns>.<N>}}`
- **Границы scope:** не делаем partial mask, regex processors, shared preprocessors, YAML/XML
- **Proof signals:** `go test ./src/internal/domain/shield/preprocessor/...` green; integration AC-008 с mock (детектор получает текст с плейсхолдерами)
- **References:** DEC-001 (inline JSONB), DEC-002 (встраивание в handler), DEC-003 (JSON map walker), DEC-004 (fail-open); DM-001–DM-004

## Фаза 1: Основа (Domain types + data model)

Цель: подготовить типы данных и расширение Profile entity — база для всех процессоров.

- [x] T1.1 Создать пакет `preprocessor/` с типами: `Processor` interface (`Name()`, `Process()`), `ProcessResult`, `PreprocessorDef`, `Rule`, `MaskMode`, и фабрику `NewPreprocessor()`. Touches: `src/internal/domain/shield/preprocessor/processor.go`, `src/internal/domain/shield/preprocessor/factory.go`

- [x] T1.2 Расширить `Profile` entity: добавить поле `preprocessors []PreprocessorDef`, опцию `WithPreprocessors()`, геттер `Preprocessors()`. Touches: `src/internal/domain/shield/entity/profile.go`

## Фаза 2: Процессоры (MVP)

Цель: реализовать CSVProcessor и JSONProcessor с ключевыми mask modes.

- [x] T2.1 Реализовать `CSVProcessor`: обнаружение CSV-блоков (>= 2 строк с одинаковым числом запятых), маскировка колонок по имени в режимах `full` (плейсхолдер `{{csv.<ns>.<N>}}`) и `surname` (только первое слово), поддержка кавычек и экранирования. Touches: `src/internal/domain/shield/preprocessor/csv.go`

- [x] T2.2 Реализовать `JSONProcessor` + JSONPath walker: обнаружение JSON-блоков (в т.ч. внутри ```json фенсов), маскировка полей по JSONPath (вложенные объекты, индексы массива, wildcard `[*]`), unmarshal в `any` → mutate → re-marshal (DEC-003). Touches: `src/internal/domain/shield/preprocessor/json.go`, `src/internal/domain/shield/preprocessor/jsonpath.go`

- [x] T2.3 Unit-тесты `CSVProcessor`: AC-001 (full mask нескольких колонок), AC-002 (surname), AC-006 (quoting/escaping). Touches: `src/internal/domain/shield/preprocessor/csv_test.go`

- [x] T2.4 Unit-тесты `JSONProcessor`: AC-003 (вложенный JSONPath), AC-004 (markdown fences), AC-005 (wildcard `[*]`). Touches: `src/internal/domain/shield/preprocessor/json_test.go`

- [x] T2.5 Unit-тесты фабрики: AC-007 (csv → `*CSVProcessor`, json → `*JSONProcessor`, unknown → error). Touches: `src/internal/domain/shield/preprocessor/factory_test.go`

## Фаза 3: Интеграция

Цель: связать препроцессоры с Profile Repository и MaskHandler.

- [x] T3.1 Реализовать JSONB marshaling/unmarshaling поля `preprocessors` в `PostgresProfileRepo` (DM-001). Поле nullable — `nil`/NULL = пустой список. Touches: `src/internal/adapters/repository/profile/postgres.go`

- [x] T3.2 Интегрировать препроцессоры в `MaskHandler.HandleMask`: после чтения body, до цикла детекторов (строка 41), запустить препроцессоры из профиля (загрузить профиль, обойти список, применить `Process()`, передать `ModifiedText` в `d.Scan()`). Fail-open при ошибке одного препроцессора (DEC-004). Touches: `src/internal/api/mask_handler.go`

## Фаза 4: Проверка

Цель: доказать полную работоспособность фичи.

- [x] T4.1 Интеграционный тест: mock repository с профилем (препроцессоры), mock detector, проверить что `d.Scan()` получает текст с плейсхолдерами (AC-008). Touches: `src/internal/api/mask_handler_test.go`

- [x] T4.2 Финальная проверка: `gofmt`, `go vet`, `go test ./...`, review trace-маркеров (`@sk-task` / `@sk-test`), удалить `TODO`. Touches: `src/internal/domain/shield/preprocessor/`, `src/internal/domain/shield/entity/profile.go`, `src/internal/api/mask_handler.go`, `src/internal/adapters/repository/profile/postgres.go`

## Покрытие критериев приемки

- AC-001 -> T2.1, T2.3
- AC-002 -> T2.1, T2.3
- AC-003 -> T2.2, T2.4
- AC-004 -> T2.2, T2.4
- AC-005 -> T2.2, T2.4
- AC-006 -> T2.1, T2.3
- AC-007 -> T1.1, T2.5
- AC-008 -> T3.2, T4.1

## Заметки

- T1.1 и T1.2 можно выполнять параллельно
- T2.1, T2.2 параллельны друг другу
- T2.3, T2.4, T2.5 — после соответствующих реализаций
- T3.2 зависит от T1.1, T1.2, T2.1, T2.2 (нужны типы + процессоры + профиль)
- T4.1 зависит от T3.2
- Trace-маркеры `@sk-task <slug>#<TASK_ID>: <short> (<AC_ID>)` — на объявления функций/методов/типов, не на package/import/file-header
