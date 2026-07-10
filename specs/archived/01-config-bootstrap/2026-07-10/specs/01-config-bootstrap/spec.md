# Config Bootstrap

## Scope Snapshot

- In scope: cobra + viper конфигурация для gateway: структура Config, загрузка YAML/ENV/flags, валидация required-полей.
- Out of scope: специфичные бизнес-поля (PostgreSQL DSN, Presidio URL, пр.), hot-reload, шифрование секретов.

## Цель

Разработчик получает единую точку конфигурации gateway (Config struct + LoadConfig()), которая объединяет YAML-файл, переменные окружения CONFIG_* и CLI-флаги. Фича считается успешной, когда main.go загружает и логирует конфиг при старте, а отсутствие required-полей приводит к ошибке с человекочитаемым сообщением.

## Основной сценарий

1. Gateway запускается через `bin/gateway --config=config.yaml --log-level=debug`.
2. LoadConfig() читает config.yaml, применяет CONFIG_* env vars (поверх YAML), затем CLI-флаги (поверх всего).
3. Валидация проверяет наличие required-полей; при ошибке — exit с сообщением в stderr.
4. При успехе main() логирует финальную структуру конфига на уровне debug и продолжает запуск.

## User Stories

- P1 Story: как разработчик, я хочу задать конфиг через YAML и переопределить через ENV, чтобы гибко настраивать gateway в разных средах.
- P2 Story: как разработчик, я хочу получать понятную ошибку при пропущенном required-поле, чтобы не гадать, почему gateway не стартует.

## MVP Slice

Config struct с полями `log.level` (default: info), `config.file` (default: config.yaml), минимальная валидация. main.go вызывает LoadConfig и логирует результат. AC-001, AC-002, AC-005.

## First Deployable Outcome

После первого implementation pass можно запустить `go run src/cmd/gateway/main.go --config=test.yaml` и увидеть в stdout/stderr загруженный конфиг или сообщение об ошибке валидации.

## Scope

- Cobra root command с флагами `--config`, `--log-level`
- Config struct в `src/internal/infra/config/config.go`
- LoadConfig() функция, читающая YAML → ENV → flags с viper
- Валидация required-полей на основе struct tags
- main.go вызывает LoadConfig и логирует структуру через structured logger
- `config.yaml` — опциональный файл; его отсутствие не ошибка, если все required поля заданы через ENV/flags

## Контекст

- Go 1.26, cobra + viper — единственные легитимные библиотеки для CLI и конфига (зафиксировано конституцией)
- Проект использует Clean Architecture; infra/config — инфраструктурный слой, не имеющий зависимостей от domain/app/ports
- Config — плоская структура с вложенностью через подструктуры (log, server, database, shield), но на данной фазе наполняются только общие поля (log.level)
- main.go — единственный entrypoint, в нём же происходит инициализация логгера

## Зависимости

- `github.com/spf13/cobra` — CLI framework
- `github.com/spf13/viper` — конфиг (YAML + ENV + flags)
- `go.uber.org/zap` — structured logging
- `none` меж-спековых зависимостей

## Требования

- RQ-001 Система ДОЛЖНА загружать конфигурацию из YAML-файла с поддержкой вложенных полей через viper
- RQ-002 Система ДОЛЖНА применять переменные окружения с префиксом `CONFIG_` как override поверх YAML
- RQ-003 Система ДОЛЖНА применять CLI-флаги (`--config`, `--log-level`) как override поверх ENV
- RQ-004 Система ДОЛЖНА валидировать required-поля и завершаться с ненулевым кодом и сообщением в stderr при отсутствии обязательного поля
- RQ-005 main.go ДОЛЖЕН вызывать LoadConfig() и логировать загруженный конфиг на уровне debug через structured logger

## Вне scope

- Секреты/шифрование полей конфига (будет отдельная фаза или embedded в более широкую)
- Hot-reload / watch изменений config.yaml
- Config subcommands (только root command)
- Разбор бизнес-секций (database, shield) — только заготовки с required-маркерами
- Генерация дефолтного config.yaml
- Покрытие unit-тестами (входит в implementation tasks)

## Критерии приемки

### AC-001 Config struct c YAML-тегами и required-валидацией

- Почему это важно: основа для всех последующих конфигурационных фаз
- **Given** пустой config.yaml отсутствует; установлена ENV `CONFIG_LOG_LEVEL=debug`
- **When** вызывается LoadConfig()
- **Then** возвращается Config{Log: LogConfig{Level: "debug"}}, без ошибок валидации required-полей
- Evidence: вывод лога содержит `"log.level": "debug"`

### AC-002 CLI флаг переопределяет ENV и YAML

- Почему это важно: наивысший приоритет для ad-hoc переопределения
- **Given** config.yaml содержит `log: {level: info}`; ENV `CONFIG_LOG_LEVEL=warn`; флаг `--log-level=error`
- **When** gateway запускается с флагом `--log-level=error`
- **Then** итоговое значение Config.Log.Level = "error"
- Evidence: вывод лога содержит `"log.level": "error"`

### AC-003 Ошибка при отсутствии required-поля

- Почему это важно: раннее обнаружение misconfiguration
- **Given** Config struct имеет поле с тегом `validate:"required"`; ни YAML, ни ENV, ни flags его не задают
- **When** вызывается LoadConfig()
- **Then** возвращается ошибка с указанием имени отсутствующего поля
- Evidence: gateway завершается с ненулевым exit code, stderr содержит "missing required field: <field>"

### AC-004 main.go логирует загруженный конфиг

- Почему это важно: прозрачность старта — разработчик видит, какой конфиг реально применился
- **Given** LoadConfig() завершилась успешно
- **When** main() продолжает выполнение
- **Then** в stdout/stderr выводится debug-сообщение с финальной структурой Config
- Evidence: в логе присутствует сообщение вида `"config loaded"` с полями конфига

### AC-005 Флаг --config задаёт путь к YAML

- Почему это важно: поддержка произвольного расположения конфига
- **Given** существует файл `/tmp/custom.yaml` с валидным конфигом
- **When** gateway запускается с `--config=/tmp/custom.yaml`
- **Then** LoadConfig() читает конфиг из `/tmp/custom.yaml`
- Evidence: загруженные значения соответствуют содержимому `/tmp/custom.yaml`

## Допущения

- viper.BindPFlags используется для связывания cobra flags с viper
- Env vars маппятся через viper.AutomaticEnv с префиксом CONFIG_ и заменой `.` на `_`
- Config.Log.Level — единственное обязательное поле на этой фазе (default: "info")
- Логгер инициализируется до LoadConfig
- Проект использует `go.uber.org/zap`

## Критерии успеха

- SC-001 LoadConfig() выполняется < 100ms на любой конфигурации (YAML, ENV, flags)
- SC-002 Ошибка валидации содержит human-readable field path (напр. `shield.presidio_url`)

## Краевые случаи

- config.yaml не существует — не ошибка, если все required заданы через ENV/flags
- config.yaml с неизвестными полями — viper игнорирует (не ошибка)
- Пустые строки в ENV — трактуются как неустановленные
- Флаг без значения (--log-level без аргумента) — cobra возвращает error
- CONFIG_ c пустым значением после префикса — игнорируется

## Открытые вопросы

- none
