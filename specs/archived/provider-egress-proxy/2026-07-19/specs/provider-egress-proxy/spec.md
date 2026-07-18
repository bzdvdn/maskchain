# provider-egress-proxy — Per-Provider Egress Proxy

## Цель

Добавить поддержку HTTP/HTTPS/SOCKS5 прокси на уровне провайдера в конфигурации MaskChain. Каждый провайдер (OpenAI, Anthropic, Ollama и т.д.) сможет иметь собственный egress proxy, независимый от глобальных `HTTP_PROXY`/`HTTPS_PROXY` env vars.

## Мотивация

- **Корпоративные сети:** Трафик к внешним LLM API обязан проходить через корпоративный proxy (audit, firewall, DLP).
- **Geo-routing:** Провайдеры в разных регионах требуют разных точек выхода (EU proxy для OpenAI EU, US proxy для Anthropic US).
- **Air-gapped:** В изолированных средах весь внешний трафик — через единственный SOCKS5 proxy.
- **Mixed mode:** Один провайдер (Ollama) — internal network без proxy, другой (OpenAI) — через корпоративный proxy.

## Scope

### In scope

- Добавление поля `proxy_url` в `ProviderConfig` (YAML + struct)
- Поддержка схем `http://`, `https://`, `socks5://`
- Если `proxy_url` не указан — fallback на `HTTP_PROXY`/`HTTPS_PROXY` env vars (текущее поведение)
- Если `proxy_url` указан — он переопределяет env vars для этого провайдера
- SOCKS5 через `golang.org/x/net/proxy` с кастомным `DialContext`
- Обновление `proxy.go` — новая функция `proxyFuncFromURL(url string)` для явного URL
- Обновление `pool.go` — `NewTransport()` принимает опциональный `proxyURL` string
- Обновление `factory.go` — передача `pcfg.ProxyURL` в `NewTransport()`
- Тесты: unit-тесты на новый proxy resolution, SOCKS5 dial

### Out of scope

- Per-tenant proxy (отдельная фича, требует propagation tenant context)
- Proxy authentication (Basic auth через URL: `http://user:pass@host:port`)
- Динамическое переключение proxy без hot-reload конфига
- Proxy health checking / failover

## Acceptance Criteria

| AC | Описание | Observable proof |
|----|----------|------------------|
| AC-001 | Поле `proxy_url` добавлено в `ProviderConfig` и читается из YAML | `grep 'ProxyURL' config.go` |
| AC-002 | Провайдер с `proxy_url: "http://proxy:3128"` направляет трафик через указанный HTTP proxy | Интеграционный тест с mock proxy сервером |
| AC-003 | Провайдер с `proxy_url: "socks5://proxy:1080"` направляет трафик через SOCKS5 proxy | Интеграционный тест с mock SOCKS5 сервером |
| AC-004 | Провайдер без `proxy_url` использует `HTTP_PROXY` env var (fallback) | Существующий `TestCallViaProxy` проходит |
| AC-005 | Если `proxy_url` указан — он используется; если не указан (пустая строка) — fallback на env var | Unit test: `proxyFuncFromURL("")` возвращает env-var proxy |
| AC-006 | Все три типа провайдеров (OpenAI, Anthropic, Ollama) поддерживают `proxy_url` | `factory.go` передаёт proxy в `NewTransport()` |
| AC-007 | `go build ./...` без ошибок | CI lint |
| AC-008 | `go test ./src/internal/adapters/egress/...` проходит | go test |

## Конфигурация

```yaml
routing:
  providers:
    - name: openai
      api_type: openai
      base_url: https://api.openai.com
      api_keys: ["sk-..."]
      # без proxy — использует HTTP_PROXY env var

    - name: anthropic
      api_type: anthropic
      base_url: https://api.anthropic.com
      api_keys: ["sk-..."]
      proxy_url: http://corp-proxy:3128

    - name: internal-ollama
      api_type: ollama
      base_url: http://ollama:11434
      api_keys: ["sk-..."]
      proxy_url: ""  # явно без proxy
```

## Implementation

### ProviderConfig (config.go)

```go
type ProviderConfig struct {
    // ... existing fields
    ProxyURL string `mapstructure:"proxy_url" yaml:"proxy_url"`  // новый
}
```

### egress/proxy.go

```go
// Новая функция — создаёт proxy-функцию для явного URL
func proxyFuncFromURL(proxyURL string) (func(*http.Request) (*url.URL, error), error) {
    if proxyURL == "" {
        return proxyFunc(), nil  // fallback на env
    }
    parsed, err := url.Parse(proxyURL)
    if err != nil {
        return nil, err
    }
    return func(r *http.Request) (*url.URL, error) {
        return parsed, nil
    }, nil
}
```

### egress/pool.go

```go
func NewTransport(cfg *Config, proxyURL string) (*http.Transport, error) {
    pf, err := proxyFuncFromURL(proxyURL)
    if err != nil {
        return nil, err
    }
    tp := &http.Transport{
        Proxy: pf,
        // ... остальные поля
    }
    return tp, nil
}
```

### SOCKS5

При схеме `socks5://` нужно использовать `golang.org/x/net/proxy`:

```go
import "golang.org/x/net/proxy"

if parsed.Scheme == "socks5" {
    dialer, err := proxy.FromURL(parsed, proxy.Direct)
    if err != nil {
        return nil, err
    }
    tp.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
        return dialer.Dial(network, addr)
    }
}
```

## Dependencies

- `golang.org/x/net` — уже есть в go.sum (транзитивная через gRPC/OTel)
- Mock proxy сервер для тестов — `github.com/elazarl/goproxy` или самописный httptest

## Security

- Proxy URL может содержать credentials: `http://user:pass@proxy:3128`
- Credentials НЕ логировать (поле ProxyURL в конфиге должно быть masked при выводе)
- SOCKS5 без аутентификации (поддержка auth — PostMVP)
