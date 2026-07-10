# Shield Preprocessors: CSV/JSON Masking Pipeline

## Scope Snapshot

- In scope: препроцессоры структурированных данных (CSV, JSON) в Shield Engine pipeline, которые маскируют чувствительные поля по имени/пути до передачи данных детекторам. Препроцессоры — inline-часть профиля, хранятся в JSONB-поле `preprocessors`.
- Out of scope: YAML/XML/Protobuf препроцессоры; standalone-препроцессоры без привязки к профилю; shared-препроцессоры между профилями; препроцессоры для ответов (response path).

## Цель

Security-администраторы смогут настроить в профилях Shield препроцессоры, которые автоматически обнаруживают CSV-блоки и JSON-блоки в AI-запросах и маскируют указанные колонки/поля до того, как данные попадут в PII/secret-детекторы. Это предотвращает ложные срабатывания детекторов на структурированных данных и позволяет обрабатывать CSV/JSON без утечки чувствительных полей. Успех измеряется корректной маскировкой указанных колонок и полей в тестовых сценариях.

## Основной сценарий

1. Администратор создаёт или редактирует профиль Shield и добавляет в поле `preprocessors` один или несколько препроцессоров (CSV и/или JSON) с правилами маскировки.
2. При обработке AI-запроса Shield Engine загружает профиль, извлекает список препроцессоров и запускает их последовательно ДО вызова детекторов.
3. Каждый препроцессор сканирует текст запроса, обнаруживает блоки своего формата (CSV-строки с разделителем-запятой; JSON-объекты/массивы, в т.ч. внутри ```json-фенсов), и заменяет значения в указанных колонках/полях на плейсхолдеры.
4. Замаскированный текст с плейсхолдерами передаётся в детекторы. Replacements map сохраняется для потенциального восстановления (если downstream процессу нужны оригинальные значения).
5. Если блок не найден или указанные поля отсутствуют — препроцессор возвращает текст без изменений.

## User Stories

- P1: Security-администратор настраивает CSV-препроцессор с правилом `{columns: ["email", "phone"], mask: "full"}`. При запросе, содержащем CSV-таблицу с пользователями, значения email и phone заменяются на плейсхолдеры до проверки детекторами.
- P2: Security-администратор настраивает JSON-препроцессор с правилом `{path: "user.email", mask: "full"}` и `{path: "items[*].secret", mask: "full"}`. При запросе, содержащем JSON с вложенными объектами и массивами, указанные поля маскируются.

## MVP Slice

CSVProcessor с mask: `full` + `surname`; JSONProcessor с mask: `full` и поддержкой JSONPath (вложенные объекты, wildcard `[*]`); фабрика `NewPreprocessor`; интеграция в Shield Engine pipeline. MVP закрывает AC-001, AC-002, AC-003, AC-005, AC-007, AC-008.

## First Deployable Outcome

После первого implementation pass можно:
- Создать профиль с CSV-препроцессором (`columns: ["email"]`) и JSON-препроцессором (`path: "user.email"`)
- Передать через Shield Engine текст со смешанными CSV и JSON блоками
- Убедиться, что email-колонка в CSV и user.email в JSON замаскированы, а остальные данные не изменены
- Проверить через unit-тесты и интеграционный тест Shield Engine

## Scope

- `Processor` interface в `src/internal/domain/shield/preprocessor/` с методами `Name() string`, `Process(data string, namespace string) *ProcessResult`
- `ProcessResult` с полями `ModifiedText string` и `Replacements map[string]string`
- Фабрика `NewPreprocessor(def PreprocessorDef) (Processor, error)` — создаёт процессор по типу
- `CSVProcessor` — обнаружение CSV-блоков (строки с запятыми, заголовок + строки), маскировка колонок по имени
- `JSONProcessor` — обнаружение JSON-блоков (включая внутри `` ```json ``), маскировка полей по JSONPath
- `PreprocessorDef` — JSONB-структура в профиле: `{"name": "...", "type": "csv"|"json", "rules": [...]}`
- Mask modes: `full` (плейсхолдер `{{csv.<ns>.<N>}}` / `{{json.<ns>.<N>}}`), `surname` (только первое слово, CSV)
- JSONPath парсер с поддержкой segments, index, wildcard `[*]`
- Поддержка кавычек и экранирования в CSV
- Интеграция в Shield Engine pipeline: препроцессоры запускаются ДО детекторов, последовательно по порядку в списке
- Расширение Profile Repository для JSONB-поля `preprocessors []PreprocessorDef`

