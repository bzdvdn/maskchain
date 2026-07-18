status: minimal-change

## ProviderConfig

Добавляются поля для AWS Bedrock:

```go
type ProviderConfig struct {
    // ... existing fields
    AWSRegion          string `mapstructure:"aws_region" yaml:"aws_region"`
    AWSAccessKeyID     string `mapstructure:"aws_access_key_id" yaml:"aws_access_key_id"`
    AWSSecretAccessKey string `mapstructure:"aws_secret_access_key" yaml:"aws_secret_access_key"`
}
```

`AWSRegion` — обязателен для `api_type: bedrock`. `AWSAccessKeyID` / `AWSSecretAccessKey` — опциональны (если не указаны — SDK читает env vars / IAM role).

## YAML

```yaml
routing:
  providers:
    - name: gemini
      api_type: gemini
      base_url: "https://generativelanguage.googleapis.com"
      api_keys: ["${GEMINI_API_KEY}"]

    - name: bedrock
      api_type: bedrock
      aws_region: "us-east-1"
      aws_access_key_id: "${AWS_ACCESS_KEY_ID}"    # опционально
      aws_secret_access_key: "${AWS_SECRET_ACCESS_KEY}"  # опционально
      timeout: 120s

    - name: groq
      api_type: proxy
      base_url: "https://api.groq.com/openai/v1"
      api_keys: ["${GROQ_API_KEY}"]
```

## No changes

- `ProviderClient` port interface — не меняется (Call + Stream покрывают все случаи)
- `ProviderRequest` / `ProviderResponse` — не меняются
- Shield domain — не затрагивается
- Routing domain — не затрагивается
- Analytics — не затрагивается
