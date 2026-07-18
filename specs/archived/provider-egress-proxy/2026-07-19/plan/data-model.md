# provider-egress-proxy — Модель данных

## ProviderConfig (расширение)

```go
type ProviderConfig struct {
    // ... existing fields ...
    ProxyURL string `mapstructure:"proxy_url" yaml:"proxy_url"`
}
```

## YAML

```yaml
routing:
  providers:
    - name: openai
      proxy_url: http://corp-proxy:3128   # новый опциональный параметр
```

## Env var fallback

```
HTTP_PROXY  → глобальный proxy для http (если proxy_url не указан)
HTTPS_PROXY → глобальный proxy для https (если proxy_url не указан)
NO_PROXY    → исключения из proxy
```

## Приоритет

1. `proxy_url` в конфиге провайдера (явный)
2. `HTTP_PROXY`/`HTTPS_PROXY` env vars (глобальный)
3. Прямое соединение (без proxy)
