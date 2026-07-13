# Provider Adapters Модель данных

## Scope

- Связанные `AC-*`: `AC-007`, `AC-008`, `AC-005`
- Связанные `DEC-*`: `DEC-003`
- Статус: `changed`

## Сущности

### DM-001 ProviderConfig (изменение)

- Назначение: конфигурация провайдера LLM для routing engine.
- Источник истины: YAML-файл / ENV, парсится viper.
- Инварианты: если APIType задан, он должен быть одним из известных (openai, anthropic). APIKey может быть пустым (no-auth).
- Связанные `AC-*`: `AC-007`, `AC-008`
- Связанные `DEC-*`: `DEC-001`, `DEC-002`
- Поля (`+` — новые):
  - `Name` — string, required
  - `BaseURL` — string, required
  - `HealthEndpoint` — string, optional
  - `Timeout` — string (duration), optional
  - `Priority` — int, optional
  - `+ APIType` — string, required (значения: openai, anthropic)
  - `+ APIKey` — string, optional (plaintext, secrets management вне scope)
- Жизненный цикл:
  - создаётся при старте gateway через LoadConfig
  - не обновляется runtime (требует рестарта)
  - не удаляется runtime
- Замечания по консистентности: не применимо (read-only после загрузки).

### DM-002 ProviderError (новая)

- Назначение: единый формат ошибки провайдера в теле ProviderResponse.
- Источник истины: HTTP-ответ от провайдера, парсится адаптером.
- Инварианты: StatusCode всегда заполнен (HTTP status). Type и Message — строки, могут быть пустыми.
- Связанные `AC-*`: `AC-005`
- Связанные `DEC-*`: `DEC-003`
- Поля:
  - `status_code` — int, HTTP status code (400, 401, 429, 500, ...)
  - `type` — string, optional (тип ошибки от провайдера, напр. "invalid_request_error")
  - `message` — string, optional (человекочитаемое описание)
- Жизненный цикл:
  - создаётся при получении HTTP 4xx/5xx от провайдера
  - не сохраняется, не кэшируется — передаётся в ProviderResponse.Body как JSON
  - удаляется после обработки
- Замечания по консистентности: если тело ошибки провайдера не JSON, поле message = raw body, type = "".

## Связи

- `DM-001 -> DM-002`: ProviderConfig.APIType определяет, по какому правилу парсить ошибку в ProviderError (формат ошибок OpenAI и Anthropic различается).
- `egress.Client -> DM-002`: egress возвращает HTTP response; адаптер парсит его в ProviderError.

## Производные правила

- Если HTTP status < 400, ошибка не парсится — ProviderResponse.Body содержит нормальный ответ.
- Если тело ошибки непарсимо как JSON, type = "", message = raw body.

## Переходы состояний

- none — обе сущности stateless.

## Вне scope

- Шифрование/маскирование APIKey — фаза secrets management.
- Валидация APIKey (что ключ валиден для провайдера).
