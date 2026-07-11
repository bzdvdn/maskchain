---
status: no-change
---

# Data Model: 50-shield-engine

Domain data model не изменяется:

- `entity.ScanResult` — без изменений.
- `entity.Profile` — без изменений (уже содержит `Detectors`, `Dictionaries`, `Preprocessors`).
- `entity.Incident` — без изменений.
- Все value objects (`ProfileSlug`, `ScanStatus`, `Severity`, etc.) — без изменений.

Создаются app-level DTO в `src/internal/app/usecase/shield/`:

- `ScanRequest{Text string, ProfileSlug string}` — входной DTO.
- `ScanResponse{*entity.ScanResult, ProcessedText string, Replacements map[string]string}` — выходной wrapper.

Эти типы не являются частью domain data model и не требуют миграций или изменений схемы БД.
