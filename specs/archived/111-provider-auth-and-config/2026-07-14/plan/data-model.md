---
status: change
reason: ProviderConfig меняет тип поля APIKey (string → []string) и добавляет три новых поля.
---

## Data Model Changes

### ProviderConfig (src/internal/infra/config/config.go)

| Поле | Тип | Изменение |
|---|---|---|
| `APIKey` | `string` | **renamed** → `APIKeys []string`, с fallback для `api_key: "str"` в YAML |
| `AuthScheme` | `string` | **new** — enum: bearer / api-key / basic, default: bearer |
| `AuthHeader` | `string` | **new** — кастомное имя заголовка, default: Authorization |
| `AdditionalHeaders` | `map[string]string` | **new** — произвольные заголовки, добавляемые к запросу провайдера |

### Доменные сущности

- `domain/routing.Provider` — не меняется.
- `ports.ProviderRequest.Headers` — используется без изменений.

### Контракты

- Не меняются. Все заголовки передаются через существующий `ProviderRequest.Headers`.
