---
status: no-change
---

# Data Model: 23-shield-reactions

## Status

**no-change** — ни одна существующая сущность, value object, таблица в PostgreSQL или Valkey не меняется.

## Обоснование

- Все реакции оперируют существующими типами: `entity.ScanResult`, `entity.Incident`, `entity.Reaction`, `mask.MaskEntry`, `mask.MaskStorage`, `entity.Incident` и `IncidentRepository`.
- Добавляется только sentinel error `ErrBlockedByPolicy` в пакет shield/errors — это runtime-константа, не data model.
- Новые типы (`ReactionExecutor`, `ReactionPipeline`) — domain interfaces, не entities.

## Затрагиваемые типы

| Тип | Изменение |
|-----|-----------|
| `shield/errors.ErrBlockedByPolicy` | ADD — новый sentinel error |
| `entity.Reaction` | No change |
| `entity.ScanResult` | No change |
| `entity.Incident` | No change |
| `mask.MaskEntry` | No change |
| `mask.MaskUseCase` | Method `MaskFromResults` ADD, `MaskText` REMOVE |
