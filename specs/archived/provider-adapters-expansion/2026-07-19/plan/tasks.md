# provider-adapters-expansion — Задачи

## Phase Contract

Inputs: plan provider-adapters-expansion, data-model, spec.
Outputs: исполнимые задачи с покрытием AC.
Stop if: нет.

## Surface Map

| Surface | Tasks |
|---------|-------|
| `src/internal/infra/config/config.go` | T1.1 |
| `src/internal/infra/config/defaults.go` | T1.1 |
| `src/internal/infra/config/serialize.go` | T1.1 |
| `src/internal/adapters/provider/factory.go` | T1.2, T2.3, T3.3 |
| `src/internal/adapters/provider/proxy.go` | T2.1 |
| `src/internal/adapters/provider/proxy_test.go` | T2.2 |
| `src/internal/adapters/provider/gemini.go` | T3.1 |
| `src/internal/adapters/provider/gemini_test.go` | T3.2 |
| `go.mod` / `go.sum` | T4.1 |
| `src/internal/adapters/provider/bedrock.go` | T4.2 |
| `src/internal/adapters/provider/bedrock_test.go` | T4.3 |
| `examples/config*.yaml` | T5.2 |
| `README.md` | T5.2 |

## Implementation Context

- **Цель MVP:** Proxy adapter + Gemini adapter. Bedrock — отдельный инкремент.
- **Инварианты:**
  - Все адаптеры реализуют `ports.ProviderClient` (`Call` + `Stream`)
  - Аутентификация: `buildAuthHeader()` из `ProviderConfig.APIKeys`, tenant-заголовки не форвардятся
  - Ошибки провайдера: `ParseProviderError(status, body, apiType)` — общий хелпер
- **Контракты:**
  - Proxy: тело запроса форвардится как есть, `Authorization` добавляется из `api_keys`
  - Gemini: OpenAI-формат ↔ Gemini-формат (только text)
  - Bedrock: AWS SigV4 через `aws-sdk-go-v2`
- **Границы scope:** Нет tool calling, нет multimodal, нет Azure Entra ID
- **Proof signals:** `go build ./...` + `go test ./src/internal/adapters/provider/...` pass

## Фаза 1: Основа

Цель: подготовить config и factory к новым типам провайдеров.

- [x] T1.1 Добавить поля AWS Bedrock в `ProviderConfig` (`AWSRegion`, `AWSAccessKeyID`, `AWSSecretAccessKey`) с mapstructure/yaml тегами. Маскировать sensitive поля в `MarshalLogObject`.
  Touches: `src/internal/infra/config/config.go`, `src/internal/infra/config/serialize.go`
  AC: AC-004 (подготовка)

- [x] T1.2 Добавить case `gemini`, `bedrock`, `proxy` в `NewProviderClient` switch. Пока возвращают `fmt.Errorf("not implemented")`.
  Touches: `src/internal/adapters/provider/factory.go`
  AC: AC-001, AC-004, AC-007

## Фаза 2: MVP — Proxy Adapter

Цель: простейший адаптер для OpenAI-совместимых провайдеров.

- [x] T2.1 Создать `ProxyClient` (структура + конструктор), реализующий `ProviderClient`. Call: форвардит тело, добавляет `Authorization: Bearer <api_key>` из конфига, Content-Type: application/json. Stream: то же + Accept: text/event-stream. Tenant-заголовки не форвардятся (кроме `X-Tenant-ID`).
  Touches: `src/internal/adapters/provider/proxy.go`
  AC: AC-007, AC-008
  DEC: DEC-003

- [x] T2.2 Тесты ProxyClient: httptest сервер проверяет тело запроса, auth header, отсутствие tenant-заголовков. Streaming: mock SSE сервер.
  Touches: `src/internal/adapters/provider/proxy_test.go`
  AC: AC-007 (observable proof)

## Фаза 3: MVP — Gemini Adapter

Цель: полноценный адаптер Google Gemini с конвертацией формата.

