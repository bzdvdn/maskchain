# provider-adapters-expansion — Google Gemini, AWS Bedrock, Generic Proxy

## Цель

Добавить поддержку трёх новых типов LLM-провайдеров: Google Gemini (собственный REST API), AWS Bedrock (через AWS SDK) и generic `proxy`-адаптер для любых OpenAI-совместимых API. Текущие адаптеры (OpenAI, Anthropic, Ollama) покрывают ~40% рынка по моделям; после фичи — ~95%.

## Мотивация

- **Gemini** — второй по популярности LLM API после OpenAI. Не имеет встроенной OpenAI-совместимости.
- **AWS Bedrock** — стандартный способ получить Claude/Llama/Mistral в enterprise на AWS. Требует AWS Signature V4 auth.
- **OpenAI-совместимые** — Mistral, DeepSeek, Groq, Together, Fireworks, OpenRouter, Perplexity и ещё ~30 сервисов. Формально уже работают через `api_type: openai` с другим `base_url`, но пользователь должен знать эту деталь. `proxy`-адаптер даёт явный semantic контракт.

## Scope

### In scope

- **Google Gemini** — новый `api_type: gemini`:
  - `POST https://generativelanguage.googleapis.com/v1/models/{model}:generateContent`
  - Конвертация OpenAI chat format → Gemini content format и обратно
  - Streaming через SSE (Gemini возвращает Server-Sent Events)
  - API key auth через `X-Goog-Api-Key` header
  - Поддержка system instruction (Gemini `system_instruction` field)

- **AWS Bedrock** — новый `api_type: bedrock`:
  - AWS Signature V4 для аутентификации каждого запроса
  - Поддержка Bedrock Runtime (`InvokeModel`, `InvokeModelWithResponseStream`)
  - Конфигурация через `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION` (env vars или config)
  - Streaming через Bedrock Runtime API
  - Поддержка моделей: Claude (Anthropic через Bedrock), Llama, Mistral

- **Generic proxy** — новый `api_type: proxy`:
  - Прозрачный форвард тела запроса (JSON) upstream без конвертации
  - **Не форвардит** входящие заголовки клиента (tenant `Authorization` уже съеден auth middleware)
  - Добавляет `Authorization: Bearer <api_key>` из `ProviderConfig.APIKeys` (аналогично OpenAI-адаптеру)
  - Опциональные доп. заголовки из `additional_headers` конфига
  - URL: `base_url + /v1/chat/completions` (как OpenAI-адаптер)
  - Streaming passthrough (SSE)
  - Полезен для: vLLM, TGI, TensorRT-LLM, Mistral, DeepSeek, Groq, Together, любых OpenAI-совместимых
  - **Privacy guarantee:** tenant API key никогда не попадает upstream

- **Factory** — `NewProviderClient()` расширяется тремя новыми case'ами
- **Config** — Bedrock требует новых полей: `aws_region`, `aws_access_key_id`, `aws_secret_access_key`
- **Тесты**: unit-тесты для каждого нового адаптера

### Out of scope

- Per-model pricing внутри Bedrock (модели биллятся по-разному на одной ноде Bedrock)
- OAuth2/Entra ID для Azure OpenAI (отдельная фича, решается через `api_type: openai` с header token)
- Multimodal для Gemini (images, audio — только text)
- Tool calling (function calling) для Gemini/Bedrock — пока только chat completions
- Batch inference (только real-time)

## Acceptance Criteria

| AC | Описание | Observable proof |
|----|----------|------------------|
| AC-001 | `api_type: gemini` создаёт GeminiClient, реализующий ProviderClient | `NewProviderClient` возвращает `*GeminiClient` |
| AC-002 | GeminiClient.Call конвертирует OpenAI-формат в Gemini-формат и шлёт запрос | Unit test с httptest сервером, проверка тела запроса |
| AC-003 | GeminiClient.Stream читает SSE и возвращает чанки через канал | Unit test с mock SSE сервером |
| AC-004 | `api_type: bedrock` создаёт BedrockClient, реализующий ProviderClient | `NewProviderClient` возвращает `*BedrockClient` |
| AC-005 | BedrockClient.Call подписывает запрос AWS Signature V4 | Unit test: проверка заголовка `Authorization` |
| AC-006 | BedrockClient.Stream читает streaming response от Bedrock Runtime | Unit test с mock Bedrock Runtime |
| AC-007 | `api_type: proxy` форвардит тело запроса, добавляет `Authorization` из конфига, не пропускает tenant-заголовки | Unit test: проверка заголовков upstream, tenant key отсутствует |
| AC-008 | Все три адаптера проходят `go build ./...` | CI lint |
| AC-009 | `go test ./src/internal/adapters/provider/...` проходит | go test |

## Конфигурация

```yaml
routing:
  providers:
    # Google Gemini
    - name: gemini-pro
      api_type: gemini
      base_url: "https://generativelanguage.googleapis.com"
      api_keys: ["${GEMINI_API_KEY}"]
      timeout: 60s

    # AWS Bedrock
    - name: bedrock
      api_type: bedrock
      aws_region: "us-east-1"
      aws_access_key_id: "${AWS_ACCESS_KEY_ID}"
      aws_secret_access_key: "${AWS_SECRET_ACCESS_KEY}"
      timeout: 120s

    # Generic proxy (OpenAI-compatible) — требует api_keys для upstream auth
    - name: groq
      api_type: proxy
      base_url: "https://api.groq.com/openai/v1"
      api_keys: ["${GROQ_API_KEY}"]
      timeout: 60s

    # Локальный vLLM без auth
    - name: vllm-local
      api_type: proxy
      base_url: "http://vllm:8000/v1"
      api_keys: [""]
      timeout: 120s
```

## User Stories

- P1 Story: Как разработчик, я хочу использовать Gemini через MaskChain, чтобы получить PII-shield для Gemini-трафика.
- P2 Story: Как enterprise-клиент на AWS, я хочу路由ировать трафик через Bedrock, не меняя код.
- P3 Story: Как MLOps, я хочу воткнуть MaskChain перед vLLM, не меняя HTTP-формат.

## MVP Slice

P1 + P3 (Gemini + Proxy) — минимальный прирост покрытия до ~95%. Bedrock — P2, требует AWS SDK dependency.
