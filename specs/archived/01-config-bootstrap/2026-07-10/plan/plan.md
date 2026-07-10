# Config Bootstrap План

## Phase Contract

- Inputs: spec (AC-001–AC-005), inspect: pass, repo: `src/internal/infra/config/` не существует
- Outputs: plan.md, data-model.md (no-change)
- Stop if: нет

## Цель

Создать infra/config пакет с Config struct, LoadConfig (cobra+viper+YAML/ENV/flags), валидацией required-полей и интеграцией в main.go с zap логгером.

## MVP Slice

Config struct (LogConfig, Config) + LoadConfig + root cobra command + main.go вызов. Покрывает AC-001, AC-002, AC-005. Валидация required (AC-003) и debug-лог (AC-004) — следующий инкремент в том же implementation pass.

## First Validation Path

`go run src/cmd/gateway/main.go --log-level=debug` → видно debug-сообщение с Config. `CONFIG_LOG_LEVEL=error go run src/cmd/gateway/main.go` → уровень debug не виден, error-level сообщения есть.

## Scope

- `src/internal/infra/config/` — новый пакет: Config, LogConfig struct'ы, LoadConfig(), defaults
- `src/cmd/gateway/main.go` — инициализация zap, вызов LoadConfig(), debug-log
- `go.mod` — добавление cobra, viper, zap
- `none` — main.go не содержит бизнес-логики, только bootstrap; domain/app/ports не затрагиваются

## Performance Budget

- none (load-time only config, single invocation)

## Implementation Surfaces

- `src/internal/infra/config/config.go` — **новая**: Config, LogConfig struct'ы, defaults, LoadConfig, validation
- `src/cmd/gateway/main.go` — **существующая**: замена `os.Exit(0)` на init логгера + LoadConfig + debug-log
- `go.mod` / `go.sum` — **существующая**: добавление зависимостей

## Bootstrapping Surfaces

- `src/internal/infra/config/` — директория будет создана

## Влияние на архитектуру

- Локальное: новый infra-пакет без зависимостей от domain/app/ports
- main.go перестаёт быть заглушкой, становится bootstrap entrypoint
- Чистая архитектура не нарушается: infra/config зависит только от stdlib + cobra/viper/zap

## Acceptance Approach

- **AC-001** (ENV override): LoadConfig с CONFIG_LOG_LEVEL=debug → Config.Log.Level == "debug" + evidence в логе. Surface: config.go + main.go
- **AC-002** (CLI override): LoadConfig с флагом `--log-level=error` → Config.Log.Level == "error". Surface: config.go (root cmd flags)
- **AC-003** (required validation): Config struct с marked required field, field не задан → error + stderr message. Surface: config.go
- **AC-004** (debug log): LoadConfig success → debug log с Config. Surface: main.go
- **AC-005** (--config path): LoadConfig с `--config=/tmp/custom.yaml` → читает из указанного пути. Surface: config.go

## Данные и контракты

- Data model не меняется (нет persisted entities). `data-model.md: no-change`.
- API/event contracts не затрагиваются.
- Новый публичный API: `config.LoadConfig(cmd *cobra.Command) (*Config, error)`.

## Стратегия реализации

### DEC-001 Валидация required-полей без go-playground/validator

- **Why**: Spec упоминает `validate:"required"` как observable тег в AC-003. go-playground/validator — популярный выбор, но его `required` на вложенных struct'ах требует ручной регистрации кастомных правил для полной трассировки field path. Альтернатива — lightweight custom validation в LoadConfig: рефлексивно проходим по struct, проверяем наличие тега `validate:"required"` и non-zero значение через `viper.IsSet`. Это не добавляет внешней зависимости, даёт полный field path и сохраняет observable contract (`validate:"required"` тег на struct field).
- **Tradeoff**: Если в будущем потребуются сложные validation rules (min, max, oneof, uuid) — придётся либо расширять кастомную, либо мигрировать на go-playground/validator. Риск низкий: бизнес-валидация живёт в domain слое, а config validation — только required + типы.
- **Affects**: `src/internal/infra/config/config.go`
- **Validation**: AC-003 проходим: поле с `validate:"required"` без значения → ошибка с field path в stderr

### DEC-002 Структура Config — плоская с вложенностью через указатели на подструктуры

- **Why**: viper хорошо маппит YAML-ключи вида `log.level` на вложенные struct поля. Используем `mapstructure` (встроен в viper) для разметки struct tags. Подструктуры — указатели, чтобы отличать "не задано" от "zero value".
- **Tradeoff**: Указатели требуют nil-проверок при доступе. Для bootstrap фазы это несущественно, так как все поля будут заполнены через viper.
- **Affects**: `src/internal/infra/config/config.go`
- **Validation**: AC-001, AC-002 проходят с правильным разрешением приоритетов YAML < ENV < flags

### DEC-003 Zap logger — пакетная инициализация в main.go

- **Why**: zap требует конфиг (уровень логирования) при создании. Логгер создаётся в main.go ПОСЛЕ LoadConfig (чтобы знать уровень) и передаётся дальше (на данном этапе — только для debug-лога конфига). Используем `zap.NewProductionConfig()` с кастомным уровнем.
- **Tradeoff**: Если позже понадобится логгировать процесс загрузки конфига, придётся либо использовать fallback-логгер (stderr), либо мигрировать на двухфазную инициализацию. Для bootstrap фазы достаточно stderr для ошибок валидации.
- **Affects**: `src/cmd/gateway/main.go`
- **Validation**: AC-004 — debug сообщение с Config в логе

## Incremental Delivery

### MVP (Первая ценность)

1. Создать `src/internal/infra/config/config.go`: Config + LogConfig struct'ы, defaults, LoadConfig (cobra root flags + viper), required-валидация
2. `main.go`: инициализация zap, LoadConfig, debug-log
3. `go mod tidy` + проверка компиляции
4. Ручная проверка: `go run src/cmd/gateway/main.go --log-level=debug`

Covers: AC-001, AC-002, AC-004, AC-005 (все, кроме AC-003, который является внутренней частью того же implementation pass)

## Порядок реализации

1. `go get github.com/spf13/cobra github.com/spf13/viper go.uber.org/zap`
2. `src/internal/infra/config/config.go` — struct'ы + defaults + LoadConfig + validation
3. `src/cmd/gateway/main.go` — zap init + LoadConfig + debug log
4. `go mod tidy` + `make lint` + `make build`

Параллелизация: нет — всё завязано на Config struct.

## Риски

- **Риск 1**: viper не поддерживает nested struct key mapping из ENV с точками.
  Mitigation: viper поддерживает `AutomaticEnv()` с префиксом и заменой `.` на `_`; mapstructure decoder tag настраивается вручную для корректного маппинга. Покрывается AC-001.

- **Риск 2**: Забыть обработать случай отсутствия config.yaml.
  Mitigation: viper не возвращает ошибку на отсутствие файла, если не вызван `viper.ReadInConfig()`. Используем `viper.SafeAddConfigPath` + `viper.ReadInConfig()` и игнорируем `viper.ConfigFileNotFoundError`.

## Rollout и compatibility

- Специальных rollout-действий не требуется. Config — новый пакет, не затрагивает существующий код.

## Проверка

- **Automated**: unit-тесты для LoadConfig (каждый AC), go vet, go build, lint
- **Manual**: `go run src/cmd/gateway/main.go --log-level=debug` — видим debug-лог; `go run src/cmd/gateway/main.go` — info-лог без debug
- **AC coverage**: AC-001–AC-005

## Соответствие конституции

- нет конфликтов. cobra/viper/zap в рамках конституции (Go + cobra/viper structured logging).
