# Data Model: Ollama Provider

## Status

no-change

## Обоснование

`ProviderConfig` в `config.go` уже содержит все необходимые поля:
- `APIType` — `"ollama"`
- `BaseURL` — `http://localhost:11434`
- `APIKeys` — опционально (для reverse proxy с auth)
- `AuthScheme`, `AuthHeader` — standard (bearer по умолчанию)
- `AdditionalHeaders` — если нужны кастомные заголовки

Новых таблиц, сущностей или полей не требуется.
