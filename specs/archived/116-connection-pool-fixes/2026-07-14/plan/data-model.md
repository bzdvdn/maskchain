---
status: extends
reason: EgressConfig расширяется новыми вложенными структурами TLS и CircuitBreaker
---

# Data Model: 116-connection-pool-fixes

## Изменения

### `EgressConfig` (config.go)

```go
type EgressConfig struct {
    MaxIdleConns        int                  `mapstructure:"max_idle_conns" yaml:"max_idle_conns"`
    IdleTimeout         time.Duration        `mapstructure:"idle_timeout" yaml:"idle_timeout"`
    MaxRetries          int                  `mapstructure:"max_retries" yaml:"max_retries"`
    BaseBackoff         time.Duration        `mapstructure:"base_backoff" yaml:"base_backoff"`
    RetryOn5xx          bool                 `mapstructure:"retry_on_5xx" yaml:"retry_on_5xx"`
    MaxIdleConnsPerHost int                  `mapstructure:"max_idle_conns_per_host" yaml:"max_idle_conns_per_host"`
    DisableKeepAlives   bool                 `mapstructure:"disable_keep_alives" yaml:"disable_keep_alives"`
    TLS                 *EgressTLSConfig     `mapstructure:"tls" yaml:"tls"`                         // NEW
    CircuitBreaker      *CircuitBreakerConfig `mapstructure:"circuit_breaker" yaml:"circuit_breaker"` // NEW
}
```

### Новые структуры

#### `EgressTLSConfig`

```go
type EgressTLSConfig struct {
    CACert             string `mapstructure:"ca_cert" yaml:"ca_cert"`
    InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify" yaml:"insecure_skip_verify"`
    Cert               string `mapstructure:"cert" yaml:"cert"`
    Key                string `mapstructure:"key" yaml:"key"`
}
```

Все поля опциональны. `CACert`, `Cert`, `Key` — пути к PEM-файлам.

#### `CircuitBreakerConfig`

```go
type CircuitBreakerConfig struct {
    MaxFailures int           `mapstructure:"max_failures" yaml:"max_failures"`
    Cooldown    time.Duration `mapstructure:"cooldown" yaml:"cooldown"`
}
```

- `MaxFailures`: число последовательных ошибок до открытия цепи (default: 3)
- `Cooldown`: время в секундах, на которое цепь остаётся открытой (default: 30s)

### `ProviderConfig` (config.go)

Без изменений структуры. `Timeout string` парсится в `time.Duration` при создании egress.Client.

## Неизменяемые сущности

- `ports.ProviderClient` — интерфейс не меняется
- `ports.ProviderRequest`, `ports.ProviderResponse` — без изменений
- `domain/routing/health_status.go` — не меняется (опциональная интеграция в будущем)
