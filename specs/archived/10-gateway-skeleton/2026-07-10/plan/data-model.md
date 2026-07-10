# Gateway Skeleton Data Model

## Status: change

## Изменения

### Config

**Location:** `src/internal/infra/config/config.go`

**Новое поле:**

```go
type Config struct {
    Log    *LogConfig    `mapstructure:"log" yaml:"log"`
    Server *ServerConfig `mapstructure:"server" yaml:"server"` // NEW
}

type ServerConfig struct {
    Port            int      `mapstructure:"port" yaml:"port" validate:"required"`
    ShutdownTimeout int      `mapstructure:"shutdown_timeout" yaml:"shutdown_timeout"` // seconds, default 10
    CORSOrigins     []string `mapstructure:"cors_origins" yaml:"cors_origins"`         // empty = no CORS
}
```

**Инвариант:** `Config.Server` не nil после `DefaultConfig()` — defaults задаются в конструкторе.

## Неизменные модели

- `LogConfig` — без изменений
- Domain/App/Ports модели — не затрагиваются

## Обоснование

ServerConfig необходим, чтобы гибко настраивать порт, таймауты graceful shutdown и CORS origins без перекомпиляции.
