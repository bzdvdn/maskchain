# Tenant Dictionary Editor

## Scope Snapshot

- In scope: create, edit, and remove tenant **Dictionaries** inside the existing tenant form (create + edit), persisted through the tenant create/update API.
- Out of scope: any new backend endpoint, a dedicated Shield dictionaries page, and per-dictionary detector wiring.

## Goal

Operators can manage tenant dictionaries from the UI instead of the API only. Today dictionaries are visible but read-only: `TenantDetail` lists them and `DictionaryModal` shows entries, while `TenantForm` initializes `dictionaries: []`, loads existing values, and passes them through untouched — so a dictionary can never be added, edited, or removed from the console. This feature adds an editor to `TenantForm` so dictionaries are a first-class console capability.

## Primary User Flow

1. Starting point: an admin opens Create tenant or edits an existing tenant.
2. Main interaction: in a new "Dictionaries" section they add a dictionary, set its name, match mode, and one entry per line, and can add/remove dictionaries.
3. Outcome: saving persists dictionaries via the existing `dictionaries` field on tenant create/update; they reappear on `TenantDetail`.
4. Failure/fallback path: client validation blocks save on a missing/duplicate name or an empty entries list and shows an inline error.

## User Stories

- none

## MVP Slice

- The editor plus persistence on save (AC-001, AC-002, AC-005).

## First Deployable Outcome

- An operator can add a dictionary to a new tenant and see it on the tenant detail page after save.

## Scope

- A `DictionaryEditor` component (dictionary list, add/remove, name, match mode, entries).
- Wiring the editor into `TenantForm` for both create and edit, using the existing `dictionaries` request field.
- Client-side validation and an entries text format that round-trips structured entries.

## Context

- The API already supports dictionaries: `GET/PUT /api/v1/tenants/:slug/dictionaries` and `dictionaries` on tenant create/update (`src/internal/api/admin.go`).
- Domain shape: `{ name, entries, match_mode }`; `entries` is a list of strings or JSON-serializable objects; `match_mode` ∈ `exact | contains | regex | fuzzy`.
- `TenantForm` already carries `dictionaries` in its `FormData` and sends it on save, so this is a UI-only change.

## Dependencies

- none (reuses the existing tenant API and `DictionaryItem` type in `ui/src/api/tenants.ts`).

## Requirements

- RQ-001 `TenantForm` renders a "Dictionaries" section in both create and edit modes, with an empty state and an "Add dictionary" action.
- RQ-002 Each dictionary row exposes a name input (required), a match-mode select (`exact`, `contains`, `regex`, `fuzzy`; default `exact`), and an entries editor.
- RQ-003 Entries are edited as one entry per line; each line is parsed as JSON (object/array/number/boolean) when valid, otherwise stored as a string, so structured entries round-trip.
- RQ-004 Removing a dictionary removes it from the form state and from the saved payload.
- RQ-005 Save persists dictionaries through the existing tenant create/update request (`dictionaries` field); no new endpoint is added.
- RQ-006 Client validation blocks save when a dictionary name is empty, duplicated within the tenant, or has no entries, and shows an inline error on the offending dictionary.

## Non-Goals

- A standalone Shield/Dictionaries management page.
- Dictionary versioning, import/export, or bulk editing.
- Binding a dictionary to a specific detector from this UI.
- Any change to the API, domain types, or persistence schema.

## Acceptance Criteria

### AC-001 Add a dictionary on create

- Why this matters: dictionaries are currently uncreatable from the console.
- **Given** the Create tenant form
- **When** the admin adds a dictionary with a name, match mode, and entries and saves
- **Then** the create request includes that dictionary and it appears on the tenant detail page.
- Evidence: a `TenantForm` test asserts `createTenant` was called with `dictionaries` containing the entered dictionary.

### AC-002 Edit and persist existing dictionaries

- Why this matters: operators must maintain dictionaries over time.
- **Given** a tenant that already has dictionaries
- **When** the admin opens the edit form, changes a dictionary, and saves
- **Then** the update request carries the edited dictionaries.
- Evidence: a test loads a tenant with a dictionary and asserts the update payload reflects the edit.

### AC-003 Structured entries round-trip

- Why this matters: entry objects must not be silently flattened to strings.
- **Given** a dictionary whose entries are JSON objects
- **When** the admin edits and saves without changing entries
- **Then** the saved entries remain objects (not stringified).
- Evidence: a unit test on the entries (de)serialization helper preserves object entries.

### AC-004 Validation blocks invalid dictionaries

- Why this matters: the API would otherwise store unusable dictionaries.
- **Given** a dictionary with an empty name, a duplicate name, or no entries
- **When** the admin clicks Save
- **Then** submission is blocked and an inline error identifies the problem.
- Evidence: a test asserts no save call is made and the error is shown.

### AC-005 Remove a dictionary

- Why this matters: obsolete dictionaries must be deletable.
- **Given** a form with one or more dictionaries
- **When** the admin removes one and saves
- **Then** the saved payload no longer contains it.
- Evidence: a test asserts the remove control updates the submitted `dictionaries` list.

## Assumptions

- The tenant create/update endpoints accept and persist the `dictionaries` field (verified in `admin.go` and the handler).
- Match modes are limited to the four domain values; no server-side enum change is needed.
- Entries that are not valid JSON on a line are intended as plain-string entries.

## Success Criteria

- SC-001 A dictionary added in the create form is visible on the tenant detail page after save.

## Edge Cases

- Zero dictionaries: the section shows an empty state with an "Add dictionary" action.
- Large entries lists: the entries editor scrolls rather than breaking the form layout.
- Switching from create to edit of a tenant with no dictionaries leaves the section empty, not stale.

## Open Questions

- none
