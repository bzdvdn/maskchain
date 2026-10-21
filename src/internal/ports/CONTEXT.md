# src/internal/ports — Cross-Layer Contracts

## Purpose

Transport-agnostic interfaces and DTOs shared by the application and adapter layers. Ports define *what* the data plane needs from a provider without importing HTTP, SQL, or a vendor SDK.

## Responsibilities

- Owns: the outbound provider contract (`ProviderClient`), request/response/chunk shapes, and the shared `ErrUnsupportedEndpoint` sentinel.
- Does not own: routing decisions, HTTP handler wiring, provider implementations, or config.

## How it works

- `ProviderRequest` carries method, URL, body, headers, an optional explicit `Path`, `RawQuery`, and a `ContentType` override for multipart/binary passthrough.
- `ProviderResponse` returns status, raw body and headers; providers do not decode business payloads here.
- `ProviderClient` has two methods: `Call` (unary) and `Stream` (returns `<-chan ProviderChunk` for SSE).
- `ErrUnsupportedEndpoint` means the selected provider cannot serve the requested endpoint shape; it is terminal, not retryable.

## Key files

- `src/internal/ports/provider.go` — all contracts in this module.

## Public contracts

- `ProviderClient.Call(ctx, *ProviderRequest) (*ProviderResponse, error)`.
- `ProviderClient.Stream(ctx, *ProviderRequest) (<-chan ProviderChunk, error)`.
- `ProviderChunk{Data, Err, Done}` for streaming consumers.
- `ErrUnsupportedEndpoint` sentinel.

## Dependencies / Consumed by

- Uses: stdlib only.
- Used by: `internal/adapters/egress`, `internal/adapters/provider`, `internal/app/*`, `internal/api/*`.

## Gotchas

- Keep this package dependency-free; adding infra imports here breaks the port/adapter boundary.
- `Stream` implementations must close the channel and surface errors via `ProviderChunk.Err`, not panic.
- `ErrUnsupportedEndpoint` must not be retried by egress callers.
