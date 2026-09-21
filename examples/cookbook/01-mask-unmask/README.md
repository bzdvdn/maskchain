# 01 — Mask & Unmask plain text

**Goal:** replace sensitive fragments with reversible placeholders, then restore them.

Prereq: [quickstart](../../quickstart/) is running.

## Mask

```bash
curl -s -D - -X POST "http://localhost:8080/api/v1/shield/mask" \
  -H "Authorization: Bearer sk-test-default" \
  -H "Content-Type: text/plain" \
  --data 'Contact alice@example.com or +1-555-0100, SSN 123-45-6789'
```

Response headers carry the two ids you need later:

```
mask-id: <key>          # storage key for this mask entry
data_mask_id: <key>     # id embedded in text (only with format=id)
```

Body (default `format=clean`):

```
Contact [MASK.1] or [MASK.2], SSN [MASK.3]
```

## Unmask

Pass the `mask-id` header value back:

```bash
curl -s -X POST "http://localhost:8080/api/v1/shield/unmask?mask_ids=<mask-id>" \
  -H "Content-Type: text/plain" \
  --data 'Contact [MASK.1] or [MASK.2], SSN [MASK.3]'
```

## Formats

| `?format=` | Token | Reversible |
|---|---|---|
| `clean` (default) | `[MASK.1]` | yes |
| `id` | `[MASK_<data_mask_id>.1]` | yes |
| `redact` | `[REDACTED]` | no — unmask returns 400 |

```bash
curl -s -X POST "http://localhost:8080/api/v1/shield/mask?format=redact" ...
```

## Notes

- `mask_ids` accepts a comma-separated list to restore a document composed of
  several mask entries.
- Mask entries are stored in Postgres; reuse `?mask_id=<id>` to append to the
  same entry (conflicting values return 409).
