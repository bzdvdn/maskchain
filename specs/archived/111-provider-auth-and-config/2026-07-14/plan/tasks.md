# Provider Auth and Config Management — Задачи

## Phase Contract

Inputs: spec, plan, data-model (change).
Outputs: упорядоченные задачи с покрытием AC-001–AC-007.
Stop if: — (артефакты готовы, декомпозиция возможна).

## Surface Map

| Surface | Tasks |
|---|---|
| `src/internal/infra/config/config.go` | T1.1, T1.2, T2.1, T2.2 |
| `src/internal/adapters/provider/openai.go` | T3.1 |
| `src/internal/adapters/provider/anthropic.go` | T3.2 |
| `src/internal/adapters/provider/provider_test.go` | T3.3, T4.2 |
| `src/internal/infra/config/config_test.go` | T4.1, T4.3 |

## Implementation Context

- Цель MVP: config-driven auth per-provider: AuthScheme, AuthHeader, APIKeys ([]string), AdditionalHeaders (map); валидация + маскировка.
- Инварианты/семантика:
  - `api_keys[0]` используется для аутентификации; остальные ключи зарезервированы
  - `auth_scheme` enum: bearer / api-key / basic, default: bearer
  - `auth_header` default: Authorization; пустая строка = Authorization
  - При `auth_scheme: basic` — base64(`:` + key) без user
  - `additional_headers` не перезаписывают auth-заголовок (приоритет у auth)
  - Старый `api_key: "str"` в YAML → fallback в `APIKeys[0]`
- Ошибки/коды:
  - Ошибка валидации при старте, если провайдер с `name` не имеет `api_keys`
  - Ошибка валидации при неизвестном `auth_scheme`
  - Маскировка ключей через `slog.LogValuer` — ключи заменяются на `["****"]` в JSON-логах
- Контракты/протокол: `ports.ProviderRequest.Headers map[string]string` — без изменений
- Границы scope: не меняем `egress.Client`, `domain/routing.Provider`, `ports.ProviderClient`, `ports.ProviderRequest`
- Proof signals: `go test ./src/internal/infra/config/ -run "TestProviderConfig_"` + `go test ./src/internal/adapters/provider/ -run "TestProviderClient_"` — все проходят
- References: DEC-001 (auth в адаптере), DEC-002 (LogValuer), DEC-003 (enum validation), DM (data-model.md)

## Фаза 1: Data model

Цель: добавить новые поля в ProviderConfig, настроить defaults и backward compat для старого `api_key`.

- [x] T1.1 Расширить `ProviderConfig` — заменить `APIKey string` на `APIKeys []string`, добавить `AuthScheme string`, `AuthHeader string`, `AdditionalHeaders map[string]string` с mapstructure/yaml-тегами. Обновить `DefaultConfig` (zero-value defaults: AuthScheme="bearer", AuthHeader="Authorization"). Задача покрывает AC-003 (значения по умолчанию).
  Touches: `src/internal/infra/config/config.go` (ProviderConfig, DefaultConfig)
  Trace: `@sk-task 111-provider-auth-and-config#T1.1`

- [x] T1.2 Реализовать fallback для старого `api_key: "str"` — post-Unmarshal нормализация: если `APIKeys` пуст и старое поле `APIKey` (через viper alias или прямой mapstructure alias) непустое, скопировать в `APIKeys[0]`. Задача обеспечивает backward compat.
  Touches: `src/internal/infra/config/config.go` (LoadConfig или post-Unmarshal hook)
  Trace: `@sk-task 111-provider-auth-and-config#T1.2`

## Фаза 2: Validation и маскировка

Цель: добавить валидацию required APIKeys + enum auth_scheme и маскировку ключей в логах.

- [x] T2.1 Добавить в `validateConfig` (или новый валидатор) проверку: для каждого провайдера с `name != ""` поле `APIKeys` должно содержать хотя бы один элемент; `auth_scheme` должен быть одним из `bearer`, `api-key`, `basic`. Задача покрывает AC-005.
  Touches: `src/internal/infra/config/config.go` (validateConfig, validateRequiredFields)
  Trace: `@sk-task 111-provider-auth-and-config#T2.1`

