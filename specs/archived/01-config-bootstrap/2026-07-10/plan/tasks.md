# Config Bootstrap Задачи

## Phase Contract

Inputs: plan (DEC-001–DEC-003), spec (AC-001–AC-005)
Outputs: упорядоченные задачи с покрытием всех AC
Stop if: нет

## Surface Map

| Surface | Tasks |
|---------|-------|
| `go.mod` / `go.sum` | T1.1 |
| `src/internal/infra/config/config.go` | T1.2, T2.1, T3.1 |
| `src/cmd/gateway/main.go` | T2.2 |
| `src/internal/infra/config/config_test.go` | T3.1 |

## Implementation Context

- **Цель MVP**: Config struct + LoadConfig (cobra+viper+YAML/ENV/flags) + main.go с zap. AC-001–AC-005.
- **Инварианты/семантика**:
  - Viper priority: YAML < ENV (CONFIG_*) < CLI flags
  - `validate:"required"` struct tag → custom reflection check, не go-playground/validator
  - Config struct с mapstructure тегами; подструктуры — указатели
  - config.yaml опционален; отсутствие не ошибка если required заданы через ENV/flags
- **Ошибки/коды**:
  - missing required field → stderr message + `os.Exit(1)`
  - неизвестные YAML-ключи → игнорируются viper (не ошибка)
  - флаг без значения → cobra error
- **Контракты/протокол**:
  - `LoadConfig(cmd *cobra.Command) (*Config, error)` — публичный API пакета config
  - main.go: init zap → LoadConfig → debug-log → (пока exit 0)
- **Границы scope**:
  - не пишем бизнес-секции (database, shield) — только LogConfig заготовка
  - не делаем hot-reload, шифрование, генерацию config.yaml
- **Proof signals**:
  - `go run src/cmd/gateway/main.go --log-level=debug` → видно debug-сообщение с Config
  - `CONFIG_LOG_LEVEL=error go run src/cmd/gateway/main.go` → без debug-сообщений
- **References**: DEC-001 (validation), DEC-002 (struct design), DEC-003 (zap init)

## Фаза 1: Bootstrapping

Цель: подготовить зависимости, директории и каркас Config struct.

- [x] T1.1 Добавить зависимости cobra, viper, zap в go.mod
  Touches: `go.mod`
  `go get github.com/spf13/cobra@latest github.com/spf13/viper@latest go.uber.org/zap@latest`

- [x] T1.2 Создать Config struct с LogConfig, тегами mapstructure/yaml/validate, дефолтными константами
  Touches: `src/internal/infra/config/config.go`
  - Config{Log: *LogConfig}, LogConfig{Level string}
  - defaultLogLevel = "info"
  - mapstructure/yaml теги для viper-маппинга
  - `validate:"required"` на LogConfig.Level (единственное required на этой фазе)

## Фаза 2: MVP реализация

Цель: LoadConfig + main.go — gateway грузит и логирует конфиг.

- [x] T2.1 Реализовать LoadConfig — cobra root command с `--config` и `--log-level`, viper YAML/ENV/flags binding, required-валидация, return (*Config, error)
  Touches: `src/internal/infra/config/config.go`
  - NewRootCmd() → *cobra.Command c flags: `--config` (string, default "config.yaml"), `--log-level` (string, default "")
  - LoadConfig(cmd) читает config.yaml через viper (если файл есть), биндит ENV (CONFIG_), биндит PFlags
  - custom validateConfig(cfg) — reflection по struct: поле с `validate:"required"` и !viper.IsSet → error
  - покрывает AC-001, AC-002, AC-003, AC-005

- [x] T2.2 Интегрировать LoadConfig в main.go — zap init, LoadConfig, debug-log config, exit(0)
  Touches: `src/cmd/gateway/main.go`
  - zap.NewProductionConfig → set Level from config → logger
  - logger.Debug("config loaded", zap.Any("config", cfg))
  - `// @sk-task 01-config-bootstrap#T2.2` над функцией main

## Фаза 3: Проверка

Цель: automated-тесты + manual validation.

- [x] T3.1 Написать unit-тесты для LoadConfig: AC-001 (ENV), AC-002 (CLI flag), AC-003 (required validation), AC-005 (--config path)
  Touches: `src/internal/infra/config/config_test.go`
  - TestLoadConfig_EnvOverride (AC-001)
  - TestLoadConfig_CLIOverride (AC-002)
  - TestLoadConfig_RequiredValidation (AC-003)
  - TestLoadConfig_CustomConfigPath (AC-005)
  - маркер `// @sk-test 01-config-bootstrap#T3.1` над каждой тестовой функцией
  - go test ./src/internal/infra/config/ -v

## Покрытие критериев приемки

- AC-001 -> T2.1, T3.1
- AC-002 -> T2.1, T3.1
- AC-003 -> T2.1, T3.1
- AC-004 -> T2.2
- AC-005 -> T2.1, T3.1

## Заметки

- Фаза 3 пропускает T4.x из шаблона — unit-тесты полностью покрывают проверку; отдельная verify-фаза не нужна
- AC-004 (debug log) покрывается T2.2 + manual check; automated проверка формата лога — out of scope для данной фазы
