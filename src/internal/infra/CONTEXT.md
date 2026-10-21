# src/internal/infra — Cross-Cutting Infrastructure

## Purpose

Platform concerns shared by every binary: configuration, cryptography, structured logging, Prometheus metrics, and OpenTelemetry wiring. Nothing here knows about tenants, routing, or HTTP routes.

## Responsibilities

- Owns: config loading/validation/defaults/hot-reload, AES-256-GCM encryption, slog logging, metrics registries, OTel setup.
- Does not own: business logic, persistence, or route definitions.

## How it works

- `config.LoadConfig(cmd)` / `LoadConfigFromDir(dir)` read YAML + env (`MASKCHAIN_*`) into `Config`; `DefaultConfig` fills gaps; `validator` fails closed on missing critical settings.
- `ConfigDirFromArgs` + `WatchConfigDir` hot-reload a config directory and call back with old/new; `DiffSections` reports which sections changed.
- `crypto.New(keyB64)` builds an AES-256-GCM `Encryptor` (`Encrypt` → nonce||ciphertext, `Decrypt`); `KeyEnvVar` is the conversation key, `config.KeysKeyEnvVar` (`MASKCHAIN_KEYS_KEY`) the at-rest keys key.
- `logging` builds `slog` handlers from `LogConfig`; `metrics` registers Prometheus collectors (including pool metrics); `telemetry` configures OTel exporters/shutdown.

## Key files

- `src/internal/infra/config/config.go` — `Config`/section structs, loaders.
- `src/internal/infra/config/validator.go` — fail-closed validation (e.g. keys key in DB mode).
- `src/internal/infra/config/watcher.go` — config directory hot-reload.
- `src/internal/infra/config/diff.go` — section-level config diff.
- `src/internal/infra/crypto/encryptor.go` — AES-256-GCM encryptor.
- `src/internal/infra/logging/logging.go` — slog handler construction.

## Public contracts

- `config.Config` with sections: log, server, database, crypto, valkey, mask, shield, routing, otel, ratelimit, egress, debug, session, dictionary_cache, analytics, conversations, budgets, compliance, data, log_export, tenants, admin.
- `crypto.Encryptor` (`Encrypt`/`Decrypt`) and `crypto.New`.
- `metrics.Handler`, OTel shutdown hooks.

## Dependencies / Consumed by

- Uses: viper/cobra, pgxpool (pool metrics), Prometheus, OTel, slog.
- Used by: every `cmd/*` binary and most packages via `config`/`logging`/`crypto`.

## Gotchas

- `MASKCHAIN_KEYS_KEY` is required in DB deployments; at-rest encryption fails closed.
- Never log config secrets; `key_at_rest` exposes only `configured` + cipher name.
- Hot-reload is section-aware — a change reports via `DiffSections`, not silent mutation.
- Keep `config` importable everywhere; do not let it depend on domain/app packages.