- [x] T2.2 Реализовать метод `LogValue() slog.Value` на `ProviderConfig` (интерфейс `slog.LogValuer`), который заменяет все элементы `APIKeys` на `"****"`. При логировании `ProviderConfig` через `slog.Any` ключи не должны появляться в открытом виде. Задача покрывает AC-006.
  Touches: `src/internal/infra/config/config.go` (ProviderConfig — новый метод)
  Trace: `@sk-task 111-provider-auth-and-config#T2.2`

## Фаза 3: Provider adapters

Цель: перевести OpenAIClient и AnthropicClient с хардкода заголовков на config-driven auth.

- [x] T3.1 Обновить `OpenAIClient` — в `newOpenAIClient` сохранить `AuthScheme`, `AuthHeader`, `APIKeys`, `AdditionalHeaders` из конфига. В `Call`/`Stream` формировать auth-заголовок согласно схеме (bearer → `Authorization: Bearer <key>`, api-key → `<auth_header>: <key>`, basic → `Authorization: Basic base64(:<key>)`). Мержить `additional_headers` в `Headers` карту (auth-заголовок имеет приоритет). Задача покрывает AC-004, AC-007.
  Touches: `src/internal/adapters/provider/openai.go`
  Trace: `@sk-task 111-provider-auth-and-config#T3.1`

- [x] T3.2 Обновить `AnthropicClient` — аналогично T3.1: читать auth/config из полей конфига, формировать заголовки динамически. Убрать хардкод `x-api-key`. Задача покрывает AC-004, AC-007.
  Touches: `src/internal/adapters/provider/anthropic.go`
  Trace: `@sk-task 111-provider-auth-and-config#T3.2`

- [x] T3.3 Обновить тестовые хелперы `newTestOpenAI`, `newTestAnthropic` и тест `TestProviderClient_Factory` — заменить `APIKey: apiKey` на `APIKeys: []string{apiKey}`. Существующие тесты `TestOpenAIClient_Call`, `TestOpenAIClient_Stream`, `TestAnthropicClient_Call`, `TestAnthropicClient_Stream` должны проходить без изменения логики (адаптеры используют config-driven auth, но тестовые данные остаются теми же).
  Touches: `src/internal/adapters/provider/provider_test.go`
  Trace: `@sk-test 111-provider-auth-and-config#T3.3`

## Фаза 4: Tests

Цель: automated coverage для всех AC.

- [x] T4.1 Добавить тесты конфига: `TestProviderConfig_APIKeys` (AC-001, api_keys из YAML), `TestProviderConfig_EnvAPIKeys` (AC-002, api_keys из env), `TestProviderConfig_AuthDefaults` (AC-003, defaults после Unmarshal).
  Touches: `src/internal/infra/config/config_test.go`
  Trace: `@sk-test 111-provider-auth-and-config#T4.1`

- [x] T4.2 Добавить тесты адаптеров: `TestProviderClient_AuthHeader` (AC-004, кастомный заголовок через auth_scheme/auth_header), `TestProviderClient_AdditionalHeaders` (AC-007, additional_headers в запросе). Использовать `newTestServer` с проверкой заголовков.
  Touches: `src/internal/adapters/provider/provider_test.go`
  Trace: `@sk-test 111-provider-auth-and-config#T4.2`

- [x] T4.3 Добавить тесты валидации и маскировки: `TestProviderConfig_RequireAPIKeys` (AC-005, ошибка при отсутствии api_keys), `TestProviderConfig_RedactAPIKeys` (AC-006, маскировка через slog output). Для маскировки — перехват slog-вывода в bytes.Buffer, проверка отсутствия оригинального ключа.
  Touches: `src/internal/infra/config/config_test.go`
  Trace: `@sk-test 111-provider-auth-and-config#T4.3`

## Покрытие критериев приемки

- AC-001 -> T4.1
- AC-002 -> T4.1
- AC-003 -> T1.1, T4.1
- AC-004 -> T3.1, T3.2, T4.2
- AC-005 -> T2.1, T4.3
- AC-006 -> T2.2, T4.3
- AC-007 -> T3.1, T3.2, T4.2
