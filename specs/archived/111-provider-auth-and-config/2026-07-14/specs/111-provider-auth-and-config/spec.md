# Provider Auth and Config Management

## Scope Snapshot

- In scope: добавление в `ProviderConfig` полей `api_keys`, `auth_scheme`, `auth_header`, `additional_headers`, поддержка чтения ключей из env/Vault, валидация обязательности ключа для сконфигурированных провайдеров, маскировка sensitive-полей в логах.
- Out of scope: ротация ключей, интеграция с внешними Vault-системами (HashiCorp Vault, AWS Secrets Manager), управление ключами через UI.

## Цель

Оператор получает возможность конфигурировать аутентификацию per-provider через YAML и/или env-переменные: выбирать схему (bearer, api-key, basic), задавать кастомный заголовок, передавать дополнительные произвольные заголовки. Система валидирует наличие хотя бы одного ключа для сконфигурированных провайдеров и не выводит ключи в debug-выводе конфига. Успех фичи измеряется отсутствием ключей в логах и корректной передачей заголовков в исходящих запросах.

## Основной сценарий

1. Оператор указывает в YAML / env провайдера с `api_keys` (или опускает, используя env).
2. При старте система читает ключи из YAML или из env `CONFIG_ROUTING_PROVIDERS_<N>_API_KEYS_<M>`.
3. Система валидирует: если провайдер указан в конфиге (есть `name`), `api_keys` обязателен (хотя бы один ключ).
4. При исходящем запросе система добавляет заголовок аутентификации согласно `auth_scheme`, `auth_header` и первому ключу из `api_keys`.
5. Дополнительные заголовки из `additional_headers` также добавляются к запросу.
6. В debug-выводе конфига значения `api_keys` заменяются на `****`.
7. При отсутствии ключей для сконфигурированного провайдера — ошибка валидации при старте.

## User Stories

- P1 Story: оператор конфигурирует провайдера с bearer-токеном через YAML, система добавляет `Authorization: Bearer <key>` и дополнительные заголовки из `additional_headers` к исходящим запросам.
- P2 Story: оператор задаёт ключ только через env без YAML, система читает и использует его.

## MVP Slice

Наименьший срез: `ProviderConfig` получает `AuthScheme`, `AuthHeader`, `APIKeys` ([]string), `AdditionalHeaders` (map), валидация required для `APIKeys`, маскировка в debug. Закрывает AC-001–AC-007.

## First Deployable Outcome

После первого pass можно запустить gateway с конфигом, содержащим `routing.providers[0].api_keys`, `auth_scheme`, `auth_header`, `additional_headers`, проверить что ключи не выводятся в debug-логах, а в исходящем запросе появляются корректные заголовки.

## Scope

- Добавление полей `AuthScheme`, `AuthHeader`, `APIKeys` ([]string), `AdditionalHeaders` (map[string]string) в `ProviderConfig`.
- Переименование существующего `APIKey` (string) → `APIKeys` ([]string) с обратной совместимостью.
- Валидация `APIKeys` как required (хотя бы один элемент) для провайдеров с `name != ""`.
- Валидация `auth_scheme` как enum: `bearer`, `api-key`, `basic`.
- Маскировка всех значений `APIKeys` при выводе конфига в debug-логах.
- Чтение `api_keys` из YAML и из env `CONFIG_ROUTING_PROVIDERS_<N>_API_KEYS_<M>`.
- Проброс `additional_headers` + auth-заголовка в `ProviderRequest.Headers` через адаптер провайдера.

## Контекст

