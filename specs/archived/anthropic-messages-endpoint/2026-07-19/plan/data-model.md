# Anthropic Native Messages Endpoint — Data Model

## Изменения

### 1. `ports.ProviderRequest` — новое поле `Path`

```go
type ProviderRequest struct {
    Method  string
    URL     string
    Body    []byte
    Headers map[string]string
    Path    string  // + оригинальный path запроса (напр. "/v1/messages")
}
```

**Назначение:** единственный сигнал для `AnthropicClient` о том, в каком формате пришло тело запроса. Не влияет на другие адаптеры (zero-value = игнорируется).

**Значение:** оригинальный path из gin request (например, `/api/v1/messages` → `Path: "/api/v1/messages"`). Устанавливается в `RoutingProxyHandler.HandleChatCompletion` до вызова адаптера.

**Потребители:**
| Адаптер | Реакция на Path |
|---------|-----------------|
| `AnthropicClient` | Проверяет `req.Path` — всегда passthrough, без конвертации. Контракт: если в будущем появится конвертация, `Path == "/v1/messages"` отключает её. |
| `OpenAIClient` | Игнорирует (zero-value) |
| `GeminiClient` | Игнорирует (zero-value) |
| `BedrockClient` | Игнорирует (zero-value) |
| `ProxyClient` | Игнорирует (zero-value) |

### 2. Пути запросов (без изменений в типах)

| Входящий путь | Upstream path для `req.URL` | `req.Path` |
|---------------|----------------------------|------------|
| `/api/v1/chat/completions` | `/v1/chat/completions` | `/api/v1/chat/completions` |
| `/api/v1/messages` | `/v1/messages` | `/api/v1/messages` |

### 3. HTTP redirect (без изменений в типах)

- `GET /v1/messages` → 301 → `/api/v1/messages` (аналогично `/v1/chat/completions` → `/api/v1/chat/completions`)

## Контракты

### ProviderRequest.Path

```
If:  Path == "/api/v1/messages"
Then: AnthropicClient НЕ конвертирует тело (passthrough)

If:  Path == "/api/v1/chat/completions"
Then: AnthropicClient НЕ конвертирует тело (passthrough, обратная совместимость)

If:  Path == "" or any other value
Then: адаптер игнорирует Path (zero-value behaviour)
```

### Ограничения

- Path — read-only для адаптеров; заполняется только в handler
- Handler не инспектирует тело для определения Path — только `c.Request.URL.Path`
- Routing/selector/fallback не используют Path