## Контекст

- Предыдущие фазы (22-shield-mask-storage) заложили Profile Repository и Shield Engine pipeline; препроцессоры расширяют pipeline на этапе pre-processing.
- Конституция требует DDD + Clean Architecture: Processor — domain interface, реализации — domain-сервисы.
- Профиль уже хранится в PostgreSQL; `preprocessors` — новое JSONB-поле в той же таблице.
- Существующие детекторы (PII, secrets, financial) не должны требовать изменений — препроцессоры модифицируют вход до них.
- Gateway работает в native-only data plane (Go); никаких external runtime-зависимостей.

## Зависимости

- Profile Repository: требуется поддержка JSONB-поля `preprocessors []PreprocessorDef` в таблице профилей.
- Shield Engine pipeline: требуется integration point для запуска препроцессоров до детекторов.
- `none` — внешних сервисов/библиотек не требуется.

## Требования

- RQ-001 Система ДОЛЖНА предоставлять интерфейс `Processor` с методами `Name() string` и `Process(data string, namespace string) *ProcessResult`.
- RQ-002 `ProcessResult` ДОЛЖЕН содержать `ModifiedText string` (текст с плейсхолдерами) и `Replacements map[string]string` (оригинальное значение → плейсхолдер).
- RQ-003 Фабрика `NewPreprocessor(def PreprocessorDef) (Processor, error)` ДОЛЖНА возвращать процессор соответствующего типа по полю `type`.
- RQ-004 Система ДОЛЖНА поддерживать `CSVProcessor`, который обнаруживает CSV-блоки и маскирует указанные колонки по правилам.
- RQ-005 Система ДОЛЖНА поддерживать `JSONProcessor`, который обнаруживает JSON-блоки (включая внутри `` ```json ``) и маскирует поля по JSONPath.
- RQ-006 Система ДОЛЖНА поддерживать mask mode `full` (замена на плейсхолдер `{{<format>.<ns>.<N>}}`) и `surname` (CSV, только первое слово).
- RQ-007 JSONProcessor ДОЛЖЕН поддерживать JSONPath с вложенными объектами, индексами массива и wildcard `[*]`.
- RQ-008 CSVProcessor ДОЛЖЕН корректно обрабатывать CSV с кавычками и escape-последовательностями.
- RQ-009 `PreprocessorDef` ДОЛЖЕН храниться в JSONB-поле `preprocessors []PreprocessorDef` в таблице профилей.
- RQ-010 Shield Engine pipeline ДОЛЖЕН запускать препроцессоры из профиля последовательно ДО вызова детекторов.

## Вне scope

- Препроцессоры для форматов YAML, XML, Protobuf, Form-URL-Encoded.
- Shared-препроцессоры (переиспользование одного preprocessor между несколькими профилями без дублирования).
- Response-path препроцессоры (только request path).
- Mask mode `partial` (частичное скрытие, e.g. `e***@***.com`).
- Восстановление оригинальных значений после детекции (Replacements возвращается, но downstream использование — вне scope).
- Preprocessor type `regex` для произвольных pattern-based преобразований.
- Валидация синтаксиса CSV/JSON (процессор не обязан проверять корректность формата).

## Критерии приемки

### AC-001 CSVProcessor маскирует указанные колонки в режиме full

- Почему это важно: базовый сценарий — чувствительные колонки должны заменяться на плейсхолдеры до детекторов.
- **Given** текст, содержащий CSV-блок с заголовком `name,email,phone` и строками данных, и `PreprocessorDef{Type: "csv", Rules: [{Columns: ["email", "phone"], Mask: "full"}]}`
- **When** `CSVProcessor.Process(data, "req-1")` вызывается
- **Then** значения в колонках `email` и `phone` заменены на плейсхолдеры вида `{{csv.req-1.0}}`, `{{csv.req-1.1}}`, ...; колонка `name` не изменена
- Evidence: `ProcessResult.ModifiedText` содержит плейсхолдеры; `ProcessResult.Replacements` содержит маппинг оригинал→плейсхолдер

### AC-002 CSVProcessor с mask surname

- Почему это важно: администратор может захотеть скрыть фамилию, оставив только имя.
- **Given** CSV-блок с колонкой `name` и значением `John Doe`, правило `{Columns: ["name"], Mask: "surname"}`
- **When** `CSVProcessor.Process(data, "req-1")` вызывается
- **Then** значение `John Doe` заменено на `John` (только первое слово)
- Evidence: `ProcessResult.ModifiedText` содержит `John` в колонке name

### AC-003 JSONProcessor маскирует поле по JSONPath

- Почему это важно: базовый сценарий — вложенные поля JSON должны маскироваться по пути.
- **Given** JSON-блок `{"user": {"email": "test@test.com", "name": "John"}}` и правило `{Path: "user.email", Mask: "full"}`
- **When** `JSONProcessor.Process(data, "req-1")` вызывается
- **Then** значение `"test@test.com"` заменено на `"{{json.req-1.0}}"`; поле `name` не изменено
- Evidence: `ProcessResult.ModifiedText` содержит плейсхолдер

### AC-004 JSONProcessor обрабатывает JSON внутри markdown fences

- Почему это важно: AI-запросы часто содержат JSON внутри ```json блоков.
- **Given** текст с `` ```json\n{"secret": "sensitive"}\n``` `` и правилом `{Path: "secret", Mask: "full"}`
- **When** `JSONProcessor.Process(data, "req-1")` вызывается
- **Then** `"sensitive"` заменён на плейсхолдер внутри fences
- Evidence: `ModifiedText` содержит `` ```json\n{"secret": "{{json.req-1.0}}"}\n``` ``

### AC-005 JSONProcessor с wildcard [*] в массиве

- Почему это важно: массивы объектов — частая структура; нужна массовая маскировка.
- **Given** JSON `{"items": [{"secret": "a"}, {"secret": "b"}]}` и правило `{Path: "items[*].secret", Mask: "full"}`
- **When** `JSONProcessor.Process(data, "req-1")` вызывается
- **Then** все `"secret"` в элементах массива заменены на плейсхолдеры `{{json.req-1.0}}`, `{{json.req-1.1}}`
- Evidence: `ModifiedText` содержит `"items": [{"secret": "{{json.req-1.0}}"}, {"secret": "{{json.req-1.1}}"}]`

### AC-006 CSVProcessor обрабатывает кавычки и экранирование

- Почему это важно: реальные CSV-данные содержат quoted-поля и escape-последовательности.
- **Given** CSV со строкой `"John ""Johnny"" Doe",email@test.com` и правилом `{Columns: ["email"], Mask: "full"}`
- **When** `CSVProcessor.Process(data, "req-1")` вызывается
- **Then** `email@test.com` в незаголовочной строке корректно заменён на плейсхолдер, структура CSV (кавычки) сохранена
- Evidence: `ModifiedText` сохраняет кавычки и экранирование в немаскированных полях

### AC-007 Фабрика NewPreprocessor создаёт процессор по типу

- Почему это важно: корректная диспетчеризация по типу — основа расширяемости.
- **Given** `PreprocessorDef{Name: "csv-mask", Type: "csv", Rules: [...]}`
- **When** `NewPreprocessor(def)` вызывается
- **Then** возвращается `*CSVProcessor`, `error == nil`
- **Given** `PreprocessorDef{Name: "json-mask", Type: "json", Rules: [...]}`
- **When** `NewPreprocessor(def)` вызывается
- **Then** возвращается `*JSONProcessor`, `error == nil`
- **Given** `PreprocessorDef{Name: "unknown", Type: "xml", Rules: [...]}`
- **When** `NewPreprocessor(def)` вызывается
- **Then** возвращается `nil`, `error != nil`

### AC-008 Препроцессоры запускаются до детекторов в Shield Engine

- Почему это важно: pipeline order гарантирует, что детекторы видят уже замаскированные данные.
- **Given** профиль с препроцессорами и текст с CSV-блоком, содержащим email
- **When** Shield Engine обрабатывает запрос
- **Then** препроцессоры применяются до вызова любого детектора; детекторы получают текст с плейсхолдерами
- Evidence: логирование порядка вызова или unit-тест с mock детектора, проверяющий входные данные

## Допущения

- CSV-блок идентифицируется по наличию строк с одинаковым количеством запятых (>=1 строка заголовка + >=1 строка данных). Одиночная строка с запятыми без заголовка не считается CSV-блоком.
- JSON-блок идентифицируется как текст, начинающийся с `{` или `[`, или находящийся внутри `` ```json `` / `` ``` `` фенсов.
- Профиль уже загружен из БД со своим полем `preprocessors` перед вызовом Shield Engine.
- Namespace в `Process()` — идентификатор вызова/сессии — гарантирует уникальность плейсхолдеров в рамках одного запроса.
- Препроцессоры в списке профиля упорядочены и применяются последовательно в заданном порядке.
- Если препроцессор не находит целевых блоков или полей, он возвращает исходный текст без изменений и пустую Replacements map.

## Критерии успеха

- SC-001 Все AC проходят в CI не более чем за 5с (суммарно unit + integration тесты препроцессоров).
- SC-002 Препроцессор не изменяет текст, не содержащий целевых форматов — нулевой overhead на неструктурированных данных (proof: benchmark).
- SC-003 CSV-блок размером 1000 строк × 10 колонок обрабатывается за <100ms.

## Краевые случаи

- CSV без заголовка (только данные) — процессор не может определить имена колонок, блок пропускается.
- JSON без целевых полей по правилу — процессор возвращает текст без изменений.
- Пустой список правил — процессор не применяет маскировку, возвращает исходный текст.
- Правило с неизвестным mask mode — `NewPreprocessor` возвращает ошибку.
- CSV с переменным количеством колонок в строках — обрабатывается как есть, колонки вне указанного индекса игнорируются.
- JSON с экранированными кавычками внутри строк — парсер должен учитывать экранирование.
- Несколько CSV-блоков в одном тексте — каждый блок обрабатывается независимо.
- JSON-блок без ` ```json ` фенса, но начинающийся с `{` — должен корректно обнаруживаться.
- Вложенные JSONPath с несколькими wildcard (`items[*].details[*].secret`) — поддержка не входит в MVP, возвращается ошибка или блок игнорируется.

## Открытые вопросы

- Нужен ли mask mode `partial` (частичная маскировка, e.g. `e***@***.com`)? Отложено до PostMVP.
- Нужен ли preprocessor type `regex` для произвольной pattern-based маскировки? Отложено.
- Как препроцессоры логируются в observability (оригинальные vs замаскированные данные)? Требует уточнения на фазе plan.
- Должна ли быть возможность отключить препроцессор без удаления из профиля (enable/disable flag)? Требует уточнения.
- Как обрабатывать ошибки препроцессора в pipeline: продолжить со следующим препроцессором или остановить обработку? Рекомендация: логировать ошибку и продолжить (fail-open).
