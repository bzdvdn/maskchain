# Provider Auth and Config Management — План

## Phase Contract

Inputs: spec, inspect:pass, minimal repo-контекст (config, provider adapters, egress client, logging).
Outputs: plan.md, data-model.md (no-change).
Stop if: — (спека и inspect пройдены).

## Цель

Обобщить аутентификацию провайдеров из хардкода в конфиг: `ProviderConfig` получает `AuthScheme`, `AuthHeader`, `APIKeys` ([]string), `AdditionalHeaders` (map); адаптеры перестают хардкодить заголовки. Ключи валидируются на старте и маскируются в логах.

## MVP Slice

Config-driven auth в адаптерах + defaults + валидация + маскировка + additional_headers. AC-001–AC-007 закрываются одним инкрементом.

## First Validation Path

```bash
# 1. test: чтение api_keys из YAML
go test ./src/internal/infra/config/ -run TestProviderConfig_APIKeys
# 2. test: чтение api_keys из env
CONFIG_ROUTING_PROVIDERS_0_API_KEYS_0=sk-env-key go test ./src/internal/infra/config/ -run TestProviderConfig_EnvAPIKeys
# 3. test: валидация required
go test ./src/internal/infra/config/ -run TestProviderConfig_RequireAPIKeys
# 4. test: маскировка
go test ./src/internal/infra/config/ -run TestProviderConfig_RedactAPIKeys
# 5. test: кастомный auth заголовок
go test ./src/internal/adapters/provider/ -run TestProviderClient_AuthHeader
# 6. test: additional_headers
go test ./src/internal/adapters/provider/ -run TestProviderClient_AdditionalHeaders
```

## Scope

- `src/internal/infra/config/config.go` — поля `AuthScheme`, `AuthHeader`, `APIKeys` ([]string), `AdditionalHeaders` (map[string]string) в `ProviderConfig`, defaults
- `src/internal/infra/config/config.go` — переименование существующего `APIKey string` → `APIKeys []string` с fallback
- `src/internal/infra/config/config.go` — валидация required APIKeys + enum auth_scheme
- `src/internal/infra/config/config.go` — `LogValue()` для маскировки APIKeys
- `src/internal/adapters/provider/openai.go` — использование `AuthScheme`/`AuthHeader`/`APIKeys`/`AdditionalHeaders` вместо хардкода
- `src/internal/adapters/provider/anthropic.go` — использование config-driven auth + additional_headers
- `src/internal/infra/config/config_test.go` — тесты AC-001–AC-006
- `src/internal/adapters/provider/provider_test.go` — тесты AC-004, AC-007

Не меняется: `egress.Client`, `domain/routing.Provider`, `ports.ProviderRequest`, `ports.ProviderClient`.

## Performance Budget

- none — добавление полей и проверка enum не влияют на hot path; маскировка только при логировании (debug config один раз при старте)

## Implementation Surfaces

| Surface | Почему участвует | Тип |
|---|---|---|
| `infra/config/config.go:ProviderConfig` | целевые поля `AuthScheme`, `AuthHeader`, `APIKeys []string`, `AdditionalHeaders map[string]string` | расширение |
| `infra/config/config.go:DefaultConfig` | defaults для новых полей | существующая |
| `infra/config/config.go:validateConfig` | rule-based validation для `APIKeys` required + enum `auth_scheme` | существующая |
| `infra/config/config.go:ProviderConfig` | `slog.LogValuer` интерфейс для маскировки | расширение |
| `adapters/provider/openai.go:newOpenAIClient` | config-driven auth + additional_headers | существующая |
| `adapters/provider/anthropic.go:newAnthropicClient` | config-driven auth + additional_headers | существующая |
| `infra/config/config_test.go` | тесты AC-001–AC-006 | существующая |
| `adapters/provider/provider_test.go` | тесты AC-004, AC-007 | существующая |

## Bootstrapping Surfaces

- none — вся нужная структура уже существует

## Влияние на архитектуру

- Локальное: `ProviderConfig` расширяется (AuthScheme, APIKeys []string, AdditionalHeaders); существующее поле `APIKey string` переименовывается в `APIKeys []string`. Адаптеры переходят на config-driven auth.
- Интеграции: не затронуты. `ProviderRequest.Headers` уже используется для передачи заголовков.
- Migration/rollout: старые конфиги с `api_key: "str"` — через fallback (viper alias или post-Unmarshal нормализация). Старые конфиги без `auth_scheme`/`auth_header` получают defaults (bearer + Authorization).

## Acceptance Approach

- AC-001: config_test.go — `TestProviderConfig_APIKeys` с YAML (api_keys: [...]), assert APIKeys содержит элементы
- AC-002: config_test.go — `TestProviderConfig_EnvAPIKeys` с `t.Setenv(CONFIG_ROUTING_PROVIDERS_0_API_KEYS_0, ...)`, YAML без api_keys
- AC-003: config_test.go — `TestProviderConfig_AuthDefaults` с YAML без auth_scheme/auth_header
- AC-004: provider_test.go — `TestProviderClient_AuthHeader` с auth_scheme=api-key, auth_header=X-API-Key, проверка Headers
- AC-005: config_test.go — `TestProviderConfig_RequireAPIKeys` с провайдером без api_keys, ожидаем error
- AC-006: config_test.go — `TestProviderConfig_RedactAPIKeys` через slog output, проверяем отсутствие ключа в JSON
- AC-007: provider_test.go — `TestProviderClient_AdditionalHeaders` с additional_headers map, проверка Headers

## Данные и контракты

- Data model: `ProviderConfig` расширяется: `AuthScheme string`, `AuthHeader string`, `APIKeys []string` (вместо `APIKey string`), `AdditionalHeaders map[string]string`. Обратно совместимо через viper alias или fallback для старого `api_key`.
- Контракты: не меняются. `ports.ProviderRequest.Headers` уже принят и используется.
- `data-model.md`: no-change stub → меняем на описание изменений.

