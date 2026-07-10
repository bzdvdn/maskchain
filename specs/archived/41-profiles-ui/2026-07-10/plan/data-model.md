---
status: extends
---

# Data Model: 41-profiles-ui

## Status

- extends: existing data model adds pagination wrapper response type

## Changes

### New: PaginatedResponse wrapper

UI needs paginated list response. Backend returns wrapper type instead of raw array.

```go
type PaginatedResponse struct {
    Data       any    `json:"data"`
    Total      int    `json:"total"`
    Page       int    `json:"page"`
    PageSize   int    `json:"page_size"`
}
```

### Changed: ListProfiles response format

- **Before (40-profiles-api):** `GET /api/v1/profiles` → `[]ProfileListItem`
- **After:** `GET /api/v1/profiles?page=1&page_size=20` → `PaginatedResponse{data: []ProfileListItem}`

Backward compatible only for the raw array consumer (no consumers yet in production).

### Unchanged

- `Profile`, `ProfileResponse`, `ProfileListItem`, `DictionaryDTO`, `CreateProfileRequest`, `UpdateProfileRequest`, `PatchDictionaryRequest`, `PreprocessorDef`, `Dictionary` — unchanged.
- `PreprocessorDef` format (`{name, type, rules[]}`) is already defined — resolves open question from spec.
- Tags field does not exist in domain/entity/DTO. RQ-002 mentions tags but no AC covers them — excluded from scope.