- [x] T3.1 Создать `GeminiClient`. Call: конвертирует OpenAI chat format → Gemini `contents[]` format → POST `${baseURL}/v1/models/${model}:generateContent?key=${apiKey}` → конвертирует Gemini response → OpenAI response. Stream: конвертирует OpenAI → Gemini format, отправляет POST с `stream: true`, читает SSE, конвертирует каждый chunk Gemini → OpenAI format.
  Конвертация OpenAI → Gemini:
  ```
  OpenAI: {messages: [{role, content}]}
  Gemini: {contents: [{role, parts: [{text}]}], systemInstruction: {parts: [{text}]}}
  ```
  Конвертация Gemini → OpenAI:
  ```
  Gemini: {candidates: [{content: {parts: [{text}]}}]}
  OpenAI: {choices: [{message: {content}, finish_reason}]}
  ```
  Touches: `src/internal/adapters/provider/gemini.go`
  AC: AC-001, AC-002, AC-003, AC-008
  DEC: DEC-004

- [x] T3.2 Тесты GeminiClient: httptest сервер проверяет Gemini-формат запроса. Golden JSON для конвертации. Mock SSE сервер для streaming. Edge cases: empty content, role mapping, finish_reason.
  Touches: `src/internal/adapters/provider/gemini_test.go`
  AC: AC-001 (factory), AC-002 (call), AC-003 (stream)

## Фаза 4: Основная реализация — Bedrock Adapter

Цель: AWS Bedrock с SigV4 аутентификацией.

- [x] T4.1 Добавить зависимость `github.com/aws/aws-sdk-go-v2` и `github.com/aws/aws-sdk-go-v2/service/bedrockruntime`. `go mod tidy`.
  Touches: `go.mod`, `go.sum`
  AC: AC-008
  DEC: DEC-002

- [x] T4.2 Создать `BedrockClient`. Call: конвертирует OpenAI → Bedrock `InvokeModel` → Bedrock → OpenAI. Stream: `InvokeModelWithResponseStream`, читает чанки, конвертирует.
  - Auth: AWS SDK v2 разрешает credentials (env → shared config → IAM role).
  - URL: `bedrock-runtime.{region}.amazonaws.com/model/{modelId}/invoke`
  - Config: `aws_region` обязателен; `aws_access_key_id`/`aws_secret_access_key` опциональны.
  Touches: `src/internal/adapters/provider/bedrock.go`
  AC: AC-004, AC-005, AC-006, AC-008
  DEC: DEC-002

- [x] T4.3 Тесты BedrockClient: mock через `bedrockruntimeiface`. Проверка SigV4 signature в заголовках. Mock streaming response.
  Touches: `src/internal/adapters/provider/bedrock_test.go`
  AC: AC-005 (sigv4), AC-006 (stream)

## Фаза 5: Проверка

Цель: доказать работоспособность и оставить код в reviewable состоянии.

- [x] T5.1 Зарегистрировать все три адаптера в `factory.go` (заменить error-stub на реальные конструкторы). Прогнать `go build ./...` и `go test -race -count=1 ./src/internal/adapters/provider/...`.
  Touches: `src/internal/adapters/provider/factory.go`
  AC: AC-001, AC-004, AC-007, AC-008, AC-009

- [x] T5.2 Обновить `examples/config.yaml` (добавить пример gemini + groq/proxy + bedrock провайдеров). Обновить `README.md` секцию Routing (список поддерживаемых api_types). Добавить comment в `values.yaml` Helm chart с примерами proxy + bedrock провайдера.
  Touches: `examples/config.yaml`, `README.md`, `deployments/helm/maskchain/values.yaml`
  AC: — (documentation)

## Покрытие критериев приемки

- AC-001 (`api_type: gemini` → `*GeminiClient`) → T1.2, T3.1, T5.1
- AC-002 (Gemini Call, проверка тела) → T3.1, T3.2
- AC-003 (Gemini Stream) → T3.1, T3.2
- AC-004 (`api_type: bedrock` → `*BedrockClient`) → T1.1, T1.2, T4.2, T5.1
- AC-005 (Bedrock SigV4) → T4.2, T4.3
- AC-006 (Bedrock Stream) → T4.2, T4.3
- AC-007 (Proxy: форвард + auth, без tenant headers) → T1.2, T2.1, T2.2, T5.1
- AC-008 (`go build ./...`) → T2.1, T3.1, T4.1, T4.2, T5.1
- AC-009 (`go test ./src/internal/adapters/provider/...`) → T2.2, T3.2, T4.3, T5.1

## Заметки

- Фазы 2 и 3 не имеют пересечений по файлам — можно выполнять параллельно
- Фаза 4 (Bedrock) не блокирует MVP и может выполняться отдельно
- Никаких миграций БД не требуется
- Helm chart values.yaml — только comment, функциональных изменений нет

Готово к: /spk.implement provider-adapters-expansion