- viper поддерживает env `CONFIG_ROUTING_PROVIDERS_<N>_API_KEYS_<M>` через `AutomaticEnv` и pre-populate loop.
- В `ProviderConfig` есть поле `APIKey string` (добавлено в #110-provider-adapters) — будет переименовано в `APIKeys []string`. Для обратной совместимости старый `api_key: "str"` в YAML должен маппиться через viper или fallback-логику.
- Домен `Provider` (`domain/routing/health_status.go`) не имеет auth-полей — auth инкапсулирован в конфиг/адаптер.
- Существующая валидация `validateConfig` проверяет `validate:"required"` через `validateRequiredFields`.
- `ProviderClient` в `ports/provider.go` принимает `ProviderRequest.Headers` — auth-заголовок может передаваться через них.

## Зависимости

- #110-provider-adapters: `ProviderConfig.APIKey` уже существует.
- #70-routing-engine: `ProviderConfig`, `Provider`, `NewProviderRegistry`.

## Требования

- RQ-001 Система ДОЛЖНА читать `api_keys` (массив строк) для провайдера из YAML поля `routing.providers[].api_keys` или из env `CONFIG_ROUTING_PROVIDERS_<N>_API_KEYS_<M>`.
- RQ-002 Система ДОЛЖНА поддерживать поле `auth_scheme` со значениями `bearer`, `api-key`, `basic`; значение по умолчанию — `bearer`.
- RQ-003 Система ДОЛЖНА поддерживать поле `auth_header` с произвольным именем заголовка; значение по умолчанию — `Authorization`.
- RQ-004 Система ДОЛЖНА валидировать: если провайдер имеет `name`, поле `api_keys` обязательно (хотя бы один ключ).
- RQ-005 Система ДОЛЖНА маскировать все значения `api_keys` (заменять на `****`) при выводе конфига в debug-логи.
- RQ-006 Система ДОЛЖНА добавлять корректный заголовок аутентификации к запросу провайдера согласно `auth_scheme`/`auth_header`/первому ключу из `api_keys`.
- RQ-007 Система ДОЛЖНА добавлять все заголовки из `additional_headers` (map[string]string) к запросу провайдера.

## Вне scope

- Внешние Vault-провайдеры (HashiCorp Vault, AWS Secrets Manager).
- Ротация ключей на лету (hot-reload).
- Управление ключами через UI.
- Шифрование ключей в YAML-файле.
- Аутентификация для health-check эндпоинтов (если отличается от API).
- Per-tenant ключи (уже есть в `TenantConfig.APIKeys` для #80-tenant-isolation).
- Множественные ключи для ротации — `api_keys[0]` используется для аутентификации, остальные зарезервированы.

## Критерии приемки

### AC-001 Чтение APIKeys из YAML

- Почему это важно: базовый сценарий — ключи лежат в конфиг-файле рядом с другими настройками провайдера.
- **Given** YAML-конфиг с секцией `routing.providers[0].api_keys: ["sk-abc123"]`
- **When** система загружает конфиг
- **Then** `ProviderConfig.APIKeys` содержит `["sk-abc123"]`
- Evidence: тест `TestProviderConfig_APIKeys` проходит с api_keys из YAML.

### AC-002 Чтение APIKeys из env

- Почему это важно: ключи не должны храниться в репозитории, env — стандартный способ поставки секретов.
- **Given** env `CONFIG_ROUTING_PROVIDERS_0_API_KEYS_0=sk-env-key` (без api_keys в YAML)
- **When** система загружает конфиг
- **Then** `ProviderConfig.APIKeys` содержит `["sk-env-key"]`
- Evidence: тест с `t.Setenv` и YAML без api_keys.

### AC-003 AuthScheme и AuthHeader значения по умолчанию

- Почему это важно: оператор не должен явно указывать bearer+Authorization каждый раз.
- **Given** конфиг провайдера без `auth_scheme` и `auth_header`
- **When** система загружает конфиг
- **Then** `AuthScheme` равен `"bearer"`, `AuthHeader` равен `"Authorization"`
- Evidence: тест проверяет defaults после Unmarshal.

### AC-004 Кастомный AuthHeader с AuthScheme

- Почему это важно: некоторые провайдеры ожидают ключ в кастомном заголовке (X-API-Key, X-Auth-Token).
- **Given** конфиг провайдера с `auth_scheme: "api-key"` и `auth_header: "X-API-Key"`
- **When** система формирует запрос к провайдеру
- **Then** заголовок `X-API-Key` присутствует в `ProviderRequest.Headers`
- Evidence: тест проверяет наличие кастомного заголовка.

### AC-005 Валидация required APIKeys для сконфигурированного провайдера

- Почему это важно: без ключа запрос провайдера гарантированно упадёт с 401.
- **Given** конфиг с `routing.providers[0].name: "my-provider"` без `api_keys` и без соответствующей env
- **When** `validateConfig` выполняется
- **Then** возвращается ошибка валидации с сообщением, содержащим `routing.providers.0.api_keys`
- Evidence: тест ожидает ошибку при отсутствии api_keys.

### AC-006 Маскировка APIKeys в debug-выводе

- Почему это важно: ключи в логах — уязвимость безопасности, операторы не должны случайно их залогировать.
- **Given** инстанс `ProviderConfig` с `APIKeys: ["secret-value"]`
- **When** конфиг выводится в debug-лог (slog, JSON)
- **Then** значение `api_keys` в выводе заменено на `["****"]` (или замаскировано)
- Evidence: тест проверяет, что JSON/slog-вывод не содержит `"secret-value"` для поля api_keys.

### AC-007 AdditionalHeaders

- Почему это важно: некоторые провайдеры требуют дополнительные заголовки (org-id, custom headers).
- **Given** конфиг провайдера с `additional_headers: {"X-Org-Id": "acme", "X-Custom": "value"}`
- **When** система формирует запрос к провайдеру
- **Then** заголовки `X-Org-Id` и `X-Custom` присутствуют в `ProviderRequest.Headers`
- Evidence: тест проверяет наличие всех заголовков из additional_headers.

## Допущения

- `api_keys` — массив строк, каждая строка плоская (не multi-line, не бинарная).
- Для аутентификации используется `api_keys[0]`; остальные ключи зарезервированы.
- `auth_scheme` валидируется как enum; неизвестное значение — ошибка валидации.
- Auth-заголовок и additional_headers добавляются в адаптере провайдера.
- При `auth_scheme: basic` значение `api_keys[0]` используется как credentials в формате `base64(user:password)` — user пустой.
- `additional_headers` мержатся поверх auth-заголовка; при конфликте ключа приоритет у auth-заголовка.
- env для `additional_headers` не поддерживается (только YAML).

## Критерии успеха

- SC-001 Ни один тест на маскировку не падает при изменении формата логов.
- SC-002 Время старта не увеличивается более чем на 1ms из-за валидации auth-полей.

## Краевые случаи

- Провайдер без `name` — валидация api_keys не применяется (возможен partial config).
- `api_keys: []` (пустой массив) приравнивается к отсутствию.
- `auth_scheme` в неверном регистре — ошибка валидации (строчные).
- Провайдеров несколько — каждый валидируется независимо.
- `auth_header` с пустой строкой — используется `Authorization`.
- `additional_headers` с ключом, совпадающим с auth-заголовком — приоритет у auth-заголовка (не перезаписывается).
- Backward compatibility: старый YAML с `api_key: "str"` (единственное значение) должен работать — через viper mapping или fallback.

## Открытые вопросы

- **Решено:** auth-заголовок и additional_headers добавляются в адаптере провайдера (следует DEC-001 из plan).
- Нужна ли поддержка `CONFIG_ROUTING_PROVIDERS_0_ADDITIONAL_HEADERS_ORG_ID` для env? Пока нет — только YAML.