## Стратегия реализации

### DEC-001 Auth + AdditionalHeaders в адаптере провайдера, не в egress

Why: каждый провайдер имеет уникальную сигнатуру заголовков (OpenAI — Authorization: Bearer, Anthropic — x-api-key). Egress-клиент — транспортный слой без знания о семантике заголовков. Адаптеры уже хранят ключи и формируют запросы — добавление конфиг-управляемого auth и additional_headers туда же минимально меняет код.

Tradeoff: если в будущем появится 20 провайдеров, каждый будет дублировать логику auth. Решение: сейчас 2 провайдера, дублирование минимально; при росте >5 можно вынести в общий `authMiddleware`.

Affects: `adapters/provider/openai.go`, `adapters/provider/anthropic.go`.

Validation: AC-004, AC-007.

### DEC-002 Маскировка через `slog.LogValuer`

Why: slog поддерживает интерфейс `LogValuer` (`LogValue() slog.Value`) — при логировании структуры через `slog.Any`, `LogValue()` вызывается автоматически. Не требует изменения формата логов или добавления middleware.

Tradeoff: маскировка работает только при логировании через `slog.Any`/`slog.Group`. Если кто-то выведет `cfg.Routing.Providers[0].APIKeys` напрямую — будет незамаскировано. Mitigation: code review + приватное поле.

Affects: `ProviderConfig` (один метод).

Validation: AC-006 проверяет, что JSON-вывод не содержит оригинального ключа.

### DEC-003 Валидация AuthScheme как enum на этапе загрузки конфига

Why: ошибочный auth_scheme (например, "Bearer" с заглавной) приведёт к 401 на первом же запросе. Валидация при старте даёт быструю обратную связь.

Tradeoff: список допустимых значений жёстко задан в коде. При добавлении нового типа (напр., "digest") потребуется изменение кода + тестов. Это осознанно — auth_scheme — это enum с конечным числом вариантов.

Affects: `validateRequiredFields` (новый валидатор для auth_scheme) в `config.go`.

Validation: AC-005 + дополнительный тест на неверный auth_scheme.

## Incremental Delivery

### MVP (Первая ценность)

- Добавить `AuthScheme` (`bearer` | `api-key` | `basic`), `AuthHeader` (default: `Authorization`), `APIKeys []string`, `AdditionalHeaders map[string]string` в `ProviderConfig`
- Переименование `APIKey string` → `APIKeys []string` с fallback для старого `api_key`
- Установить defaults в `DefaultConfig()` (через zero value + логика после Unmarshal)
- Обновить `newOpenAIClient` и `newAnthropicClient` для использования config-driven auth + additional_headers
- Добавить валидацию required для `APIKeys` + enum `auth_scheme` в `validateConfig`
- Реализовать `LogValue()` для маскировки APIKeys
- Тесты AC-001–AC-007

Критерий: `go test ./src/internal/... -run "TestProviderConfig_|TestProviderClient_AuthHeader"` проходит.

### Итеративное расширение

- none — все AC закрываются в MVP

## Порядок реализации

1. `ProviderConfig` — поля AuthScheme, AuthHeader, APIKeys []string, AdditionalHeaders map, теги, defaults (AC-003)
2. Fallback для старого `api_key: "str"` (viper alias или post-Unmarshal)
3. `validateConfig` — required APIKeys + enum auth_scheme (AC-005)
4. `LogValue()` — маскировка APIKeys (AC-006)
5. OpenAIClient / AnthropicClient — config-driven auth + additional_headers (AC-004, AC-007)
6. Тесты AC-001, AC-002 (env/YAML)
7. Финальный прогон всех тестов

Параллельно: нет (все изменения линейны).

## Риски

- Риск 1: Anthropic использует `x-api-key`, а не `Authorization: Bearer` — при auth_scheme=api-key и auth_header по умолчанию (Authorization) Anthropic может не сработать.
  Mitigation: в конфиг Anthropic по умолчанию задать `auth_scheme: api-key` и `auth_header: x-api-key`. Это делается в YAML-примерах и документации. Код адаптера читает эти поля из конфига, а не хардкодит.
- Риск 2: Старые конфиги без `auth_scheme`/`auth_header` должны получить корректные defaults.
  Mitigation: zero value string даёт `""` → код интерпретирует пустую строку как `bearer`/`Authorization`. Тест AC-003 покрывает.
- Риск 3: `basic` auth требует base64-кодирования.
  Mitigation: в MVP достаточно корректного формирования заголовка; base64 кодируется в адаптере.
- Риск 4: Старые конфиги с `api_key: "str"` несовместимы с новым `api_keys: [...]`.
  Mitigation: viper alias или post-Unmarshal fallback: если `APIKeys` пуст и есть старый `APIKey` — перенести значение.

## Rollout и compatibility

- Обратная совместимость: старые конфиги с `api_key: "str"` работают через fallback; старые конфиги без `auth_scheme`/`auth_header` получают defaults.
- Feature flag: не требуется.
- Rollback: откат изменений ProviderConfig и адаптеров.
- Monitoring/auditability: после деплоя проверить логи на появление незамаскированных ключей.

## Проверка

- `go test ./src/internal/infra/config/ -v -run "TestProviderConfig_"`
- `go test ./src/internal/adapters/provider/ -v -run "TestProviderClient_AuthHeader|TestProviderClient_AdditionalHeaders"`
- `go vet ./...`
- Manual: запуск gateway с YAML-конфигом, содержащим провайдера с api_keys и additional_headers, проверка debug-логов на отсутствие ключа.

## Соответствие конституции

- нет конфликтов
